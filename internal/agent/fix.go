package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/prompt"
	"github.com/uskanda/switchboard/internal/trust"
	"github.com/uskanda/switchboard/internal/workspace"
)

// fixInfo is what a fix run fixes: why it runs, the report claude reads, and what its claim names.
type fixInfo struct {
	cause   string // prompt.FixCI, prompt.FixPush or prompt.FixConflict
	report  string // the failure report, fenced in the prompt; redacted
	checks  string // for a CI fix: the failed checks, as the claim names them
	attempt int    // the run's number among the fix runs since the last reset, from 1
}

// fixes counts the fix runs of the issue since the last reset: a trusted comment, or a claim made from a start label.
// The fix runs are known by the reason of their claims, so every host counts the same. A fix run that stopped on a
// usage limit, whose wait carries until, is not counted (ADR-047).
func (pl *plan) fixes() int {
	n := 0
	limited := map[string]bool{} // the runs whose wait says they hit a usage limit
	for i := len(pl.comments) - 1; i >= 0; i-- {
		if pl.kinds[i] == trust.Instruction {
			break
		}
		m := pl.agentMarker(i)
		if m != nil && m.State == marker.Waiting && !m.Until.IsZero() {
			limited[m.Run] = true
		}
		if m == nil || m.State != marker.Running {
			continue
		}
		if m.Mode != "" {
			break
		}
		if m.Reason == marker.Fix && !limited[m.Run] {
			n++
		}
	}
	return n
}

// fixesDone is how many fix runs the issue had, counting this run when it is one.
func (pl *plan) fixesDone() int {
	if pl.fix != nil {
		return pl.fix.attempt
	}
	return pl.fixes()
}

// fixPlan reads the issue again and returns the plan of a fix run, or nil when the issue no longer allows one:
// closed, not labeled want, or with a body that is not trustworthy.
func (a *Agent) fixPlan(ctx context.Context, p config.Project, ref forge.IssueRef, want string, fix *fixInfo) (*plan, error) {
	is, comments, err := a.forgeOf(ref).Thread(ctx, ref)
	if err != nil {
		return nil, err
	}
	j := a.judgeOf(ref)
	if is.State != "open" || labels.StateOf(is.Labels, a.Config.Labels) != want || !j.Body(is) {
		return nil, nil
	}
	pl := &plan{project: p, issue: is, comments: comments, kind: fixRun, fix: fix}
	for _, c := range comments {
		pl.kinds = append(pl.kinds, j.Comment(ref, c))
	}
	fix.attempt = pl.fixes() + 1
	return pl, nil
}

// claimText is the comment that claims the run of pl, without its marker.
func (a *Agent) claimText(pl *plan) string {
	s := fmt.Sprintf(a.msg.claim, a.Config.Host.Name)
	if f := pl.fix; f != nil {
		limit := a.Config.Agent.FixAttempts()
		switch f.cause {
		case prompt.FixPush:
			s += "\n\n" + fmt.Sprintf(a.msg.fixPush, f.attempt, limit)
		case prompt.FixConflict:
			s += "\n\n" + fmt.Sprintf(a.msg.fixConflict, f.attempt, limit)
		default:
			s += "\n\n" + fmt.Sprintf(a.msg.fixCI, f.attempt, limit, f.checks)
		}
	}
	return s
}

// fixAgain ends the run of rec, whose push was refused because the issue branch moved on, and starts a fix run of
// the same issue in its place: the issue stays running, and the run's slot and lock pass to the fix run. It returns
// the fix run's outcome; o made a failure when the fix run could not be claimed; or nil when someone took the issue
// over meanwhile.
func (a *Agent) fixAgain(ctx context.Context, prev *plan, act *active, rec *record, o *outcome) *outcome {
	fix := o.next
	o.next = nil
	ref := prev.issue.Ref
	pl, err := a.fixPlan(ctx, prev.project, ref, a.Config.Labels.Running, fix)
	switch {
	case ctx.Err() != nil:
		return nil
	case err != nil:
		return o.fail(fmt.Sprintf(a.msg.pushFailed, err))
	case pl == nil:
		return nil
	}
	run := newID()
	m := marker.Marker{Host: a.Config.Host.Name, Run: run, State: marker.Running, Reason: marker.Fix, Ack: pl.ack(), At: time.Now()}
	if _, err := a.forgeOf(ref).PostComment(ctx, ref, a.claimText(pl)+"\n\n"+m.Format()+"\n"); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return o.fail(fmt.Sprintf(a.msg.pushFailed, err))
	}
	a.dropProgress(ctx, ref, rec.Run, pl.comments)
	rec.State, rec.Ended, rec.Result = "finished", time.Now().UTC(), "fix"
	a.saveRecord(ref, *rec)
	a.mu.Lock()
	act.run = run
	a.mu.Unlock()
	*rec = record{Issue: ref.String(), Run: run, Session: newUUID(), State: "running", Started: time.Now().UTC(), Ack: pl.ack()}
	a.saveRecord(ref, *rec)
	a.logf("%s: the push was refused as the branch moved on; fix run %s started (%d/%d)", ref, run, fix.attempt, a.Config.Agent.FixAttempts())
	a.Audit.Log("run.fix", map[string]any{"issue": ref.String(), "run": run, "cause": fix.cause, "attempt": fix.attempt})
	return a.work(ctx, pl, *rec)
}

// pushDelays are the waits between the tries of a push that failed for a moment: three tries in all.
var pushDelays = []time.Duration{5 * time.Second, 20 * time.Second}

// pusher pushes an issue branch; *workspace.Space is one.
type pusher interface {
	Push(ctx context.Context) error
}

// push pushes the issue branch, trying again after a transient failure.
func (a *Agent) push(ctx context.Context, sp pusher) error {
	delays := pushDelays
	if a.PushDelays != nil {
		delays = a.PushDelays
	}
	for i := 0; ; i++ {
		err := sp.Push(ctx)
		var pe *workspace.PushError
		if err == nil || !errors.As(err, &pe) || pe.Kind != workspace.PushTransient || i == len(delays) {
			return err
		}
		a.logf("push failed for a moment, trying again in %s: %s", delays[i], tail(workspace.Redact(pe.Output)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delays[i]):
		}
	}
}
