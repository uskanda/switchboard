package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// baseHead is the commit at the head of main in the tests of the red base.
const baseHead = "base1"

// setBase moves main to commit sha and gives it checks.
func (w *ciWorld) setBase(sha string, checks ...forge.Check) {
	w.f.SetBranchHead("o", "r", "main", sha)
	w.f.SetChecks("o", "r", sha, checks...)
}

// notes are the agent's notes on ref (state=note).
func (w *ciWorld) notes(ref forge.IssueRef) []post {
	var out []post
	for _, p := range w.posts(ref) {
		if p.Marker != nil && p.Marker.State == marker.Note {
			out = append(out, p)
		}
	}
	return out
}

// A CI that fails as the base fails does not start a fix run and is not counted: the issue stays done, and a note
// says once that it waits for the base (ADR-075).
func TestACIFailingLikeTheBaseWaits(t *testing.T) {
	w := newCI(t, "", func(c *config.Config) { c.Comments.Language = "ja" })
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("verify", 7))
	w.setBase(baseHead, failedCheck("verify", 70))
	w.poll()
	w.settle()
	if w.starts() != 1 || len(w.claims(w.ref)) != 1 || w.labels(w.ref) != "agent:done" || !w.a.watched(w.ref) {
		t.Fatalf("starts=%d claims=%d labels=%q watched=%v: a red base must not start a fix run", w.starts(), len(w.claims(w.ref)), w.labels(w.ref), w.a.watched(w.ref))
	}
	notes := w.notes(w.ref)
	if len(notes) != 1 || notes[0].Marker.Reason != marker.Base || !strings.Contains(notes[0].Body, "baseのCIが失敗しているため、緑に戻るまで待ちます") ||
		!strings.Contains(notes[0].Body, "`verify`") || !strings.Contains(notes[0].Body, "main") {
		t.Fatalf("notes = %+v", notes)
	}
	if !w.audit.has("ci.base.red", func(e map[string]any) bool { return e["issue"] == w.ref.String() }) {
		t.Error("the wait is not in the audit log")
	}
	for i := 0; i < 10; i++ {
		w.later(w.cfg.Agent.PollInterval)
		w.poll()
		w.settle()
	}
	if w.starts() != 1 || len(w.notes(w.ref)) != 1 || w.labels(w.ref) != "agent:done" || !w.a.watched(w.ref) {
		t.Fatalf("starts=%d notes=%d labels=%q watched=%v after 10 polls", w.starts(), len(w.notes(w.ref)), w.labels(w.ref), w.a.watched(w.ref))
	}
}

// Past the fix limit, a CI failing as the base fails still waits for the base instead of the person.
func TestARedBaseDoesNotUseUpTheFixes(t *testing.T) {
	w := newCI(t, "agent:\n  max_fix_attempts: 0\n", nil)
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("verify", 7))
	w.setBase(baseHead, failedCheck("verify", 70))
	w.poll()
	w.settle()
	if w.labels(w.ref) != "agent:done" || len(w.notes(w.ref)) != 1 {
		t.Fatalf("labels=%q notes=%d: a red base must not make the issue wait", w.labels(w.ref), len(w.notes(w.ref)))
	}
}

func TestAFailureTheBaseDoesNotShareStartsAFix(t *testing.T) {
	w := newCI(t, "", nil)
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("verify", 7), failedCheck("lint", 8))
	w.setBase(baseHead, failedCheck("verify", 70))
	w.step(2, commit("fix.txt", "fixed")+done("Fixed lint."))
	w.poll()
	w.settle()
	if w.starts() != 2 || len(w.notes(w.ref)) != 0 {
		t.Fatalf("starts=%d notes=%d: a failure of the change request's own must be fixed", w.starts(), len(w.notes(w.ref)))
	}
}

