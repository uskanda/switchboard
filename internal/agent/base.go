package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/workspace"
)

// The states of the CI at the head of a base branch.
const (
	baseGreen   = "green"   // every check ended and none failed, or there is no CI
	baseRed     = "red"     // every check ended and some failed
	basePending = "pending" // some check still runs
)

// baseRead is the CI at the head of a base branch, as read in a cycle of the watches.
type baseRead struct {
	cycle  int
	sha    string
	state  string
	failed []forge.Check
}

func baseKey(p config.Project) string { return p.Ref + "\x00" + p.BaseBranch }

// readBase reads the CI at the head of the base branch of p, at most once per cycle of the watches, so that the
// change requests that wait on one base read it once per poll (ADR-075).
func (a *Agent) readBase(ctx context.Context, ref forge.IssueRef, p config.Project, ci forge.CI) (*baseRead, error) {
	key := baseKey(p)
	a.mu.Lock()
	b, cycle := a.bases[key], a.cycle
	a.mu.Unlock()
	if b != nil && b.cycle == cycle {
		return b, nil
	}
	sha, err := ci.BranchHead(ctx, ref.Owner, ref.Repo, p.BaseBranch)
	if err != nil {
		return nil, err
	}
	checks, err := ci.CommitChecks(ctx, ref.Owner, ref.Repo, sha)
	if err != nil {
		return nil, err
	}
	b = &baseRead{cycle: cycle, sha: sha, state: baseGreen}
	for _, c := range checks {
		if !c.Done {
			b.state, b.failed = basePending, nil
			break
		}
		if c.Failed() {
			b.failed = append(b.failed, c)
		}
	}
	if b.state == baseGreen && len(b.failed) > 0 {
		b.state = baseRed
	}
	a.mu.Lock()
	a.bases[key] = b
	a.mu.Unlock()
	return b, nil
}

// baseOf reads the base of w for a decision on ref, or reports why it cannot tell: ok is false when the decision
// waits for the next poll (a rate limit, or a read that failed), and b is nil when the base cannot be read at all
// (no such branch, or a token that may not read it), so that the change request is judged as before.
func (a *Agent) baseOf(ctx context.Context, ref forge.IssueRef, w *watch, ci forge.CI) (b *baseRead, ok bool) {
	b, err := a.readBase(ctx, ref, w.project, ci)
	switch {
	case err == nil:
		return b, true
	case a.noteLimit(ref, err):
		return nil, false
	case errors.Is(err, forge.ErrNotFound), errors.Is(err, forge.ErrForbidden):
		return nil, true
	}
	a.logf("%s: read the CI of the base branch %s: %v; tried again at the next poll", ref, w.project.BaseBranch, err)
	return nil, false
}

// baseHolds reports whether the fix run for ref waits for the base: for a CI that failed (failed), while the CI of the
// base still runs, or when the base is red with every check that failed on the change request; for a conflict
// (failed is nil), while the base is red. A change request that begins to wait gets its note.
func (a *Agent) baseHolds(ctx context.Context, ref forge.IssueRef, w *watch, ci forge.CI, failed []forge.Check) bool {
	b, ok := a.baseOf(ctx, ref, w, ci)
	switch {
	case !ok:
		return true
	case b == nil:
		return false
	case b.state == basePending:
		return failed != nil
	case b.state == baseGreen:
		a.mu.Lock()
		w.baseWait = false
		a.mu.Unlock()
		return false
	case failed != nil && !failsAlike(failed, b.failed):
		return false
	}
	a.endWait(w, "")
	a.mu.Lock()
	w.baseWait = true
	noted := w.baseNoted
	a.mu.Unlock()
	if !noted {
		a.noteBase(ctx, ref, w, b)
	}
	return true
}

// failsAlike reports whether every check that failed on the change request failed on the base under the same name.
func failsAlike(failed, base []forge.Check) bool {
	names := map[string]bool{}
	for _, c := range base {
		names[c.Name] = true
	}
	for _, c := range failed {
		if !names[c.Name] {
			return false
		}
	}
	return true
}

// noteBase posts the note that ref waits for its red base, once per watch; a note that fails is posted again at the
// next poll.
func (a *Agent) noteBase(ctx context.Context, ref forge.IssueRef, w *watch, b *baseRead) {
	m := marker.Marker{Host: a.Config.Host.Name, State: marker.Note, Reason: marker.Base, At: a.now()}
	text := fmt.Sprintf(a.msg.baseNote, w.project.BaseBranch, checkNames(b.failed))
	if _, err := a.forgeOf(ref).PostComment(ctx, ref, text+"\n\n"+m.Format()+"\n"); err != nil {
		a.logf("%s: post the note of the wait for the base: %v; tried again at the next poll", ref, err)
		return
	}
	a.mu.Lock()
	w.baseNoted = true
	a.mu.Unlock()
	a.logf("%s: the CI of the base branch %s fails as well (%s); waits until it passes again", ref, w.project.BaseBranch, b.sha)
	a.Audit.Log("ci.base.red", map[string]any{"issue": ref.String(), "base": w.project.BaseBranch, "sha": b.sha, "checks": checkNames(b.failed)})
}

