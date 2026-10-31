package agent

import (
	"context"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

// runConnected starts the agent with a relay, connects it, and returns the forge counter and a stop function.
func runConnected(t *testing.T, x *h, hooks []config.Hook) (*counting, func()) {
	t.Helper()
	x.cfg.Agent.PollInterval = 30 * time.Millisecond
	x.cfg.Agent.ReconcileInterval = time.Hour
	x.cfg.Relay.Hooks = hooks
	c := &counting{Forge: x.f}
	f := newFeed()
	x.a = &Agent{Config: x.cfg, Forge: c, State: x.dir, Audit: x.audit, Relay: f, Debounce: 10 * time.Millisecond}
	j, err := trustFor(x)
	if err != nil {
		t.Fatal(err)
	}
	x.a.Judge = j
	ctx, cancel := context.WithCancel(x.ctx)
	done := make(chan error, 1)
	go func() { done <- x.a.Run(ctx) }()
	x.waitFor("the first poll", func() bool { return c.calls.Load() >= 1 })
	f.set(true)
	x.waitFor("a reconcile on connecting", func() bool { return c.calls.Load() >= 2 })
	return c, func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		x.a = nil
	}
}

// A project the relay has no webhook for sends no event: it is polled at poll_interval while connected.
func TestUnhookedProjectIsPolledWhileConnected(t *testing.T) {
	x := newH(t, "", nil)
	c, stop := runConnected(t, x, nil)
	defer stop()
	if got := x.a.interval(); got != 30*time.Millisecond {
		t.Fatalf("interval = %s", got)
	}
	x.waitFor("polls after connecting", func() bool { return c.calls.Load() >= 5 })
}

// A project with a webhook waits for reconcile_interval while connected.
func TestHookedProjectIsOnlyReconciledWhileConnected(t *testing.T) {
	x := newH(t, "", nil)
	c, stop := runConnected(t, x, []config.Hook{{Project: "gh:o/r", Key: "k", SecretRef: "s"}})
	defer stop()
	if got := x.a.interval(); got != time.Hour {
		t.Fatalf("interval = %s", got)
	}
	n := c.calls.Load()
	time.Sleep(200 * time.Millisecond)
	if got := c.calls.Load(); got != n {
		t.Fatalf("polled %d more times while connected", got-n)
	}
}

// With both kinds, the loop runs at poll_interval and takes the hooked projects only when a reconcile is due.
func TestDueProjects(t *testing.T) {
	gl := config.Project{Ref: "gl:nas/sb"}
	gh := config.Project{Ref: "gh:o/r"}
	cfg := &config.Config{Agent: config.Agent{PollInterval: time.Second, ReconcileInterval: time.Hour}}
	cfg.Relay.Hooks = []config.Hook{{Project: "gl:nas/sb"}}
	now := time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)
	a := &Agent{Config: cfg, projects: []config.Project{gl, gh}}
	refs := func(ps []config.Project) (out []string) {
		for _, p := range ps {
			out = append(out, p.Ref)
		}
		return out
	}
	if got := refs(a.due(now)); len(got) != 2 {
		t.Fatalf("disconnected: %v", got)
	}
	a.connected, a.reconciled = true, now
	if got := a.interval(); got != time.Second {
		t.Fatalf("interval with an unhooked project = %s", got)
	}
	if got := refs(a.due(now.Add(time.Minute))); len(got) != 1 || got[0] != "gh:o/r" {
		t.Fatalf("connected, before the reconcile: %v", got)
	}
	if got := refs(a.due(now.Add(time.Hour))); len(got) != 2 {
		t.Fatalf("connected, reconcile due: %v", got)
	}
}
