package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// orderH is the harness with o/r#12 running (blocked on start 1) on agent.max_parallel 1, and issues queued
// behind it one poll after another, so that each enters the queue later than the one before.
type orderH struct {
	*h
	queued []forge.IssueRef
	issues map[int]forge.IssueRef
	n      int // the claude start the next queued issue gets
}

// queueItem is an issue to queue: its number and the start label put on it.
type queueItem struct {
	num   int
	label string
}

func newOrderH(t *testing.T, preferPlan bool, items ...queueItem) *orderH {
	x := &orderH{h: newH(t, "", func(c *config.Config) { c.Agent.PreferPlan = preferPlan }), issues: map[int]forge.IssueRef{}, n: 1}
	x.step(1, block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	for _, it := range items {
		ref := x.f.AddIssue("o", "r", it.num, fmt.Sprintf("Issue %d", it.num), "Do it.", x.master)
		x.issues[it.num] = ref
		x.queued = append(x.queued, ref)
		x.f.AddLabel(ref, it.label, x.master)
		x.poll()
	}
	if x.starts() != 1 {
		x.t.Fatalf("starts = %d, want the queued issues still waiting", x.starts())
	}
	return x
}

// next ends the run of the current start and returns the queued issue that the freed slot went to. plan says the
// run is expected to be a plan run, so that it answers with questions.
func (x *orderH) next(plan bool) forge.IssueRef {
	x.t.Helper()
	out := waiting("s", "q")
	if plan {
		out = questions([3]string{"Default", "Which default?", "A."})
	}
	x.step(x.n+1, block(x.n+1, out))
	x.release(x.n)
	x.n++
	x.waitStarted(x.n)
	for _, ref := range x.queued {
		if strings.Contains(x.labels(ref), "agent:running") {
			return ref
		}
	}
	x.t.Fatalf("start %d: no queued issue is running", x.n)
	return forge.IssueRef{}
}

// expect checks that the slots go to the issues nums in this order; plans names the plan runs among them.
func (x *orderH) expect(plans map[int]bool, nums ...int) {
	x.t.Helper()
	for i, num := range nums {
		if got := x.next(plans[num]); got != x.issues[num] {
			x.t.Fatalf("slot %d went to %s, want %s (order %v)", i+1, got, x.issues[num], nums)
		}
	}
	x.release(x.n)
	x.settle()
}

func TestQueueOrderBreaksTiesByRefAndKeepPlaceRestoresTheTime(t *testing.T) {
	ref := func(n int) forge.IssueRef {
		return forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: n}
	}
	t0 := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	a := &Agent{pending: map[forge.IssueRef]queuedRun{
		ref(2): {since: t0},
		ref(1): {since: t0},
		ref(3): {since: t0.Add(-time.Minute)},
	}}
	if got := fmt.Sprint(a.queueOrder()); got != fmt.Sprint([]forge.IssueRef{ref(3), ref(1), ref(2)}) {
		t.Errorf("order = %s, want #3 (earliest), then #1 and #2 by ref", got)
	}
	// As when drain took #3 out and launch queued it again with a new time.
	a.pending[ref(3)] = queuedRun{since: t0.Add(time.Minute)}
	a.keepPlace(ref(3), t0.Add(-time.Minute))
	if got := a.queueOrder()[0]; got != ref(3) {
		t.Errorf("first = %s, want #3 with its time in the queue restored", got)
	}
	a.keepPlace(ref(4), t0)
	if _, ok := a.pending[ref(4)]; ok {
		t.Error("keepPlace queued an issue that was not queued")
	}
}

func TestQueueStartsInTheOrderEntered(t *testing.T) {
	x := newOrderH(t, false, queueItem{10, "agent:ready"}, queueItem{9, "agent:plan"}, queueItem{11, "agent:ready"})
	x.expect(map[int]bool{9: true}, 10, 9, 11)
}

func TestPreferPlanStartsThePlanRunFirst(t *testing.T) {
	x := newOrderH(t, true, queueItem{10, "agent:ready"}, queueItem{9, "agent:plan"}, queueItem{11, "agent:ready"})
	x.expect(map[int]bool{9: true}, 9, 10, 11)
}

func TestReloadTurnsPreferPlanOn(t *testing.T) {
	x := newOrderH(t, false, queueItem{10, "agent:ready"}, queueItem{9, "agent:plan"}, queueItem{11, "agent:ready"})
	x.a.Reload(&config.Change{Projects: x.cfg.Projects, MaxParallel: 1, PreferPlan: true})
	x.poll()
	x.expect(map[int]bool{9: true}, 9, 10, 11)
}

// queueIssue puts o/r#num with label in the queue behind the run that holds the slot.
func (w *ciWorld) queueIssue(num int, label string) forge.IssueRef {
	w.t.Helper()
	ref := w.f.AddIssue("o", "r", num, fmt.Sprintf("Issue %d", num), "Do it.", w.master)
	w.f.AddLabel(ref, label, w.master)
	w.poll()
	if len(w.queuedPosts(ref)) != 1 {
		w.t.Fatalf("%s is not queued", ref)
	}
	return ref
}

// noQueuedPost fails when the done issue ref got a queued comment.
func (w *ciWorld) noQueuedPost(ref forge.IssueRef) {
	w.t.Helper()
	if n := len(w.queuedPosts(ref)); n != 0 {
		w.t.Fatalf("%s got %d queued comment(s); a fix run never posts one", ref, n)
	}
}

