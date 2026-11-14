package agent

import (
	"errors"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// limitKey names the forge client that reads ref: the project's own route, or the agent's default client. A rate
// limit is kept per client, since each route may carry its own token.
func (a *Agent) limitKey(ref forge.IssueRef) string {
	key := ref.Kind() + ":" + ref.Project()
	a.pmu.RLock()
	_, ok := a.Routes[key]
	a.pmu.RUnlock()
	if ok {
		return key
	}
	return ""
}

// rateLimitName is how the log names the client of key, for an issue of ref.
func rateLimitName(key string, ref forge.IssueRef) string {
	if key != "" {
		return key
	}
	return "the " + ref.Kind() + " forge"
}

// noteLimit records the rate limit err reports for the client that reads ref, and reports whether err is one. The
// first refusal of a limit is logged; the reads of that client stop until its reset (ADR-072).
func (a *Agent) noteLimit(ref forge.IssueRef, err error) bool {
	var rl *forge.RateLimitError
	if !errors.As(err, &rl) {
		return false
	}
	key := a.limitKey(ref)
	a.mu.Lock()
	_, known := a.limits[key]
	if !known || rl.Reset.After(a.limits[key]) {
		a.limits[key] = rl.Reset
	}
	a.mu.Unlock()
	if !known {
		a.logf("%s: the API rate limit is used up; no reads until %s", rateLimitName(key, ref), rl.Reset.Local().Format(time.TimeOnly))
	}
	return true
}

// limited reports whether the client that reads ref is under a rate limit now. The first call after the reset
// forgets the limit and logs that reads go on.
func (a *Agent) limited(ref forge.IssueRef) bool {
	key := a.limitKey(ref)
	now := a.now()
	a.mu.Lock()
	reset, ok := a.limits[key]
	over := ok && !now.Before(reset)
	if over {
		delete(a.limits, key)
	}
	a.mu.Unlock()
	if over {
		a.logf("%s: the API rate limit has reset; reading again", rateLimitName(key, ref))
	}
	return ok && !over
}

// projectRef is a reference into p with no issue number, to find the client that reads p.
func projectRef(p config.Project) forge.IssueRef {
	return forge.IssueRef{Forge: p.Forge, Owner: p.Owner, Repo: p.Repo}
}
