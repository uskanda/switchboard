package agent

import (
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

// The done post of a fix run names what it fixed, so that the inbox service can tell it from the done post of a run a
// person asked for (issue #119); the first run's done post names nothing.
func TestTheDonePostOfAFixRunNamesTheFix(t *testing.T) {
	w := newCI(t, "", nil)
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
	var fixes []string
	for _, p := range w.posts(w.ref) {
		if p.Marker != nil && p.Marker.State == marker.Done {
			fixes = append(fixes, p.Marker.Fix)
		}
	}
	if strings.Join(fixes, ",") != ",ci,conflict" {
		for _, p := range w.posts(w.ref) {
			t.Log(p.Body)
		}
		t.Fatalf("the fix of each done post = %q, want the first run's empty, then ci, then conflict", fixes)
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