func TestAConflictFixStartsBeforeTheQueue(t *testing.T) {
	w := newCI(t, "", nil)
	w.passCI()
	w.block13(2)
	q := w.queueIssue(10, "agent:ready")
	w.later(w.cfg.Agent.ReconcileInterval)
	w.f.SetConflict(0, true)
	w.poll()
	w.step(3, block(3, commit("merge.txt", "merged")+done("Merged main.")))
	w.release(2)
	w.waitStarted(3)
	if !w.isConflictFix(3) || w.labels(w.ref) != "agent:running" || w.labels(q) != "agent:ready" {
		t.Fatalf("start 3: conflict fix=%v labels #12=%q #10=%q; want the fix run before the queue", w.isConflictFix(3), w.labels(w.ref), w.labels(q))
	}
	w.noQueuedPost(w.ref)
	w.release(3)
	w.settle()
	if w.starts() != 4 || w.labels(q) != "agent:waiting" {
		t.Fatalf("starts=%d labels #10=%q: the queued issue runs after the fix", w.starts(), w.labels(q))
	}
}

func TestACIFixStartsBeforeAPreferredPlanRun(t *testing.T) {
	w := newCI(t, "", func(c *config.Config) { c.Agent.PreferPlan = true })
	w.block13(2)
	q := w.queueIssue(9, "agent:plan")
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("test", 7))
	w.poll()
	w.step(3, block(3, commit("fix.txt", "fixed")+done("Fixed.")))
	w.step(4, questions([3]string{"Default", "Which default?", "A."}))
	w.release(2)
	w.waitStarted(3)
	if !w.isCIFix(3) || w.labels(w.ref) != "agent:running" || w.labels(q) != "agent:plan" {
		t.Fatalf("start 3: CI fix=%v labels #12=%q #9=%q; want the fix run before the plan run", w.isCIFix(3), w.labels(w.ref), w.labels(q))
	}
	w.noQueuedPost(w.ref)
	w.release(3)
	w.settle()
	if w.starts() != 4 {
		t.Fatalf("starts=%d: the plan run follows the fix", w.starts())
	}
}

func TestAPollDoesNotGiveAWaitingFixsSlotToANewIssue(t *testing.T) {
	w := newCI(t, "", nil)
	w.passCI()
	w.block13(2)
	w.later(w.cfg.Agent.ReconcileInterval)
	w.f.SetConflict(0, true)
	w.poll()
	// A second slot opens while o/r#13 runs, and a new issue becomes ready in the same poll.
	fresh := w.f.AddIssue("o", "r", 10, "Issue 10", "Do it.", w.master)
	w.f.AddLabel(fresh, "agent:ready", w.master)
	w.step(3, block(3, commit("merge.txt", "merged")+done("Merged main.")))
	w.a.Reload(&config.Change{Projects: w.cfg.Projects, MaxParallel: 2})
	w.poll()
	w.waitStarted(3)
	if !w.isConflictFix(3) || w.labels(fresh) != "agent:ready" || len(w.queuedPosts(fresh)) != 1 {
		t.Fatalf("start 3: conflict fix=%v labels #10=%q; want the slot for the waiting fix and #10 queued", w.isConflictFix(3), w.labels(fresh))
	}
	w.release(2)
	w.release(3)
	w.settle()
}

func TestAFixAtItsProjectLimitDoesNotHoldUpAnotherProject(t *testing.T) {
	x := newReloadH(t)
	x.cfg.Agent.MaxParallel = 2
	x.cfg.Projects[0].MaxParallel = limit(1)
	w := &ciWorld{h: x.h}
	x.agentWith(func(a *Agent) {
		a.Now = w.now
		a.Log = &x.log
		a.CloneURL = func(p config.Project) string {
			if p.Ref == "gh:o/notes" {
				return x.origin
			}
			return x.h.origin
		}
	})
	w.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.settle()
	w.syncHead()
	w.passCI()
	w.block13(2) // o/r at its limit
	w.step(3, block(3, waiting("s", "q")))
	x.reload(x.notes)
	w.poll()
	w.waitStarted(3) // o/notes#1: the host is full
	notes2 := w.f.AddIssue("o", "notes", 2, "More notes", "More.", w.master)
	w.f.AddLabel(notes2, "agent:ready", w.master)
	w.later(w.cfg.Agent.ReconcileInterval)
	w.f.SetConflict(0, true)
	w.poll()
	w.step(4, block(4, waiting("s", "q")))
	w.step(5, commit("merge.txt", "merged")+done("Merged main."))
	w.release(3)
	w.waitStarted(4)
	if w.labels(notes2) == "agent:ready" || w.labels(w.ref) != "agent:done" {
		t.Fatalf("labels notes#2=%q r#12=%q; want notes#2 taken up and the fix of o/r#12 still waiting", w.labels(notes2), w.labels(w.ref))
	}
	w.release(2)
	w.waitFor("the fix run of o/r#12", func() bool { return w.starts() == 5 })
	w.release(4)
	w.settle()
	if !w.isConflictFix(5) {
		t.Fatal("start 5 is not the conflict's fix run")
	}
}

func TestQueuedIssueKeepsItsPlaceWhenItsKindChanges(t *testing.T) {
	x := newOrderH(t, true, queueItem{10, "agent:ready"}, queueItem{9, "agent:plan"}, queueItem{11, "agent:ready"})
	// o/r#9 becomes an implementation while it waits: it no longer goes first, and keeps its time in the queue.
	x.f.RemoveLabel(x.issues[9], "agent:plan")
	x.f.AddLabel(x.issues[9], "agent:ready", x.master)
	x.poll()
	x.expect(nil, 10, 9, 11)
}