func TestABaseWithoutCIIsNotRed(t *testing.T) {
	w := newCI(t, "", nil)
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("verify", 7))
	w.setBase(baseHead)
	w.step(2, commit("fix.txt", "fixed")+done("Fixed."))
	w.poll()
	w.settle()
	if w.starts() != 2 || len(w.notes(w.ref)) != 0 {
		t.Fatalf("starts=%d notes=%d: a base without CI is not red", w.starts(), len(w.notes(w.ref)))
	}
}

func TestARunningBaseCIDefersTheFix(t *testing.T) {
	w := newCI(t, "", nil)
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("verify", 7))
	w.setBase(baseHead, forge.Check{Name: "verify"})
	w.poll()
	w.settle()
	if w.starts() != 1 || len(w.notes(w.ref)) != 0 || !w.a.watching() {
		t.Fatalf("starts=%d notes=%d watching=%v: the fix waits for the base's CI to end", w.starts(), len(w.notes(w.ref)), w.a.watching())
	}
	w.f.SetChecks("o", "r", baseHead, failedCheck("verify", 70))
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
	if w.starts() != 1 || len(w.notes(w.ref)) != 1 {
		t.Fatalf("starts=%d notes=%d: the base failed the same way", w.starts(), len(w.notes(w.ref)))
	}
}

// After a restart, the watch is found again, the failure is matched against the base again, and no second note is
// posted.
func TestTheBaseWaitSurvivesARestart(t *testing.T) {
	w := newCI(t, "", nil)
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("verify", 7))
	w.setBase(baseHead, failedCheck("verify", 70))
	w.poll()
	w.settle()
	w.agentWith(func(a *Agent) { a.Now = w.now })
	w.poll()
	w.settle()
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
	if !w.a.watched(w.ref) || w.starts() != 1 || len(w.notes(w.ref)) != 1 {
		t.Fatalf("watched=%v starts=%d notes=%d after the restart", w.a.watched(w.ref), w.starts(), len(w.notes(w.ref)))
	}
}

// The agent and the trusted author share an account: the note is the agent's, not a reply that resumes the issue.
func TestTheBaseNoteIsNotAReply(t *testing.T) {
	w := newCI(t, "", nil)
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("verify", 7))
	w.setBase(baseHead, failedCheck("verify", 70))
	w.poll()
	w.settle()
	w.later(w.cfg.Agent.ReconcileInterval)
	w.poll()
	w.settle()
	if w.starts() != 1 || w.labels(w.ref) != "agent:done" {
		t.Fatalf("starts=%d labels=%q: the note resumed the issue", w.starts(), w.labels(w.ref))
	}
}

// Many issues waiting on one base read it once per poll, and not at all while the forge's API is limited.
func TestTheBaseIsReadOncePerPoll(t *testing.T) {
	w := newCI(t, "", nil)
	other := w.f.AddIssue("o", "r", 13, "Other", "Do something else.", w.master)
	w.step(2, commit("other.txt", "other")+done("Other done."))
	w.f.AddLabel(other, "agent:ready", w.master)
	w.poll()
	w.settle()
	op := w.lastPost(other)
	w.f.SetPRHead(op.Marker.CR-1, op.Marker.SHA)
	w.f.SetChecks("o", "r", op.Marker.SHA, failedCheck("verify", 8))
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("verify", 7))
	w.setBase(baseHead, failedCheck("verify", 70))
	w.poll()
	w.settle()
	if len(w.notes(w.ref)) != 1 || len(w.notes(other)) != 1 {
		t.Fatalf("notes = %d, %d", len(w.notes(w.ref)), len(w.notes(other)))
	}
	branch, ci := w.f.BranchCalls(), w.f.CICalls()
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
	if got := w.f.BranchCalls() - branch; got != 1 {
		t.Fatalf("the base was read %d times in one poll", got)
	}
	if got := w.f.CICalls() - ci; got != 1 {
		t.Fatalf("CommitChecks was called %d times in one poll; want the base's checks alone", got)
	}
	w.f.FailCI = &forge.RateLimitError{Reset: w.now().Add(30 * time.Minute), Err: forge.ErrForbidden}
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
	branch = w.f.BranchCalls()
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
	if w.f.BranchCalls() != branch || !w.a.watched(w.ref) {
		t.Fatalf("the base was read while the API was limited (%d -> %d), watched=%v", branch, w.f.BranchCalls(), w.a.watched(w.ref))
	}
}

