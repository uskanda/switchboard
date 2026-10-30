package agent

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/relay"
)

// Subscriber is the relay's event stream (relayclient.Client). Run follows it until ctx ends, passing each event to
// on and each connection change to state.
type Subscriber interface {
	Run(ctx context.Context, on func(relay.Event), state func(connected bool))
}

// interval is how long the loop waits between polls: reconcile_interval while the relay is up, else poll_interval.
func (a *Agent) interval() time.Duration {
	a.rmu.Lock()
	defer a.rmu.Unlock()
	if a.connected {
		return a.Config.Agent.ReconcileInterval
	}
	return a.Config.Agent.PollInterval
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
		a.logf("relay connected; reconciling every %s", a.Config.Agent.ReconcileInterval)
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
func (a *Agent) onEvent(e relay.Event) {
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
func (a *Agent) projectOf(e relay.Event) (config.Project, bool) {
	for _, p := range a.projects {
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
	if err := errors.Join(a.pollProject(ctx, p)...); err != nil && ctx.Err() == nil {
		a.logf("relay refresh: %v", err)
	}
	a.drain(ctx)
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
