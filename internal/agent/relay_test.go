package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/relay"
	"github.com/uskanda/switchboard/internal/trust"
)

// counting counts UpdatedIssues calls and notices two of them running at once.
type counting struct {
	forge.Forge
	calls, inFlight, overlap atomic.Int32
}

func (c *counting) UpdatedIssues(ctx context.Context, owner, repo string, since time.Time) ([]forge.IssueSummary, error) {
	c.calls.Add(1)
	if c.inFlight.Add(1) > 1 {
		c.overlap.Add(1)
	}
	defer c.inFlight.Add(-1)
	time.Sleep(5 * time.Millisecond)
	return c.Forge.UpdatedIssues(ctx, owner, repo, since)
}

// feed is a relay subscription the test drives.
type feed struct {
	mu    sync.Mutex
	on    func(relay.Event)
	state func(bool)
	ready chan struct{}
}

func newFeed() *feed { return &feed{ready: make(chan struct{})} }

func (f *feed) Run(ctx context.Context, on func(relay.Event), state func(bool)) {
	f.mu.Lock()
	f.on, f.state = on, state
	f.mu.Unlock()
	close(f.ready)
	<-ctx.Done()
}

func (f *feed) send(e relay.Event) { <-f.ready; f.mu.Lock(); on := f.on; f.mu.Unlock(); on(e) }
func (f *feed) set(up bool)        { <-f.ready; f.mu.Lock(); st := f.state; f.mu.Unlock(); st(up) }

func relayH(t *testing.T) (*h, *counting) {
	x := newH(t, "", nil)
	c := &counting{Forge: x.f}
	x.agentWith(func(a *Agent) { a.Forge = c; a.Debounce = 40 * time.Millisecond })
	return x, c
}

var ours = relay.Event{Forge: "gh", Project: "o/r", Issue: 12, Kind: "issues"}

func TestRelayEventOfAnotherProjectIsIgnored(t *testing.T) {
	x, c := relayH(t)
	for _, e := range []relay.Event{
		{Forge: "gh", Project: "o/other", Issue: 1, Kind: "issues"},
		{Forge: "gl", Project: "o/r", Issue: 12, Kind: "note"},
	} {
		x.a.onEvent(e)
	}
	time.Sleep(150 * time.Millisecond)
	if n := c.calls.Load(); n != 0 {
		t.Fatalf("the forge was asked %d times", n)
	}
}

func TestRelayEventsOfAProjectAreMergedIntoOneFetch(t *testing.T) {
	x, c := relayH(t)
	x.a.onEvent(ours)
	time.Sleep(10 * time.Millisecond)
	x.a.onEvent(relay.Event{Forge: "gh", Project: "O/R", Issue: 12, Kind: "issue_comment"}) // GitHub names are case-insensitive
	x.waitFor("a fetch", func() bool { return c.calls.Load() >= 1 })
	time.Sleep(150 * time.Millisecond)
	if n := c.calls.Load(); n != 1 {
		t.Fatalf("the project was fetched %d times", n)
	}
}

// An event starts the run without waiting for a poll.
func TestRelayEventStartsTheRun(t *testing.T) {
	x, _ := relayH(t)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.a.onEvent(ours)
	x.waitFor("claude to start", func() bool { return x.starts() == 1 })
	x.settle()
	if got := x.labels(x.ref); got != "agent:waiting" {
		t.Fatalf("labels %s", got)
	}
}

func TestRelayRefreshAndPollDoNotOverlap(t *testing.T) {
	x, c := relayH(t)
	for i := 0; i < 5; i++ {
		x.a.onEvent(ours)
		x.poll()
		time.Sleep(45 * time.Millisecond)
	}
	x.waitFor("the last fetch", func() bool { return c.inFlight.Load() == 0 })
	if n := c.overlap.Load(); n != 0 {
		t.Fatalf("%d fetches overlapped", n)
	}
}

// Connected, the agent reconciles at once and then every reconcile_interval; disconnected, it polls at once and then
// every poll_interval.
func TestRelayStateSwitchesTheInterval(t *testing.T) {
	x := newH(t, "", nil)
	x.cfg.Agent.PollInterval = time.Hour
	x.cfg.Agent.ReconcileInterval = 2 * time.Hour
	x.cfg.Relay.Hooks = []config.Hook{{Project: "gh:o/r", Key: "k", SecretRef: "s"}} // the relay covers the project
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
	x.waitFor("the first poll", func() bool { return c.calls.Load() == 1 })
	if got := x.a.interval(); got != time.Hour {
		t.Fatalf("before connecting: %s", got)
	}
	f.set(true)
	x.waitFor("a reconcile on connecting", func() bool { return c.calls.Load() == 2 })
	if got := x.a.interval(); got != 2*time.Hour {
		t.Fatalf("connected: %s", got)
	}
	f.set(false)
	x.waitFor("a poll on losing the relay", func() bool { return c.calls.Load() == 3 })
	if got := x.a.interval(); got != time.Hour {
		t.Fatalf("disconnected: %s", got)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	x.a = nil
}

func trustFor(x *h) (*trust.Judge, error) {
	return trust.Resolve(x.ctx, x.f, x.dir, x.cfg.TrustedAuthors.GitHub, x.cfg.Forges.GitHub.AgentIdentity, x.audit)
}