// While the base is red, a conflict with it starts no fix run (ADR-075).
func TestAConflictWaitsWhileTheBaseIsRed(t *testing.T) {
	w := newCI(t, "", nil)
	w.passCI()
	w.setBase(baseHead, failedCheck("verify", 70))
	w.later(w.cfg.Agent.ReconcileInterval)
	w.f.SetConflict(0, true)
	w.poll()
	w.settle()
	if w.starts() != 1 || w.labels(w.ref) != "agent:done" || len(w.notes(w.ref)) != 1 || !w.a.watched(w.ref) {
		t.Fatalf("starts=%d labels=%q notes=%d watched=%v: a conflict waits for the red base", w.starts(), w.labels(w.ref), len(w.notes(w.ref)), w.a.watched(w.ref))
	}
	// The wait is read every poll_interval, not reconcile_interval.
	prs := w.f.PRCalls()
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
	if w.f.PRCalls() != prs+1 {
		t.Fatalf("change request calls %d -> %d in the base wait", prs, w.f.PRCalls())
	}
	if got := w.a.interval(); got != w.cfg.Agent.PollInterval {
		t.Fatalf("interval = %s", got)
	}
	w.step(2, commit("merge.txt", "merged")+done("Resolved the conflict."))
	w.setBase("base2", forge.Check{Name: "verify", Done: true, Conclusion: forge.Success})
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
	claims := w.claims(w.ref)
	if w.starts() != 2 || !strings.Contains(claims[len(claims)-1].Body, "(1/2)") {
		t.Fatalf("starts=%d: the conflict fix must start once the base is green", w.starts())
	}
}

// commitOn commits name with body on branch of the origin from a clone of its own, as a person or another merge does.
func (w *ciWorld) commitOn(branch, name, body string) string {
	w.t.Helper()
	dir := filepath.Join(w.t.TempDir(), "other")
	run(w.t, filepath.Dir(dir), "git", "clone", "-q", "-b", branch, w.origin, dir)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		w.t.Fatal(err)
	}
	run(w.t, dir, "git", "add", name)
	run(w.t, dir, "git", "commit", "-q", "-m", "change "+name)
	run(w.t, dir, "git", "push", "-q", "origin", branch)
	return run(w.t, dir, "git", "rev-parse", "HEAD")
}

// waitOnRedBase makes the pull request's CI fail as main's does, and lets the watch post its note.
func (w *ciWorld) waitOnRedBase() string {
	w.t.Helper()
	sha := w.syncHead()
	w.f.SetChecks("o", "r", sha, failedCheck("verify", 7))
	w.setBase(baseHead, failedCheck("verify", 70))
	w.poll()
	w.settle()
	if len(w.notes(w.ref)) != 1 {
		w.t.Fatalf("notes = %d", len(w.notes(w.ref)))
	}
	return sha
}

// greenBase moves main to a commit whose CI passes, and lets the watch see it.
func (w *ciWorld) greenBase() {
	w.t.Helper()
	w.setBase("base2", forge.Check{Name: "verify", Done: true, Conclusion: forge.Success})
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
}

