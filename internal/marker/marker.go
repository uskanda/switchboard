// Package marker writes and reads the hidden line that ends every comment the agent posts:
//
//	<!-- switchboard v=1 host=<host> session=<id> run=<id> state=<state> reason=<reason> ack=<comment-id> at=<RFC3339> -->
//
// A claim made from a start label also carries mode=<plan|direct>, and a post of plan questions q=<last number>.
// A done post that names a change request carries sha=<the commit pushed> and cr=<its number>; the done post of a fix
// run also carries fix=<ci|push|conflict>, what it fixed.
// Unknown keys are ignored, so that a newer version's marker still parses.
package marker

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Version is the marker version this package writes.
const Version = 1

const prefix = "<!-- switchboard "

// Marker is the parsed content of a marker line.
type Marker struct {
	V       int
	Host    string
	Session string
	Run     string
	State   string // running (the claim of a run), progress (its progress comment), queued, waiting, done, attachment
	Reason  string // for waiting: question, permission, error, verify, limit, plan-questions, plan, push, ci, conflict; for queued: the limit; for running: fix on a fix run's claim
	Ack     int64  // the last comment ID the run took in
	At      time.Time
	Mode    string // on a claim made from a start label: plan or direct
	Q       int    // on a post of plan questions: the number of its last question
	SHA     string // on a done post: the commit pushed, whose CI the agent watches
	CR      int    // on a done post: the number of the change request
	Fix     string // on the done post of a fix run: what it fixed (CI, Push or Conflict); empty on any other post
}

// The modes a claim names, and the reasons of the waits in the plan stage.
const (
	ModePlan      = "plan"
	ModeDirect    = "direct"
	PlanQuestions = "plan-questions"
	PlanReady     = "plan"
	// The limits a queued issue waits on: agent.max_parallel, or its project's max_parallel.
	QueuedHost    = "max-parallel"
	QueuedProject = "project-max-parallel"
	// Fix marks the claim of a fix run; Push, CI and Conflict the waits a rejected push, a CI failure and a change
	// request conflicting with its base end in.
	Fix      = "fix"
	Push     = "push"
	CI       = "ci"
	Conflict = "conflict"
)

// The states a marker names.
const (
	Running  = "running"
	Progress = "progress"
	Queued   = "queued" // the comment of an issue waiting for a free slot
	Waiting  = "waiting"
	Done     = "done"
	// Attachment marks the comment that attaches the images of a done run, posted after its done post (ADR-050).
	Attachment = "attachment"
)

// Transient reports whether m marks a post that says nothing about why an issue waits or how a run ended: the claim
// of a run, its progress comment, the queued comment, or the comment that attaches a run's images.
func (m *Marker) Transient() bool {
	return m.State == Running || m.State == Progress || m.State == Queued || m.State == Attachment
}

// clean keeps a value on one token: spaces become underscores and the comment terminator cannot appear.
func clean(s string) string {
	s = strings.Join(strings.Fields(s), "_")
	return strings.ReplaceAll(s, "--", "-")
}

// Format renders m as a marker line (without a trailing newline).
func (m Marker) Format() string {
	var b strings.Builder
	b.WriteString(prefix)
	fmt.Fprintf(&b, "v=%d", Version)
	add := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, " %s=%s", k, clean(v))
		}
	}
	add("host", m.Host)
	add("session", m.Session)
	add("run", m.Run)
	add("state", m.State)
	add("reason", m.Reason)
	if m.Ack != 0 {
		fmt.Fprintf(&b, " ack=%d", m.Ack)
	}
	if !m.At.IsZero() {
		fmt.Fprintf(&b, " at=%s", m.At.UTC().Format(time.RFC3339))
	}
	add("mode", m.Mode)
	if m.Q != 0 {
		fmt.Fprintf(&b, " q=%d", m.Q)
	}
	add("sha", m.SHA)
	if m.CR != 0 {
		fmt.Fprintf(&b, " cr=%d", m.CR)
	}
	add("fix", m.Fix)
	b.WriteString(" -->")
	return b.String()
}

// Parse returns the marker on the last non-blank line of body. A marker anywhere else, or inside a block quote,
// does not count.
func Parse(body string) (*Marker, bool) {
	lines := strings.Split(strings.TrimRight(body, " \t\r\n"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(last, prefix) || !strings.HasSuffix(last, "-->") {
		return nil, false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(last, prefix), "-->"))
	m := &Marker{}
	for _, field := range strings.Fields(inner) {
		k, v, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch k {
		case "v":
			m.V, _ = strconv.Atoi(v)
		case "host":
			m.Host = v
		case "session":
			m.Session = v
		case "run":
			m.Run = v
		case "state":
			m.State = v
		case "reason":
			m.Reason = v
		case "ack":
			m.Ack, _ = strconv.ParseInt(v, 10, 64)
		case "at":
			m.At, _ = time.Parse(time.RFC3339, v)
		case "mode":
			m.Mode = v
		case "q":
			m.Q, _ = strconv.Atoi(v)
		case "sha":
			m.SHA = v
		case "cr":
			m.CR, _ = strconv.Atoi(v)
		case "fix":
			m.Fix = v
		}
	}
	if m.V < 1 {
		return nil, false
	}
	return m, true
}
