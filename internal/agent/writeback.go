package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/workspace"
)

// outcome is what a run writes back: one comment and the state label. It is saved before the comment is posted,
// so that an agent stopped in the middle of a write-back finishes it on its next start without posting twice.
type outcome struct {
	Run     string `json:"run"`
	Session string `json:"session"`
	State   string `json:"state"`  // waiting or done
	Reason  string `json:"reason"` // for waiting: question, permission, error, verify
	Ack     int64  `json:"ack"`
	Body    string `json:"body"` // the comment without its marker
	PR      string `json:"pr,omitempty"`

	msg texts
}

func (o *outcome) result() string {
	if o.Reason == "" {
		return o.State
	}
	return o.State + " (" + o.Reason + ")"
}

func (o *outcome) set(state, reason, title string, parts ...string) *outcome {
	o.State, o.Reason = state, reason
	o.Body = "### " + title + "\n\n" + strings.Join(parts, "\n\n")
	return o
}

func (o *outcome) fail(why string) *outcome {
	return o.set("waiting", "error", o.msg.errorTitle, why, o.msg.reply)
}

func (o *outcome) verify(why string) *outcome {
	return o.set("waiting", "verify", o.msg.verifyTitle, why, o.msg.reply)
}

func (o *outcome) wait(d *Declaration, denials []claudecode.Denial) *outcome {
	parts := []string{"**" + o.msg.soFar + "**\n\n" + d.Summary}
	if len(denials) > 0 {
		var lines []string
		for _, dn := range denials {
			what := dn.ToolName
			if c := dn.Command(); c != "" {
				what += ": `" + strings.ReplaceAll(c, "`", "'") + "`"
			}
			lines = append(lines, "- "+what)
		}
		parts = append(parts, "**"+o.msg.denied+"**\n\n"+strings.Join(lines, "\n"))
	}
	if q := strings.TrimSpace(d.Question); q != "" {
		parts = append(parts, "**"+o.msg.question+"**\n\n"+q)
	} else {
		parts = append(parts, o.msg.noQuestion)
	}
	if len(denials) > 0 {
		return o.set("waiting", "permission", o.msg.permissionTitle, parts...)
	}
	return o.set("waiting", "question", o.msg.questionTitle, parts...)
}

func (o *outcome) done(d *Declaration, url string) *outcome {
	o.PR = url
	return o.set("done", "", o.msg.doneTitle, fmt.Sprintf(o.msg.pr, url), d.Summary)
}

func (a *Agent) runDir(run string) string { return filepath.Join(a.State.Path, "runs", run) }

// writeBack posts the outcome's comment unless a comment of the same run exists, then sets the state label.
func (a *Agent) writeBack(ctx context.Context, ref forge.IssueRef, o *outcome) error {
	if err := a.State.WriteJSON(filepath.Join("runs", o.Run, "outcome.json"), o); err != nil {
		return err
	}
	_, comments, err := a.Forge.Thread(ctx, ref)
	if err != nil {
		return err
	}
	posted := false
	for _, c := range comments {
		if m, ok := marker.Parse(c.Body); ok && c.Author.ID == a.Judge.AgentID && m.Run == o.Run {
			posted = true
		}
	}
	if !posted {
		body := o.Body
		if workspace.HasSecret(body) {
			body = fmt.Sprintf(o.msg.withheld, o.Run)
			a.Audit.Log("comment.withheld", map[string]any{"issue": ref.String(), "run": o.Run})
		}
		m := marker.Marker{Host: a.Config.Host.Name, Session: o.Session, Run: o.Run, State: o.State, Reason: o.Reason, Ack: o.Ack, At: time.Now()}
		body += "\n\n" + m.Format() + "\n"
		a.checkComment(ref, o.Run, body)
		c, err := a.Forge.PostComment(ctx, ref, body)
		if err != nil {
			return fmt.Errorf("post comment: %w", err)
		}
		a.Audit.Log("comment.posted", map[string]any{"issue": ref.String(), "run": o.Run, "comment": c.ID, "state": o.State, "reason": o.Reason})
	}
	to := a.Config.Labels.Waiting
	if o.State == "done" {
		to = a.Config.Labels.Done
	}
	return a.swapper().Swap(ctx, ref, to)
}