// checkBaseWait reads the base of a change request that waits for it, and once the base is green again, takes it in.
func (a *Agent) checkBaseWait(ctx context.Context, ref forge.IssueRef, w *watch, ci forge.CI) {
	b, ok := a.baseOf(ctx, ref, w, ci)
	switch {
	case !ok:
		return
	case b != nil && b.state != baseGreen:
		return
	}
	a.mu.Lock()
	w.baseWait = false
	a.mu.Unlock()
	a.takeBase(ctx, ref, w)
}

// takeBase merges the base, green again, into the issue branch of ref without claude, pushes it, and posts a done of
// fix=base for the new commit, which is then watched (ADR-075). The merge is no fix run: it takes no slot and is not
// counted. A conflict goes to a conflict fix run, which is counted; nothing to merge ends the CI stage. A failure
// that may pass is tried again at the next poll.
func (a *Agent) takeBase(ctx context.Context, ref forge.IssueRef, w *watch) {
	retry := func(what string, err error) {
		a.mu.Lock()
		w.baseWait = true
		a.mu.Unlock()
		a.logf("%s: %s: %v; tried again at the next poll", ref, what, err)
	}
	unlock, err := a.State.Lock(lockName(ref))
	if err != nil {
		retry("lock the issue to merge the base", err)
		return
	}
	defer func() { unlock() }()
	sp := a.spaceOf(w.project, ref)
	got, err := sp.MergeBase(ctx, w.sha)
	switch {
	case errors.Is(err, workspace.ErrMoved):
		a.unwatch(ref, "the issue branch moved on; the base is not merged")
		return
	case errors.Is(err, workspace.ErrUncommitted):
		a.unwatch(ref, "the worktree has uncommitted changes; the base is not merged")
		return
	case err != nil:
		retry("merge the base branch "+w.project.BaseBranch, err)
		return
	}
	switch got {
	case workspace.MergeConflict:
		a.Audit.Log("ci.base.conflict", map[string]any{"issue": ref.String(), "base": w.project.BaseBranch})
		unlock() // the conflict fix run takes the issue's lock
		unlock = func() {}
		a.conflicted(ctx, ref, w)
		return
	case workspace.MergeNone:
		a.Audit.Log("ci.base.noop", map[string]any{"issue": ref.String(), "base": w.project.BaseBranch})
		a.ciOver(ref, w, "the base passes again with nothing to merge")
		return
	}
	if err := sp.Push(ctx); err != nil {
		var pe *workspace.PushError
		if errors.As(err, &pe) && pe.Kind == workspace.PushNonFastForward {
			a.unwatch(ref, "the issue branch moved on; the base is not merged")
			return
		}
		retry("push the merge of the base", err)
		return
	}
	sha, err := sp.Head(ctx)
	if err != nil {
		retry("read the merge of the base", err)
		return
	}
	a.Audit.Log("ci.base.merged", map[string]any{"issue": ref.String(), "base": w.project.BaseBranch, "sha": sha})
	a.logf("%s: merged the base branch %s, green again, into %s", ref, w.project.BaseBranch, sha)
	o := &outcome{Run: newID(), State: marker.Done, SHA: sha, CR: w.cr, Fix: marker.Base, msg: a.msg}
	o.set(marker.Done, "", a.msg.doneTitle, fmt.Sprintf(a.msg.baseMerged, w.project.BaseBranch), a.changeLine(ctx, ref, w))
	a.mu.Lock()
	w.baseDone = o
	a.mu.Unlock()
	a.writeBaseDone(ctx, ref, w)
}

// changeLine is the line of a done post that names the change request of w, or "" when the forge cannot tell.
func (a *Agent) changeLine(ctx context.Context, ref forge.IssueRef, w *watch) string {
	url, err := a.forgeOf(ref).OpenChangeRequest(ctx, ref.Owner, ref.Repo, a.spaceOf(w.project, ref).IssueBranch(), w.project.BaseBranch)
	if err != nil {
		return ""
	}
	if ref.Kind() == forge.GitLab {
		return fmt.Sprintf(a.msg.mrUpdated, url)
	}
	return fmt.Sprintf(a.msg.prUpdated, url)
}

// writeBaseDone posts the done of the merged base, kept on w until it is posted, and watches its commit.
func (a *Agent) writeBaseDone(ctx context.Context, ref forge.IssueRef, w *watch) {
	o := w.baseDone
	if err := a.writeOut(ctx, ref, o, false); err != nil {
		a.logf("%s: write back the merge of the base: %v; tried again at the next poll", ref, err)
		return
	}
	a.startWatch(w.project, ref, o.SHA, o.CR, a.now())
}
