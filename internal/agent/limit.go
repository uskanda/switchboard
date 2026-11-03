package agent

import (
	"os"
	"path/filepath"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/usage"
)

// pauseFile keeps the pause of this host, so that an agent started again keeps it (ADR-047).
const pauseFile = "pause.json"

// costFile keeps what the runs of this host cost today and this month, local time, when claude uses an API key.
const costFile = "cost.json"

// pause is a time until which this host starts no new run: it hit a usage limit (marker.QueuedLimit) or its API
// quota (marker.QueuedQuota). Runs already going on are left to finish.
type pause struct {
	Reason string    `json:"reason"`
	Until  time.Time `json:"until"`
}

// costs are the totals the quota is checked against.
type costs struct {
	Day      string  `json:"day"` // 2006-01-02, local time
	DayUSD   float64 `json:"day_usd"`
	Month    string  `json:"month"` // 2006-01
	MonthUSD float64 `json:"month_usd"`
}

// loadLimits reads the pause and the costs a previous process left.
func (a *Agent) loadLimits() {
	var p pause
	if a.State.ReadJSON(pauseFile, &p) == nil && p.Reason != "" {
		a.mu.Lock()
		a.paused = &p
		a.armPause(p.Until)
		a.mu.Unlock()
		a.logf("paused (%s) until %s, as the previous process left it", p.Reason, p.Until.Local().Format(time.RFC3339))
	}
	var c costs
	if a.State.ReadJSON(costFile, &c) == nil {
		a.mu.Lock()
		a.cost = c
		a.mu.Unlock()
	}
}

// pausedLocked returns why and until when this host starts no new run now, or "" when it may. A pause whose time
// passed ends here; an API quota reached starts one. Call it with mu held.
func (a *Agent) pausedLocked() (string, time.Time) {
	now := a.now()
	if a.paused != nil {
		if now.Before(a.paused.Until) {
			return a.paused.Reason, a.paused.Until
		}
		a.logf("the pause (%s) ended", a.paused.Reason)
		a.Audit.Log("host.resumed", map[string]any{"reason": a.paused.Reason})
		a.paused = nil
		if err := os.Remove(filepath.Join(a.State.Path, pauseFile)); err != nil && !os.IsNotExist(err) {
			a.logf("remove %s: %v", pauseFile, err)
		}
	}
	if until, ok := a.overQuota(now); ok {
		a.pauseLocked(marker.QueuedQuota, until)
		return marker.QueuedQuota, until
	}
	return "", time.Time{}
}

// pauseLocked pauses the host until until, unless it is already paused until later. Call it with mu held.
func (a *Agent) pauseLocked(reason string, until time.Time) {
	if a.paused != nil && !until.After(a.paused.Until) {
		return
	}
	a.paused = &pause{Reason: reason, Until: until}
	if err := a.State.WriteJSON(pauseFile, a.paused); err != nil {
		a.logf("save the pause: %v", err)
	}
	a.armPause(until)
	a.logf("paused (%s): no new run starts until %s", reason, until.Local().Format(time.RFC3339))
	a.Audit.Log("host.paused", map[string]any{"reason": reason, "until": until.UTC().Format(time.RFC3339)})
}

// armPause takes the queued issues up again once until passes. Each poll does too; the timer only spares the wait
// for the next one. Call it with mu held.
func (a *Agent) armPause(until time.Time) {
	if a.pauseTimer != nil {
		a.pauseTimer.Stop()
	}
	d := until.Sub(a.now()) + time.Second
	if d <= 0 {
		return
	}
	a.pauseTimer = time.AfterFunc(d, func() {
		if a.ctx == nil || a.ctx.Err() != nil {
			return
		}
		a.pollMu.Lock()
		defer a.pollMu.Unlock()
		a.drain(a.ctx)
		a.retune()
	})
}

// stopPauseTimer stops the timer of the pause, when the agent closes.
func (a *Agent) stopPauseTimer() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pauseTimer != nil {
		a.pauseTimer.Stop()
		a.pauseTimer = nil
	}
}

// pauseForLimit pauses the host on the limit a run hit and returns until when: the reset time the service gave, or
// agent.limit_wait from now when it gave none or one already past.
func (a *Agent) pauseForLimit(hit *usage.LimitHit) time.Time {
	now := a.now()
	until := now.Add(a.Config.Agent.LimitWait)
	if hit.ResetsAt != nil && hit.ResetsAt.After(now) {
		until = *hit.ResetsAt
	}
	a.mu.Lock()
	a.pauseLocked(marker.QueuedLimit, until)
	if a.paused != nil && a.paused.Until.After(until) {
		until = a.paused.Until
	}
	a.mu.Unlock()
	return until
}

// quotaOn reports whether the runs of this host count toward the API quota.
func (a *Agent) quotaOn() bool {
	return a.Config.Claude.Auth == "api_key" && a.Config.Quota.Set()
}

// addCost adds what res cost to today's and this month's totals, when claude uses an API key.
func (a *Agent) addCost(res *claudecode.Result) {
	if res == nil || a.Config.Claude.Auth != "api_key" {
		return
	}
	now := a.now().Local()
	day, month := now.Format("2006-01-02"), now.Format("2006-01")
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cost.Day != day {
		a.cost.Day, a.cost.DayUSD = day, 0
	}
	if a.cost.Month != month {
		a.cost.Month, a.cost.MonthUSD = month, 0
	}
	a.cost.DayUSD += res.CostUSD
	a.cost.MonthUSD += res.CostUSD
	if err := a.State.WriteJSON(costFile, a.cost); err != nil {
		a.logf("save the costs: %v", err)
	}
}

// overQuota reports whether today's or this month's costs reached their quota at now, and until when no run starts:
// the next local midnight, or the first of the next month. Call it with mu held.
func (a *Agent) overQuota(now time.Time) (time.Time, bool) {
	if !a.quotaOn() {
		return time.Time{}, false
	}
	q, local := a.Config.Quota, now.Local()
	if q.MonthlyUSD != nil && a.cost.Month == local.Format("2006-01") && a.cost.MonthUSD >= *q.MonthlyUSD {
		return time.Date(local.Year(), local.Month()+1, 1, 0, 0, 0, 0, time.Local), true
	}
	if q.DailyUSD != nil && a.cost.Day == local.Format("2006-01-02") && a.cost.DayUSD >= *q.DailyUSD {
		return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, time.Local), true
	}
	return time.Time{}, false
}

// isPause reports whether a queued reason is a pause of the host rather than a full slot.
func isPause(reason string) bool { return reason == marker.QueuedLimit || reason == marker.QueuedQuota }

// limitResume returns the comment that marks the issue to resume after a usage limit: the agent's queued comment of
// a run that hit the limit, when it is the agent's latest post other than a progress comment or another queued
// comment. It returns 0 when there is none.
func (pl *plan) limitResume() int64 {
	for i := len(pl.comments) - 1; i >= 0; i-- {
		m := pl.agentMarker(i)
		switch {
		case m == nil, m.State == marker.Progress:
		case m.State == marker.Queued && m.Run == "":
		case m.State == marker.Queued && isPause(m.Reason):
			return pl.comments[i].ID
		default:
			return 0
		}
	}
	return 0
}
