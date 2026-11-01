package agent

import (
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

func limit(n int) *int { return &n }

func TestProjectLimitQueuesTheSecondIssue(t *testing.T) {
	x := newH(t, "", func(c *config.Config) {
		c.Agent.MaxParallel = 3
		c.Projects[0].MaxParallel = limit(1)
	})
	second := x.f.AddIssue("o", "r", 13, "Another", "Do another thing.", x.master)
	x.step(1, block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.AddLabel(second, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.poll()
	time.Sleep(100 * time.Millisecond)
	if x.starts() != 1 || x.labels(second) != "agent:ready" {
		t.Fatalf("the second issue started past the project limit: starts=%d labels=%q", x.starts(), x.labels(second))
	}
	x.release(1)
	x.settle()
	if x.starts() != 2 || x.labels(second) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.labels(second))
	}
}

func TestNoProjectLimitLeavesTheAgentLimit(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Agent.MaxParallel = 2 })
	second := x.f.AddIssue("o", "r", 13, "Another", "Do another thing.", x.master)
	x.step(1, block(1, waiting("s", "q")))
	x.step(2, block(2, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.AddLabel(second, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.waitStarted(2)
	x.release(1)
	x.release(2)
	x.settle()
	if x.labels(x.ref) != "agent:waiting" || x.labels(second) != "agent:waiting" {
		t.Fatalf("labels=%q, %q", x.labels(x.ref), x.labels(second))
	}
}

func TestDrainSkipsAProjectAtItsLimit(t *testing.T) {
	x := newReloadH(t)
	x.cfg.Agent.MaxParallel = 2
	x.notes.MaxParallel = limit(1)
	x.step(1, block(1, waiting("s", "q")))
	x.step(2, block(2, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1) // o/r#12
	x.reload(x.notes)
	x.poll()
	x.waitStarted(2) // o/notes#1
	notes2 := x.f.AddIssue("o", "notes", 2, "More notes", "More.", x.master)
	r13 := x.f.AddIssue("o", "r", 13, "Another", "Do another thing.", x.master)
	x.f.AddLabel(notes2, "agent:ready", x.master)
	x.f.AddLabel(r13, "agent:ready", x.master)
	x.poll()
	x.release(1)
	x.waitFor("o/r#13 to start", func() bool { return x.starts() == 3 })
	time.Sleep(100 * time.Millisecond)
	if x.labels(notes2) != "agent:ready" || x.labels(r13) == "agent:ready" {
		t.Fatalf("labels: notes#2=%q r#13=%q; want notes#2 queued and r#13 taken up", x.labels(notes2), x.labels(r13))
	}
	x.release(2)
	x.settle()
	if x.starts() != 4 || x.labels(notes2) != "agent:waiting" {
		t.Fatalf("starts=%d notes#2=%q", x.starts(), x.labels(notes2))
	}
}

func TestReloadRaisesTheAgentLimit(t *testing.T) {
	x := newH(t, "", nil)
	second := x.f.AddIssue("o", "r", 13, "Another", "Do another thing.", x.master)
	x.step(1, block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.AddLabel(second, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.a.Reload(&config.Change{Projects: x.cfg.Projects, MaxParallel: 2})
	x.poll()
	x.waitFor("the queued issue to start", func() bool { return x.starts() == 2 })
	x.release(1)
	x.settle()
}

func TestReloadLowersTheAgentLimitWithoutStoppingRuns(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Agent.MaxParallel = 2 })
	second := x.f.AddIssue("o", "r", 13, "Another", "Do another thing.", x.master)
	third := x.f.AddIssue("o", "r", 14, "A third", "And another.", x.master)
	x.step(1, block(1, waiting("s", "q")))
	x.step(2, block(2, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.AddLabel(second, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.waitStarted(2)
	x.a.Reload(&config.Change{Projects: x.cfg.Projects, MaxParallel: 1})
	x.f.AddLabel(third, "agent:ready", x.master)
	x.poll()
	x.release(1)
	x.waitFor("a run to end", func() bool {
		return x.labels(x.ref) == "agent:waiting" || x.labels(second) == "agent:waiting"
	})
	time.Sleep(100 * time.Millisecond)
	if x.starts() != 2 || x.labels(third) != "agent:ready" {
		t.Fatalf("a run started above the lowered limit: starts=%d labels=%q", x.starts(), x.labels(third))
	}
	x.release(2)
	x.settle()
	if x.starts() != 3 || x.labels(x.ref) != "agent:waiting" || x.labels(second) != "agent:waiting" || x.labels(third) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q, %q, %q", x.starts(), x.labels(x.ref), x.labels(second), x.labels(third))
	}
}
