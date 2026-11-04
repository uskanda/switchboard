package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/event"
	"github.com/uskanda/switchboard/internal/forge"
)

// Subscriber is the relay's event stream (relayclient.Client). Run follows it until ctx ends, passing each event to
// on and each connection change to state.
type Subscriber interface {
	Run(ctx context.Context, on func(event.Event), state func(connected bool))
}

// interval is how long the loop waits between polls: reconcile_interval while the relay is up and has a webhook for
// every project, else poll_interval. A project without a webhook in relay.hooks gets no event, so it is polled as
// if there were no relay. Something to try again (a write-back, an evaluation that failed) or a CI to watch also
// brings the loop back to poll_interval until it is done.
func (a *Agent) interval() time.Duration {
	retrying := len(a.retried()) > 0 || a.pendingWriteBacks() || a.watching()
	a.rmu.Lock()
	defer a.rmu.Unlock()
	if a.connected && len(a.uncovered()) == 0 && !retrying {
		return a.Config.Agent.ReconcileInterval
	}
	return a.Config.Agent.PollInterval
}

// uncovered lists the projects the relay sends no event of.
func (a *Agent) uncovered() []config.Project {
	var out []config.Project
	for _, p := range a.projectList() {
		if !a.Config.Relay.Covers(p.Ref) {
			out = append(out, p)
		}
	}
	return out
}

// due lists the projects a tick at now fetches: every one while the relay is down; while it is up, those without a
// webhook or with an issue to evaluate again, and the others too once reconcile_interval has passed since the last
// reconcile.
func (a *Agent) due(now time.Time) []config.Project {
	again := a.retried()
	a.rmu.Lock()
	defer a.rmu.Unlock()
	if !a.connected || now.Sub(a.reconciled) >= a.Config.Agent.ReconcileInterval {
		return a.projectList()
	}
	var out []config.Project
	for _, p := range a.projectList() {
		if !a.Config.Relay.Covers(p.Ref) || again[p.Ref] {
			out = append(out, p)
		}
	}
	return out
}

// retried is the set of projects, by ref, with an issue to evaluate again that does not only wait for a slot.
func (a *Agent) retried() map[string]bool {
	a.mu.Lock()
	var refs []forge.IssueRef
	for ref, u := range a.unsettled {
		if !u.queued {
			refs = append(refs, ref)
		}
	}
	a.mu.Unlock()
	out := map[string]bool{}
	for _, ref := range refs {
		for _, p := range a.projectList() {
			if inProject(ref, p) {
				out[p.Ref] = true
			}
		}
	}
	return out
}

// pendingWriteBacks reports whether a write-back waits to be tried again.
func (a *Agent) pendingWriteBacks() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.writebacks) > 0
}

// retune tells the loop to check its interval again, without polling: something to try again appeared outside a
// poll of the loop (a relay refresh, the end of a run).
func (a *Agent) retune() {
	select {
	case a.tune <- struct{}{}:
	default:
	}
}

// onRelayState records whether the relay is up and wakes the loop, which polls at once: on connecting, to take in
// what the relay missed while away; on losing it, to go back to polling.
func (a *Agent) onRelayState(up bool) {
	a.rmu.Lock()
	changed := a.connected != up
	a.connected = up
	a.rmu.Unlock()
	if !changed {
		return
	}
	if up {
		a.rmu.Lock()
		n := len(a.uncovered())
		a.rmu.Unlock()
		a.logf("relay connected; reconciling every %s, and polling %d project(s) without a webhook every %s",
			a.Config.Agent.ReconcileInterval, n, a.Config.Agent.PollInterval)
	} else {
		a.logf("relay lost; polling every %s until it is back", a.Config.Agent.PollInterval)
	}
	a.Audit.Log("relay.state", map[string]any{"connected": up})
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// onEvent schedules a refresh of the event's project, if it is one of this host's. The event says only that
// something changed: what changed is read from the forge, as on a poll.
func (a *Agent) onEvent(e event.Event) {
	p, ok := a.projectOf(e)
	if !ok {
		return
	}
	d := a.Debounce
	if d == 0 {
		d = 3 * time.Second
	}
	a.rmu.Lock()
	defer a.rmu.Unlock()
	if a.timers == nil {
		a.timers = map[string]*time.Timer{}
	}
	if t, ok := a.timers[p.Ref]; ok {
		t.Reset(d)
		return
	}
	a.timers[p.Ref] = time.AfterFunc(d, func() {
		a.rmu.Lock()
		delete(a.timers, p.Ref)
		a.rmu.Unlock()
		a.refresh(p)
	})
}

// projectOf finds this host's project an event is about. Project paths compare without case, as the forges do.
func (a *Agent) projectOf(e event.Event) (config.Project, bool) {
	for _, p := range a.projectList() {
		if p.Forge == e.Forge && strings.EqualFold(p.FullName(), e.Project) {
			return p, true
		}
	}
	return config.Project{}, false
}

// refresh polls one project, in turn with the loop's polls, and starts what became ready.
func (a *Agent) refresh(p config.Project) {
	ctx := a.ctx
	if ctx == nil || ctx.Err() != nil {
		return
	}
	a.pollMu.Lock()
	defer a.pollMu.Unlock()
	a.startFixes(ctx) // before a newly ready issue takes a free slot
	if err := errors.Join(a.pollProject(ctx, p)...); err != nil && ctx.Err() == nil {
		a.logf("relay refresh: %v", err)
	}
	a.drain(ctx)
	a.retune()
}

// stopRefreshes cancels the pending refreshes and, by taking the poll lock once, waits for one in progress.
func (a *Agent) stopRefreshes() {
	a.rmu.Lock()
	for ref, t := range a.timers {
		t.Stop()
		delete(a.timers, ref)
	}
	a.rmu.Unlock()
	a.pollMu.Lock()
	a.pollMu.Unlock()
}
