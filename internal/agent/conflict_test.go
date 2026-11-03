package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// watched reports whether ref is watched, in either stage.
func (a *Agent) watched(ref forge.IssueRef) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.watches[ref]
	return ok
}

// passCI makes the CI of the pull request's head pass and lets the watch see it.
func (w *ciWorld) passCI() {
	w.t.Helper()
	w.f.SetChecks("o", "r", w.syncHead(), forge.Check{Name: "test", Done: true, Conclusion: forge.Success})
	w.poll()
	w.settle()
}

func TestAPassedCIKeepsWatchingForConflicts(t *testing.T) {
	w := newCI(t, "", nil)
	w.passCI()
	if !w.a.watched(w.ref) || w.a.watching() {
		t.Fatalf("watched=%v watching=%v: after the CI passed, only the conflict watch must remain", w.a.watched(w.ref), w.a.watching())
	}
	if got := w.a.interval(); got != w.cfg.Agent.PollInterval {
		t.Fatalf("interval = %s", got)
	}
}

func TestTheConflictStageReadsOnlyTheChangeRequestEveryReconcile(t *testing.T) {
	w := newCI(t, "", nil)
	w.passCI()
	ci, prs := w.f.CICalls(), w.f.PRCalls()
	w.poll()
	if w.f.CICalls() != ci || w.f.PRCalls() != prs {
		t.Fatalf("CI calls %d->%d, change request calls %d->%d: nothing is read before reconcile_interval", ci, w.f.CICalls(), prs, w.f.PRCalls())
	}
	w.later(w.cfg.Agent.ReconcileInterval)
	w.poll()
	if w.f.CICalls() != ci || w.f.PRCalls() != prs+1 {
		t.Fatalf("CI calls %d->%d, change request calls %d->%d: want the change request read once", ci, w.f.CICalls(), prs, w.f.PRCalls())
	}
}

func TestAConflictAfterTheCIStartsAFixRun(t *testing.T) {
	w := newCI(t, "", func(c *config.Config) { c.Comments.Language = "ja" })
	w.passCI()
	w.later(2 * 24 * time.Hour)
	w.f.SetConflict(0, true)
	w.step(2, commit("merge.txt", "merged")+done("Merged main."))
	w.poll()
	w.settle()
	claims := w.claims(w.ref)
	last := claims[len(claims)-1].Body
	m, _ := marker.Parse(last)
	if w.starts() != 2 || m.Reason != marker.Fix || !strings.Contains(last, "baseとの衝突を解消します（1/2）") {
		t.Fatalf("starts=%d, the last claim:\n%s", w.starts(), last)
	}
	if sys := w.file("system-2.md"); !strings.Contains(sys, "git merge origin/main") || !strings.Contains(sys, "The change request conflicts with its base branch") {
		t.Fatalf("the fix run's system prompt does not ask for a merge of the base:\n%s", sys)
	}
	if !strings.Contains(w.file("prompt-2.md"), "<failure-") {
		t.Fatal("the fix run's prompt has no failure report")
	}
	if w.labels(w.ref) != "agent:done" {
		t.Fatalf("labels = %q", w.labels(w.ref))
	}
}

func TestAConflictOnAnOldDonePostIsFoundAfterARestart(t *testing.T) {
	w := newCI(t, "", nil)
	w.later(7 * time.Hour)
	w.agentWith(func(a *Agent) { a.Now = w.now })
	if !w.a.watched(w.ref) || w.a.watching() {
		t.Fatalf("watched=%v watching=%v: a done post older than the CI watch must be watched for conflicts only", w.a.watched(w.ref), w.a.watching())
	}
	ci := w.f.CICalls()
	w.f.SetConflict(0, true)
	w.step(2, commit("merge.txt", "merged")+done("Merged main."))
	w.later(w.cfg.Agent.ReconcileInterval)
	w.poll()
	w.settle()
	if w.starts() != 2 || w.f.CICalls() != ci {
		t.Fatalf("starts=%d CI calls %d->%d", w.starts(), ci, w.f.CICalls())
	}
}

func TestTheConflictWatchEnds(t *testing.T) {
	t.Run("merged", func(t *testing.T) {
		w := newCI(t, "", nil)
		w.passCI()
		w.f.MergePR(0)
		w.later(w.cfg.Agent.ReconcileInterval)
		w.poll()
		if w.a.watched(w.ref) || len(w.posts(w.ref)) != 1 {
			t.Fatalf("watched=%v posts=%d", w.a.watched(w.ref), len(w.posts(w.ref)))
		}
	})
	t.Run("a person pushed", func(t *testing.T) {
		w := newCI(t, "", nil)
		w.passCI()
		w.f.SetPRHead(0, "0000000000000000000000000000000000000000")
		w.f.SetConflict(0, true)
		w.later(w.cfg.Agent.ReconcileInterval)
		w.poll()
		w.settle()
		if w.a.watched(w.ref) || w.starts() != 1 || w.labels(w.ref) != "agent:done" {
			t.Fatalf("watched=%v starts=%d labels=%q", w.a.watched(w.ref), w.starts(), w.labels(w.ref))
		}
	})
	t.Run("closed issue", func(t *testing.T) {
		w := newCI(t, "", nil)
		w.passCI()
		w.f.Close(w.ref)
		w.later(w.cfg.Agent.ReconcileInterval)
		w.poll()
		if w.a.watched(w.ref) {
			t.Fatal("a closed issue is still watched")
		}
	})
}

