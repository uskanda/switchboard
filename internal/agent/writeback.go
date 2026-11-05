package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/notify"
	"github.com/uskanda/switchboard/internal/postfmt"
	"github.com/uskanda/switchboard/internal/workspace"
)

// outcome is what a run writes back: one comment and the state label. It is saved before the comment is posted,
// so that an agent stopped in the middle of a write-back finishes it on its next start without posting twice.
type outcome struct {
	Run     string `json:"run"`
	Session string `json:"session"`
	State   string `json:"state"`  // waiting, done, or queued for a run that hit a usage limit
	Reason  string `json:"reason"` // for waiting: question, permission, error, verify, push, ci, conflict
	Ack     int64  `json:"ack"`
	Body    string `json:"body"` // the comment without its marker
	PR      string `json:"pr,omitempty"`
	Q       int    `json:"q,omitempty"`   // for plan-questions: the number of the last question
	SHA     string `json:"sha,omitempty"` // for done: the commit pushed, whose CI is watched
	CR      int    `json:"cr,omitempty"`  // for done: the number of the change request
	Fix     string `json:"fix,omitempty"` // for done of a fix run: what it fixed (marker.CI, marker.Push or marker.Conflict)
	// For done: the images kept in the run directory's screenshots/ to attach, the files left out, and why the
	// images could not be collected (ADR-051).
	Shots      []string            `json:"shots,omitempty"`
	Skipped    []workspace.Skipped `json:"skipped,omitempty"`
	ShotsError string              `json:"shots_error,omitempty"`
	// Until, for a run that hit a usage limit (queued, or a fix run's wait): when the host starts runs again.
	Until time.Time `json:"until,omitempty"`

	msg texts
	// next, set instead of a state, hands the issue to a fix run of a refused push; such an outcome is not written.
	next *fixInfo
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

// limited puts the issue of a run that hit a usage limit back in the queue: its comment is the queued comment that
// marks the issue to resume once the host's pause ends at until (ADR-053).
func (o *outcome) limited(host string, until time.Time) *outcome {
	o.Until = until
	return o.set(marker.Queued, marker.QueuedLimit, o.msg.limitTitle, fmt.Sprintf(o.msg.limitQueued, host, o.msg.when(until)))
}

// limitedFix ends a fix run that hit a usage limit: it waits with reason error, and until keeps it out of the count
// of fix runs.
func (o *outcome) limitedFix(host string, until time.Time) *outcome {
	o.fail(fmt.Sprintf(o.msg.limitFix, host, o.msg.when(until)))
	o.Until = until
	return o
}

func (o *outcome) verify(why string) *outcome {
	return o.set("waiting", "verify", o.msg.verifyTitle, why, o.msg.reply)
}

func (o *outcome) wait(d *Declaration, denials []claudecode.Denial) *outcome {
	parts := []string{"**" + o.msg.soFar + "**\n\n" + d.Summary}
	if len(denials) > 0 {
		parts = append(parts, "**"+o.msg.denied+"**\n\n"+deniedList(denials))
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

// done records the change request at url and the commit sha pushed to it; link is the comment's line that names it
// ("Pull request: ...").
func (o *outcome) done(d *Declaration, url, sha, link string) *outcome {
	o.PR, o.SHA, o.CR = url, sha, forge.ChangeRequestNumber(url)
	return o.set("done", "", o.msg.doneTitle, link, d.Summary)
}

// again hands the issue to a fix run of a refused push instead of writing an outcome.
func (o *outcome) again(fix *fixInfo) *outcome {
	o.next = fix
	return o
}

// pushRefused writes back a push that the agent does not fix: its kind, git's output (redacted), and what a person
// can do. done and limit are the fix runs made and allowed, for a branch that moved on.
func (o *outcome) pushRefused(kind, output string, done, limit int) *outcome {
	var why string
	switch kind {
	case workspace.PushAuth:
		why = o.msg.pushAuth
	case workspace.PushNonFastForward:
		why = fmt.Sprintf(o.msg.pushMoved, done, limit)
	default:
		why = o.msg.pushRule
	}
	return o.set("waiting", marker.Push, o.msg.pushTitle, why, "**"+o.msg.gitOutput+"**\n\n```\n"+tail(output)+"\n```", o.msg.reply)
}

func (a *Agent) runDir(run string) string { return filepath.Join(a.State.Path, "runs", run) }

// writeBack posts the outcome's comment unless a comment of the same run exists, then sets the state label.
func (a *Agent) writeBack(ctx context.Context, ref forge.IssueRef, o *outcome) error {
	return a.writeOut(ctx, ref, o, true)
}

// writeOut is writeBack; without setLabel, it only posts the comment, and sends no notification.
func (a *Agent) writeOut(ctx context.Context, ref forge.IssueRef, o *outcome, setLabel bool) error {
	if err := a.State.WriteJSON(filepath.Join("runs", o.Run, "outcome.json"), o); err != nil {
		return err
	}
	is, comments, err := a.forgeOf(ref).Thread(ctx, ref)
	if err != nil {
		return err
	}
	posted := false
	for _, c := range comments {
		// The claim of the run carries the same run; only a comment in the outcome's state counts as posted.
		if m, ok := marker.Parse(c.Body); ok && c.Author.ID == a.judgeOf(ref).AgentIDOf(ref) && m.Run == o.Run && m.State == o.State {
			posted = true
		}
	}
	if !posted {
		body := o.Body
		if workspace.HasSecret(body) {
			body = fmt.Sprintf(o.msg.withheld, o.Run)
			a.Audit.Log("comment.withheld", map[string]any{"issue": ref.String(), "run": o.Run})
		}
		posting := body
		m := marker.Marker{Host: a.Config.Host.Name, Session: o.Session, Run: o.Run, State: o.State, Reason: o.Reason, Ack: o.Ack, At: time.Now(), Q: o.Q, SHA: o.SHA, CR: o.CR, Until: o.Until}
		if o.State == marker.Done {
			m.Fix = o.Fix
		}
		body += "\n\n" + m.Format() + "\n"
		a.checkComment(ref, o.Run, "comment.md", body)
		c, err := a.forgeOf(ref).PostComment(ctx, ref, body)
		if err != nil {
			return fmt.Errorf("post comment: %w", err)
		}
		a.Audit.Log("comment.posted", map[string]any{"issue": ref.String(), "run": o.Run, "comment": c.ID, "state": o.State, "reason": o.Reason})
		// The recovery of a write-back finds this comment and posts nothing, so the notification goes out now,
		// after the label, even when setting the label fails.
		if setLabel && o.State != marker.Queued { // a queue is never notified
			defer a.notify(ctx, &is.IssueSummary, o, posting)
		}
	}
	a.dropProgress(ctx, ref, o.Run, comments)
	if setLabel {
		to := a.Config.Labels.Waiting
		switch o.State {
		case "done":
			to = a.Config.Labels.Done
		case marker.Queued:
			to = a.Config.Labels.Ready
		}
		if err := a.swapper(ref).Swap(ctx, ref, to); err != nil {
			return err
		}
	}
	return a.attach(ctx, ref, o, comments)
}

// attach posts the comment with the images of a done run, after its done post and its label (ADR-051), unless
// comments hold one of the same run. When the images cannot be attached, the comment says why instead; the done
// stays. Only a failure to post that comment fails the write-back, which is then tried again.
func (a *Agent) attach(ctx context.Context, ref forge.IssueRef, o *outcome, comments []forge.Comment) error {
	if o.State != "done" || len(o.Shots) == 0 && len(o.Skipped) == 0 && o.ShotsError == "" {
		return nil
	}
	agentID := a.judgeOf(ref).AgentIDOf(ref)
	for _, c := range comments {
		if m, ok := marker.Parse(c.Body); ok && c.Author.ID == agentID && m.Run == o.Run && m.State == marker.Attachment {
			return nil
		}
	}
	body := "### " + a.msg.shotsTitle
	if len(o.Skipped) > 0 {
		var lines []string
		for _, s := range o.Skipped {
			lines = append(lines, "- `"+strings.ReplaceAll(s.Name, "`", "'")+"`: "+a.skipReason(s.Reason))
		}
		list := strings.Join(lines, "\n")
		if workspace.HasSecret(list) {
			list = fmt.Sprintf(a.msg.withheld, o.Run) // the names come from the run
		}
		body += "\n\n**" + a.msg.shotsSkipped + "**\n\n" + list
	}
	m := marker.Marker{Host: a.Config.Host.Name, Session: o.Session, Run: o.Run, State: marker.Attachment, At: time.Now()}
	why := o.ShotsError
	if len(o.Shots) > 0 && why == "" {
		files := make([]string, len(o.Shots))
		for i, n := range o.Shots {
			files[i] = filepath.Join(a.runDir(o.Run), "screenshots", n)
		}
		at, ok := a.forgeOf(ref).(forge.Attacher)
		if !ok {
			why = a.msg.shotsNoAttach
		} else {
			c, err := at.AttachComment(ctx, ref, body+"\n\n"+m.Format()+"\n", files)
			if err == nil {
				a.Audit.Log("attachment.posted", map[string]any{"issue": ref.String(), "run": o.Run, "comment": c.ID, "files": len(files)})
				return nil
			}
			if ctx.Err() != nil {
				return err
			}
			why = err.Error()
		}
	}
	if why != "" {
		why = workspace.Redact(why)
		a.Audit.Log("attachment.failed", map[string]any{"issue": ref.String(), "run": o.Run, "error": why})
		body += "\n\n" + fmt.Sprintf(a.msg.shotsFailed, why)
	}
	c, err := a.forgeOf(ref).PostComment(ctx, ref, body+"\n\n"+m.Format()+"\n")
	if err != nil {
		return fmt.Errorf("post the screenshots comment: %w", err)
	}
	a.Audit.Log("attachment.posted", map[string]any{"issue": ref.String(), "run": o.Run, "comment": c.ID, "files": 0})
	return nil
}

// skipReason is the text of why a file was not attached.
func (a *Agent) skipReason(reason string) string {
	switch reason {
	case workspace.SkipTooLarge:
		return fmt.Sprintf(a.msg.shotsTooLarge, workspace.MaxScreenshotSize>>20)
	case workspace.SkipTooMany:
		return fmt.Sprintf(a.msg.shotsTooMany, workspace.MaxScreenshots)
	}
	return a.msg.shotsNotImage
}

// checkComment runs comments.check_command on the comment, written to name in the run directory. A failure is
// recorded; the comment is posted anyway.
func (a *Agent) checkComment(ref forge.IssueRef, run, name, body string) {
	cmd := a.Config.Comments.CheckCommand
	if cmd == "" {
		return
	}
	path := filepath.Join(a.runDir(run), name)
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
// is; a run without one becomes waiting with reason error. A write-back that fails is tried again at each poll; only
// a failed search fails the start.
func (a *Agent) recover(ctx context.Context) error {
	var errs []error
	for _, p := range a.projectList() {
		list, err := a.routeOf(p).Forge.SearchOpenIssuesWithLabel(ctx, forge.Projects(p.FullName()), a.Config.Labels.Running)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.FullName(), err))
			continue
		}
		for _, s := range list {
			if !inProject(s.Ref, p) {
				continue
			}
			if err := a.recoverOne(ctx, p, s.Ref); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", s.Ref, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (a *Agent) recoverOne(ctx context.Context, p config.Project, ref forge.IssueRef) error {
	// The issue runs on the host that claimed it last. When that is another host (both by mistake assigned the
	// project), its run may still be going, and it is that host's to write back.
	// When the issue cannot be read, it is recovered as before: only a failed search fails the start.
	if host, err := a.lastClaimHost(ctx, ref); err != nil {
		if ctx.Err() != nil {
			return err
		}
		a.logf("%s: read the claims before recovering: %v", ref, err)
	} else if host != "" && host != a.Config.Host.Name {
		a.logf("%s: left running: host %s claimed it last", ref, host)
		a.Audit.Log("run.not_recovered", map[string]any{"issue": ref.String(), "host": host})
		return nil
	}
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
	if rec == nil {
		rec = &record{Issue: ref.String(), Run: o.Run}
	}
	if err := a.writeBack(ctx, ref, o); err != nil {
		if ctx.Err() != nil {
			return err
		}
		a.keepWriteBack(ref, &pendingWriteBack{project: p, o: o, recovered: true}, *rec, err)
		return nil
	}
	rec.State, rec.Ended, rec.Result = "finished", time.Now().UTC(), o.result()
	a.saveRecord(ref, *rec)
	a.logf("%s: recovered run %s: %s", ref, o.Run, o.result())
	return nil
}

// lastClaimHost is the host of the agent's latest claim on the issue, or "" when there is none.
func (a *Agent) lastClaimHost(ctx context.Context, ref forge.IssueRef) (string, error) {
	_, comments, err := a.forgeOf(ref).Thread(ctx, ref)
	if err != nil {
		return "", err
	}
	agentID := a.judgeOf(ref).AgentIDOf(ref)
	for i := len(comments) - 1; i >= 0; i-- {
		c := comments[i]
		if m, ok := marker.Parse(c.Body); ok && c.Author.ID == agentID && m.State == marker.Running {
			return m.Host, nil
		}
	}
	return "", nil
}

// pendingWriteBack is a run of this host whose write-back did not finish. Each poll tries it again until it is
// written back, or until the issue no longer waits for it (design D3).
type pendingWriteBack struct {
	project config.Project // also after the project left the configuration: the run's result is still posted
	o       *outcome
	// interrupted: the run never ran (its start failed after the claim); o says so, and is not posted on an issue
	// someone took over.
	interrupted bool
	// recovered: kept by the recovery at start, which does not check whose claim the issue carries.
	recovered bool
}

// keepWriteBack keeps the write-back w of ref, which failed with err, for the next polls, and records the run as
// waiting for it.
func (a *Agent) keepWriteBack(ref forge.IssueRef, w *pendingWriteBack, rec record, err error) {
	a.mu.Lock()
	a.writebacks[ref] = w
	a.mu.Unlock()
	rec.State, rec.Ended = "writeback", time.Now().UTC()
	a.saveRecord(ref, rec)
	a.logf("%s: write back run %s: %v; tried again at each poll", ref, w.o.Run, err)
	a.Audit.Log("writeback.failed", map[string]any{"issue": ref.String(), "run": w.o.Run, "error": err.Error()})
}

// retryWriteBacks tries again the kept write-backs of the issues that do not run. Call it with pollMu held.
func (a *Agent) retryWriteBacks(ctx context.Context) {
	a.mu.Lock()
	refs := make([]forge.IssueRef, 0, len(a.writebacks))
	for ref := range a.writebacks {
		if a.active[ref] == nil {
			refs = append(refs, ref)
		}
	}
	a.mu.Unlock()
	sort.Slice(refs, func(i, j int) bool { return refs[i].String() < refs[j].String() })
	for _, ref := range refs {
		if ctx.Err() != nil {
			return
		}
		a.mu.Lock()
		w := a.writebacks[ref]
		a.mu.Unlock()
		a.retryWriteBack(ctx, ref, w)
	}
}

// retryWriteBack writes back w when the issue still waits for it: open, labeled running, and, unless w comes from
// the recovery, last claimed by w's run. Otherwise someone took the issue over: the labels stay, and only a result
// not yet posted is posted.
func (a *Agent) retryWriteBack(ctx context.Context, ref forge.IssueRef, w *pendingWriteBack) {
	is, comments, err := a.forgeOf(ref).Thread(ctx, ref)
	if err != nil {
		a.logf("%s: write back run %s: %v; tried again at the next poll", ref, w.o.Run, err)
		return
	}
	n := a.Config.Labels
	if is.State == "open" && labels.StateOf(is.Labels, n) == n.Running && (w.recovered || a.lastClaim(ref, comments) == w.o.Run) {
		if err := a.writeBack(ctx, ref, w.o); err != nil {
			a.logf("%s: write back run %s: %v; tried again at the next poll", ref, w.o.Run, err)
			return
		}
		a.dropWriteBack(ref)
		a.Audit.Log("writeback.retried", map[string]any{"issue": ref.String(), "run": w.o.Run, "state": w.o.State})
		a.ended(w.project, ref, a.recordOf(ref, w.o.Run), w.o)
		if w.o.State == "waiting" || w.o.State == "done" || w.o.State == marker.Queued {
			a.resume(w.project, ref)
		}
		return
	}
	if !w.interrupted {
		if err := a.writeOut(ctx, ref, w.o, false); err != nil {
			a.logf("%s: post the result of run %s: %v; tried again at the next poll", ref, w.o.Run, err)
			return
		}
	}
	a.dropWriteBack(ref)
	rec := a.recordOf(ref, w.o.Run)
	rec.State, rec.Ended, rec.Result = "finished", time.Now().UTC(), w.o.result()
	a.saveRecord(ref, rec)
	a.logf("%s: run %s is no longer what the issue waits for; its labels are left as they are", ref, w.o.Run)
	a.Audit.Log("writeback.abandoned", map[string]any{"issue": ref.String(), "run": w.o.Run, "state": is.State, "labels": is.Labels})
}

func (a *Agent) dropWriteBack(ref forge.IssueRef) {
	a.mu.Lock()
	delete(a.writebacks, ref)
	a.mu.Unlock()
}

// lastClaim is the run of the latest claim the agent posted on the issue, or "".
func (a *Agent) lastClaim(ref forge.IssueRef, comments []forge.Comment) string {
	agentID := a.judgeOf(ref).AgentIDOf(ref)
	var last *forge.Comment
	run := ""
	for i, c := range comments {
		m, ok := marker.Parse(c.Body)
		if !ok || c.Author.ID != agentID || m.State != "running" {
			continue
		}
		if last == nil || c.ID > last.ID {
			last, run = &comments[i], m.Run
		}
	}
	return run
}

// recordOf is the issue's run record when it is of run, else a new one.
func (a *Agent) recordOf(ref forge.IssueRef, run string) record {
	if rec, err := a.loadRecord(ref); err == nil && rec.Run == run {
		return *rec
	}
	return record{Issue: ref.String(), Run: run}
}

// record is the state of an issue's latest run on this host, kept in issues/<owner>/<repo>/<n>.json.
type record struct {
	Issue   string    `json:"issue"`
	Run     string    `json:"run"`
	Session string    `json:"session"`
	State   string    `json:"state"` // running, writeback (its write-back is tried again), finished, canceled
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitempty"`
	Ack     int64     `json:"ack"`
	Result  string    `json:"result,omitempty"`
}

// recordName keeps the Phase 1 place for GitHub and puts GitLab under "@gitlab", which no GitHub owner can be.
func recordName(ref forge.IssueRef) string {
	if ref.Kind() == forge.GitLab {
		return filepath.Join("issues", "@gitlab", ref.Owner, ref.Repo, fmt.Sprintf("%d.json", ref.Number))
	}
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

// Notifier tells the user about an issue outside the forge.
type Notifier interface {
	Send(ctx context.Context, text string) error
}

// notify sends the notification of a comment just posted with body (without its marker). A failure is recorded and
// changes nothing else.
func (a *Agent) notify(ctx context.Context, is *forge.IssueSummary, o *outcome, body string) {
	if a.Notifier == nil {
		return
	}
	ref, lang := is.Ref.String(), a.Config.Comments.Language
	var text string
	if o.State == "done" {
		text = notify.Done{Ref: ref, ChangeURL: o.PR, IssueURL: is.URL}.Text(lang)
	} else {
		// A plan-stage comment opens with the plan's title, which Question quotes.
		text = notify.Waiting{
			Ref: ref, Title: is.Title, Reason: o.Reason, Question: postfmt.Question(body),
			IssueURL: is.URL, WebURL: a.Config.Notify.WebURL,
		}.Text(lang)
	}
	if err := a.Notifier.Send(ctx, text); err != nil {
		a.Audit.Log("notify.failed", map[string]any{"issue": ref, "kind": o.State, "reason": err.Error()})
		a.logf("%s: the notification could not be sent: %v", ref, err)
	}
}