// Once the base is green again, the agent merges it without claude, pushes, posts a done of fix=base, and watches
// the new commit; the fixes are not counted (ADR-075).
func TestAGreenBaseIsMergedWithoutClaude(t *testing.T) {
	w := newCI(t, "", nil)
	old := w.waitOnRedBase()
	fix := w.commitOn("main", "fix.txt", "fixed\n")
	claims := len(w.claims(w.ref))
	w.greenBase()
	p := w.lastPost(w.ref)
	if w.starts() != 1 || len(w.claims(w.ref)) != claims || w.labels(w.ref) != "agent:done" {
		t.Fatalf("starts=%d claims=%d labels=%q: the merge must not run claude or claim", w.starts(), len(w.claims(w.ref)), w.labels(w.ref))
	}
	if p.Marker.State != marker.Done || p.Marker.Fix != marker.Base || p.Marker.SHA == old || p.Marker.CR != 1 || !strings.Contains(p.Body, "https://") {
		t.Fatalf("done post = %+v\n%s", p.Marker, p.Body)
	}
	if !gitOK(w.origin, "merge-base", "--is-ancestor", fix, p.Marker.SHA) || !gitOK(w.origin, "merge-base", "--is-ancestor", p.Marker.SHA, "switchboard/issue-12") {
		t.Fatal("the issue branch on the origin does not hold the base")
	}
	if !w.audit.has("ci.base.merged", func(e map[string]any) bool { return e["issue"] == w.ref.String() }) {
		t.Error("the merge is not in the audit log")
	}
	if !w.a.watching() {
		t.Fatal("the merged commit's CI is not watched")
	}
	// The merge is not a fix: a failure of the change request's own is fixed as the first.
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("lint", 8))
	w.step(2, commit("lint.txt", "fixed")+done("Fixed lint."))
	w.later(w.cfg.Agent.PollInterval)
	w.poll()
	w.settle()
	cl := w.claims(w.ref)
	if w.starts() != 2 || !strings.Contains(cl[len(cl)-1].Body, "(1/2)") {
		t.Fatalf("starts=%d last claim:\n%s", w.starts(), cl[len(cl)-1].Body)
	}
}

// The base, green again, conflicts with the issue branch: the merge is dropped and a conflict fix run starts, counted.
func TestAGreenBaseThatConflictsStartsAConflictFix(t *testing.T) {
	w := newCI(t, "", nil)
	old := w.waitOnRedBase()
	w.commitOn("main", "flag.txt", "verbose\n")
	w.step(2, commit("merge.txt", "merged")+done("Resolved the conflict."))
	w.greenBase()
	cl := w.claims(w.ref)
	if w.starts() != 2 || !strings.Contains(cl[len(cl)-1].Body, "(1/2)") || !w.audit.has("ci.base.conflict", nil) {
		t.Fatalf("starts=%d conflict logged=%v last claim:\n%s", w.starts(), w.audit.has("ci.base.conflict", nil), cl[len(cl)-1].Body)
	}
	if run(t, w.origin, "git", "rev-parse", "switchboard/issue-12") == old {
		t.Fatal("the conflict fix pushed nothing")
	}
}

// A person pushed to the issue branch while it waited: the agent does not merge, and stops watching.
func TestAMovedBranchIsNotMerged(t *testing.T) {
	w := newCI(t, "", nil)
	w.waitOnRedBase()
	person := w.commitOn("switchboard/issue-12", "person.txt", "mine\n")
	posts := len(w.posts(w.ref))
	w.greenBase()
	if w.a.watched(w.ref) || len(w.posts(w.ref)) != posts || run(t, w.origin, "git", "rev-parse", "switchboard/issue-12") != person {
		t.Fatalf("watched=%v posts=%d: a moved branch must be left alone", w.a.watched(w.ref), len(w.posts(w.ref)))
	}
}

// The base turned green without a commit to merge, such as after a rerun: nothing is pushed, and the CI stage ends.
func TestAGreenBaseWithNothingToMerge(t *testing.T) {
	w := newCI(t, "", nil)
	old := w.waitOnRedBase()
	posts := len(w.posts(w.ref))
	w.greenBase()
	if len(w.posts(w.ref)) != posts || run(t, w.origin, "git", "rev-parse", "switchboard/issue-12") != old || w.a.watching() || !w.a.watched(w.ref) {
		t.Fatalf("posts=%d watching=%v watched=%v", len(w.posts(w.ref)), w.a.watching(), w.a.watched(w.ref))
	}
	if !w.audit.has("ci.base.noop", nil) {
		t.Error("the merge with nothing to take in is not in the audit log")
	}
}