// checkComment runs comments.check_command on the comment file. A failure is recorded; the comment is posted anyway.
func (a *Agent) checkComment(ref forge.IssueRef, run, body string) {
	cmd := a.Config.Comments.CheckCommand
	if cmd == "" {
		return
	}
	path := filepath.Join(a.runDir(run), "comment.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return
	}
	out, err := a.shell(a.ctx, a.runDir(run), a.Env, cmd+` "$1"`, path)
	if err != nil {
		a.logf("%s: check_command failed on the comment of run %s; posting it anyway", ref, run)
		a.Audit.Log("comment.check_failed", map[string]any{"issue": ref.String(), "run": run, "error": err.Error(), "output": tail(out)})
	}
}

// recover finishes the runs an earlier agent left in the running state: a saved outcome is written back as it
// is; a run without one becomes waiting with reason error.
func (a *Agent) recover(ctx context.Context) error {
	var errs []error
	for _, p := range a.projects {
		list, err := a.Forge.SearchOpenIssuesWithLabel(ctx, "repo:"+p.FullName(), a.Config.Labels.Running)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.FullName(), err))
			continue
		}
		for _, s := range list {
			if s.Ref.Owner != p.Owner || s.Ref.Repo != p.Repo {
				continue
			}
			if err := a.recoverOne(ctx, s.Ref); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.Ref, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (a *Agent) recoverOne(ctx context.Context, ref forge.IssueRef) error {
	rec, _ := a.loadRecord(ref)
	o := &outcome{msg: a.msg}
	if rec != nil && a.State.ReadJSON(filepath.Join("runs", rec.Run, "outcome.json"), o) == nil {
		o.msg = a.msg
		a.Audit.Log("run.recovered", map[string]any{"issue": ref.String(), "run": o.Run, "how": "finish the write-back"})
	} else {
		o.Run, o.Session = newID(), ""
		if rec != nil {
			o.Run, o.Session, o.Ack = rec.Run, rec.Session, rec.Ack
		}
		o.fail(a.msg.interrupted)
		a.Audit.Log("run.recovered", map[string]any{"issue": ref.String(), "run": o.Run, "how": "interrupted"})
	}
	if err := a.writeBack(ctx, ref, o); err != nil {
		return err
	}
	if rec == nil {
		rec = &record{Issue: ref.String(), Run: o.Run}
	}
	rec.State, rec.Ended, rec.Result = "finished", time.Now().UTC(), o.result()
	a.saveRecord(ref, *rec)
	a.logf("%s: recovered run %s: %s", ref, o.Run, o.result())
	return nil
}

// record is the state of an issue's latest run on this host, kept in issues/<owner>/<repo>/<n>.json.
type record struct {
	Issue   string    `json:"issue"`
	Run     string    `json:"run"`
	Session string    `json:"session"`
	State   string    `json:"state"` // running, finished, canceled
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitempty"`
	Ack     int64     `json:"ack"`
	Result  string    `json:"result,omitempty"`
}

func recordName(ref forge.IssueRef) string {
	return filepath.Join("issues", ref.Owner, ref.Repo, fmt.Sprintf("%d.json", ref.Number))
}

func (a *Agent) saveRecord(ref forge.IssueRef, rec record) {
	if err := a.State.WriteJSON(recordName(ref), rec); err != nil {
		a.logf("%s: save the run record: %v", ref, err)
	}
}

func (a *Agent) loadRecord(ref forge.IssueRef) (*record, error) {
	var rec record
	if err := a.State.ReadJSON(recordName(ref), &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}
