// Package usage holds what a run reports about the limits of the AI service it used, in a form that does not depend
// on the service or the CLI. An adapter (such as package claudecode) fills it from its own output; the agent decides
// from it alone, so that another runner only needs its own adapter.
package usage

import "time"

// The kinds of limit a run can hit.
const (
	// KindUsage is the usage limit of a subscription or an account, which resets at a known time.
	KindUsage = "usage"
	// KindRate is any other refusal for too many requests (HTTP 429), usually without a reset time.
	KindRate = "rate"
)

// LimitHit says that a run stopped because the service refused it on a limit.
type LimitHit struct {
	Kind string
	// ResetsAt is when the service said the limit resets; nil when it did not say.
	ResetsAt *time.Time
	// Message is what the service or the CLI said, for the log; it is not shown on the forge.
	Message string
}
