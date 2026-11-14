package agent

import (
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// readCostH is the harness with o/r#12 running (blocked on start 1) and n issues queued behind it on
// agent.max_parallel 1, with the agent's forge calls counted.
type readCostH struct {
	*h
	cf     *countingForge
	queued []forge.IssueRef
}

func newReadCostH(t *testing.T, n int) *readCostH {
	x := &readCostH{h: newH(t, "", nil)}
	x.cf = newCountingForge(x.f)
	for i := 0; i < n; i++ {
		x.queued = append(x.queued, x.f.AddIssue("o", "r", 100+i, "Queued", "Do it.", x.master))
	}
	x.step(1, block(1, waiting("s", "q")))
	x.agentWith(func(a *Agent) { a.Forge = x.cf })
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	for _, r := range x.queued {
		x.f.AddLabel(r, "agent:ready", x.master)
	}
	x.poll()
	x.waitStarted(1)
	// The queued comments moved the issues' update times, so the next poll reads each of them once (grilling Q4).
	x.advance(time.Second)
	x.poll()
	x.cf.take()
	return x
}

func (x *readCostH) finish() {
	x.release(1)
	x.stop()
}

func TestQueuedIssuesAreNotReadAgainWithoutChange(t *testing.T) {
	x := newReadCostH(t, 80)
	defer x.finish()
	for i := 0; i < 3; i++ {
		x.advance(time.Second)
		x.poll()
	}
	x.a.refresh(x.a.projectList()[0])
	n := x.cf.take()
	if n["Thread"] != 0 || n["LabelActor"] != 0 {
		t.Errorf("3 polls and a refresh with nothing new read the queued issues: %v", n)
	}
	if len(x.queuedPosts(x.queued[0])) != 1 {
		t.Errorf("queued comments = %d, want 1", len(x.queuedPosts(x.queued[0])))
	}
}

func TestQueuedIssueLosesItsLabelAndLeavesTheQueue(t *testing.T) {
	x := newReadCostH(t, 3)
	defer x.finish()
	x.advance(time.Second)
	x.f.RemoveLabel(x.queued[1], "agent:ready")
	x.poll()
	if n := len(x.queuedPosts(x.queued[1])); n != 0 {
		t.Errorf("queued comments after the label went = %d, want 0", n)
	}
	if n := x.cf.take(); n["Thread"] > 1 {
		t.Errorf("the poll read more than the changed issue: %v", n)
	}
	x.advance(time.Second)
	x.f.AddComment(x.queued[2], "Also mind the docs.", x.master)
	x.poll()
	if n := x.cf.take(); n["Thread"] != 1 {
		t.Errorf("a comment on one queued issue: Thread = %d, want 1 (%v)", n["Thread"], n)
	}
	if n := len(x.queuedPosts(x.queued[2])); n != 1 {
		t.Errorf("queued comments of the commented issue = %d, want 1", n)
	}
}