func TestAConflictIsFixedBeforeAFailedCI(t *testing.T) {
	w := newCI(t, "", nil)
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("test", 7))
	w.f.SetConflict(0, true)
	w.step(2, commit("merge.txt", "merged")+done("Merged main."))
	w.poll()
	w.settle()
	if w.starts() != 2 || !strings.Contains(w.file("system-2.md"), "The change request conflicts with its base branch") {
		t.Fatalf("starts=%d: want one fix run of the conflict", w.starts())
	}
	if strings.Contains(w.file("prompt-2.md"), "https://ci.test/test") {
		t.Error("the conflict's fix run got the CI failure")
	}
	if !w.a.watching() {
		t.Error("the merged commit's CI is not watched")
	}
}

func TestAConflictPastTheLimitWaits(t *testing.T) {
	w := newCI(t, "agent:\n  max_fix_attempts: 0\n", nil)
	w.f.SetConflict(0, true)
	w.poll()
	w.settle()
	p := w.lastPost(w.ref)
	if w.starts() != 1 || w.labels(w.ref) != "agent:waiting" || p.Marker.Reason != marker.Conflict {
		t.Fatalf("starts=%d labels=%q marker=%+v\n%s", w.starts(), w.labels(w.ref), p.Marker, p.Body)
	}
	if !strings.Contains(p.Body, "#1") || !strings.Contains(p.Body, "max_fix_attempts is 0") {
		t.Fatalf("the wait lacks the pull request or the setting:\n%s", p.Body)
	}
	if w.a.watched(w.ref) {
		t.Error("the watch goes on after the wait")
	}
}

func TestAConflictAfterTheFixesAreUsedUpListsThem(t *testing.T) {
	w := newCI(t, "agent:\n  max_fix_attempts: 1\n", nil)
	w.f.SetConflict(0, true)
	w.step(2, commit("merge.txt", "merged")+done("Merged main once."))
	w.poll()
	w.settle()
	w.syncHead()
	w.poll()
	w.settle()
	p := w.lastPost(w.ref)
	if w.starts() != 2 || p.Marker.Reason != marker.Conflict || !strings.Contains(p.Body, "Merged main once.") || !strings.Contains(p.Body, "1 automatic fix run(s)") {
		t.Fatalf("starts=%d marker=%+v\n%s", w.starts(), p.Marker, p.Body)
	}
}

func TestAConflictAfterACIFixSharesTheCount(t *testing.T) {
	w := newCI(t, "", func(c *config.Config) { c.Comments.Language = "ja" })
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("test", 7))
	w.step(2, commit("fix.txt", "fixed")+done("Fixed."))
	w.poll()
	w.settle()
	w.passCI()
	w.f.SetConflict(0, true)
	w.step(3, commit("merge.txt", "merged")+done("Merged main."))
	w.later(w.cfg.Agent.ReconcileInterval)
	w.poll()
	w.settle()
	claims := w.claims(w.ref)
	if last := claims[len(claims)-1].Body; w.starts() != 3 || !strings.Contains(last, "（2/2）") {
		t.Fatalf("starts=%d, the last claim:\n%s", w.starts(), last)
	}
}

func TestTheConflictWaitIsNotified(t *testing.T) {
	w := newCI(t, "agent:\n  max_fix_attempts: 0\n", nil)
	n := &notes{}
	w.agentWith(func(a *Agent) { a.Now = w.now; a.Notifier = n })
	w.f.SetConflict(0, true)
	w.poll()
	got := n.sent()
	if len(got) != 1 || !strings.Contains(got[0], "⏸ Waiting (conflict) gh:o/r#12") {
		t.Fatalf("notifications = %q", got)
	}
}

// fixLog is newCI with the agent's log kept.
func fixLog(t *testing.T, edit func(*config.Config)) (*ciWorld, *lockedLog) {
	w := newCI(t, "", edit)
	log := &lockedLog{}
	w.agentWith(func(a *Agent) { a.Now = w.now; a.Log = log })
	return w, log
}

// block13 starts o/r#13 as claude start n, blocked until release(n), so that it holds the only slot.
func (w *ciWorld) block13(n int) forge.IssueRef {
	w.t.Helper()
	other := w.f.AddIssue("o", "r", 13, "Other", "Do something else.", w.master)
	w.step(n, block(n, done("Other done.")))
	w.f.AddLabel(other, "agent:ready", w.master)
	w.poll()
	w.waitStarted(n)
	return other
}

