// Package marker writes and reads the hidden line that ends every comment the agent posts:
//
//	<!-- switchboard v=1 host=<host> session=<id> run=<id> state=<state> reason=<reason> ack=<comment-id> at=<RFC3339> -->
//
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
	State   string // running, waiting, done
	Reason  string // for waiting: question, permission, error, verify, limit
	Ack     int64  // the last comment ID the run took in
	At      time.Time
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
		}
	}
	if m.V < 1 {
		return nil, false
	}
	return m, true
}
