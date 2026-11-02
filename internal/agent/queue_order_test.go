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

func TestQueuedIssueKeepsItsPlaceWhenItsKindChanges(t *testing.T) {
	x := newOrderH(t, true, queueItem{10, "agent:ready"}, queueItem{9, "agent:plan"}, queueItem{11, "agent:ready"})
	// o/r#9 becomes an implementation while it waits: it no longer goes first, and keeps its time in the queue.
	x.f.RemoveLabel(x.issues[9], "agent:plan")
	x.f.AddLabel(x.issues[9], "agent:ready", x.master)
	x.poll()
	x.expect(nil, 10, 9, 11)
}