// isConflictFix reports whether claude start n was a fix run of a conflict.
func (w *ciWorld) isConflictFix(n int) bool {
	return strings.Contains(w.file(fmt.Sprintf("system-%d.md", n)), "The change request conflicts with its base branch")
}

// isCIFix reports whether claude start n was a fix run of a failed CI.
func (w *ciWorld) isCIFix(n int) bool {
	return strings.Contains(w.file(fmt.Sprintf("system-%d.md", n)), "The CI of the change request failed")
}

func TestAConflictFixStartsWhenASlotFreesBeforeTheReconcile(t *testing.T) {
	w, log := fixLog(t, nil)
	w.passCI()
	w.block13(2)
	w.later(w.cfg.Agent.ReconcileInterval)
	w.f.SetConflict(0, true)
	w.poll()
	if w.starts() != 2 || w.labels(w.ref) != "agent:done" {
		t.Fatalf("starts=%d labels=%q: the fix run must wait for the slot", w.starts(), w.labels(w.ref))
	}
	w.step(3, commit("merge.txt", "merged")+done("Merged main."))
	w.release(2)
	w.settle()
	if w.starts() != 3 || !w.isConflictFix(3) {
		t.Fatalf("starts=%d: want the conflict's fix run as soon as the slot freed, before reconcile_interval", w.starts())
	}
	if n := strings.Count(log.String(), "gh:o/r#12: the fix run (conflict) waits for a slot"); n != 1 {
		t.Errorf("wait log lines = %d, want 1\n%s", n, log.String())
	}
}

func TestAConflictIsFixedBeforeAFailedCIOfAnotherIssue(t *testing.T) {
	w := newCI(t, "", nil)
	second := w.f.AddIssue("o", "r", 14, "Second", "Do a second thing.", w.master)
	w.step(2, commit("second.txt", "second")+done("Did it."))
	w.f.AddLabel(second, "agent:ready", w.master)
	w.poll()
	w.settle()
	p := w.lastPost(second)
	w.f.SetPRHead(p.Marker.CR-1, p.Marker.SHA)
	w.block13(3)
	w.f.SetChecks("o", "r", p.Marker.SHA, failedCheck("test", 7))
	w.poll() // o/r#14's CI fix waits first
	w.later(time.Minute)
	w.f.SetConflict(0, true)
	w.poll() // then o/r#12's conflict fix
	if w.starts() != 3 {
		t.Fatalf("starts=%d: both fix runs must wait for the slot", w.starts())
	}
	w.step(4, block(4, commit("merge.txt", "merged")+done("Merged main.")))
	w.step(5, commit("fix.txt", "fixed")+done("Fixed."))
	w.release(3)
	w.waitStarted(4)
	if !w.isConflictFix(4) || w.labels(w.ref) != "agent:running" || w.labels(second) != "agent:done" {
		t.Fatalf("start 4: conflict fix=%v labels #12=%q #14=%q; want the conflict first", w.isConflictFix(4), w.labels(w.ref), w.labels(second))
	}
	w.release(4)
	w.settle()
	if w.starts() != 5 || !w.isCIFix(5) {
		t.Fatalf("starts=%d: want the CI fix of o/r#14 after the conflict", w.starts())
	}
}

func TestAWaitingConflictFixDoesNotStartOnceTheConflictIsGone(t *testing.T) {
	w, log := fixLog(t, nil)
	w.passCI()
	w.block13(2)
	w.later(w.cfg.Agent.ReconcileInterval)
	w.f.SetConflict(0, true)
	w.poll()
	w.later(w.cfg.Agent.ReconcileInterval)
	w.poll()
	w.f.SetConflict(0, false)
	w.release(2)
	w.settle()
	if w.starts() != 2 || w.labels(w.ref) != "agent:done" || !w.a.watched(w.ref) {
		t.Fatalf("starts=%d labels=%q watched=%v: no fix run once the conflict is gone", w.starts(), w.labels(w.ref), w.a.watched(w.ref))
	}
	if n := strings.Count(log.String(), "gh:o/r#12: the fix run (conflict) waits for a slot"); n != 1 {
		t.Errorf("wait log lines = %d, want 1\n%s", n, log.String())
	}
}

func TestAWaitingConflictFixDoesNotStartOnceTheHeadMoved(t *testing.T) {
	w := newCI(t, "", nil)
	w.passCI()
	w.block13(2)
	w.later(w.cfg.Agent.ReconcileInterval)
	w.f.SetConflict(0, true)
	w.poll()
	w.f.SetPRHead(0, "0000000000000000000000000000000000000000")
	w.release(2)
	w.settle()
	if w.starts() != 2 || w.labels(w.ref) != "agent:done" || w.a.watched(w.ref) {
		t.Fatalf("starts=%d labels=%q watched=%v: a person's push ends the watch", w.starts(), w.labels(w.ref), w.a.watched(w.ref))
	}
}
