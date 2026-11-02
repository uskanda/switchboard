package agent

import (
	"regexp"
	"strings"

	"github.com/uskanda/switchboard/internal/workspace"
)

// The end of a job's log handed to a fix run: at most logLines lines and logBytes bytes.
const (
	logLines = 200
	logBytes = 20 * 1024
)

var (
	ansi         = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)
	logTimestamp = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z ?`)
	glSection    = regexp.MustCompile(`^section_(?:start|end):\d+:[^\r]*\r?`)
)

// trimLog is the end of a CI job's log as a fix run reads it: colors, GitHub's line timestamps and GitLab's section
// markers removed, lines rewritten with \r kept at their last state, secrets redacted.
func trimLog(raw string) string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = glSection.ReplaceAllString(line, "")
		if i := strings.LastIndex(line, "\r"); i >= 0 {
			line = line[i+1:]
		}
		line = ansi.ReplaceAllString(line, "")
		line = logTimestamp.ReplaceAllString(line, "")
		out = append(out, line)
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	// Drop leading blank lines and runs of them, which removed section markers leave.
	kept := out[:0]
	for _, l := range out {
		if l != "" || len(kept) > 0 && kept[len(kept)-1] != "" {
			kept = append(kept, l)
		}
	}
	if len(kept) > logLines {
		kept = kept[len(kept)-logLines:]
	}
	s := workspace.Redact(strings.Join(kept, "\n"))
	if len(s) > logBytes {
		s = s[len(s)-logBytes:]
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
	}
	return s
}
