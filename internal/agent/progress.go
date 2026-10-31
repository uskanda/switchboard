package agent

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// purposeLen is how many characters of the assistant's latest text a progress line keeps.
const purposeLen = 120

// progressLog is the lines of a run's progress comment: one a minute, unless nothing changed.
type progressLog struct {
	msg     texts
	started time.Time
	lines   []string
	last    string // the tools and purpose of the last line, which a new line must differ from
}

// add appends the line for now, given the tool counts and the assistant's latest text, and reports whether it did.
// A line with the same tools and purpose as the last one is not added.
func (l *progressLog) add(now time.Time, tools map[string]int, text string) bool {
	what := l.tools(tools) + l.msg.purposeSep + purpose(text)
	if len(l.lines) > 0 && what == l.last {
		return false
	}
	l.last = what
	l.lines = append(l.lines, now.Format("15:04")+" "+l.elapsed(now.Sub(l.started))+" "+what)
	return true
}

func (l *progressLog) tools(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for n, c := range counts {
		if c > 0 {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return l.msg.noTools
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf("%s %d", n, counts[n])
	}
	return strings.Join(parts, l.msg.toolSep)
}

func (l *progressLog) elapsed(d time.Duration) string {
	m := int(d.Minutes())
	if m >= 60 {
		return fmt.Sprintf(l.msg.hours, m/60, m%60)
	}
	return fmt.Sprintf(l.msg.minutes, m)
}

// purpose is the first non-blank line of text, cut to purposeLen characters; "?" when there is none.
func purpose(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			if r := []rune(line); len(r) > purposeLen {
				line = string(r[:purposeLen])
			}
			return line
		}
	}
	return "?"
}
