package inbox

import (
	"regexp"
	"strings"

	"github.com/uskanda/switchboard/internal/permissions"
)

// Denial is one operation a run was refused, as the agent's permission wait lists it, with the allow rules that
// would let it through (permissions.Candidates). Command is the forge's data, written by claude: show it as text.
type Denial struct {
	Tool    string `json:"tool"`
	Command string `json:"command,omitempty"`
	// Candidates are the rules the web inbox offers; empty when none would match, such as for a Compound command.
	Candidates []string `json:"candidates"`
	// Compound is set on a command joined with others, which no rule matches.
	Compound bool `json:"compound,omitempty"`
}

// deniedHeadings are the headings the agent writes above the refused operations, in each language of its comments.
var deniedHeadings = []string{"**Refused operations**", "**拒否された操作**"}

var deniedLine = regexp.MustCompile("^- ([A-Za-z0-9_]+)(?:: `(.*)`)?$")

// ParseDenials reads the refused operations of the body of the agent's permission wait: the "- Tool" and
// "- Tool: `command`" lines that follow one of deniedHeadings (add-permission-grants).
func ParseDenials(body string) []Denial {
	lines := strings.Split(body, "\n")
	out := []Denial{}
	for i, l := range lines {
		if !isDeniedHeading(strings.TrimSpace(l)) {
			continue
		}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		for ; j < len(lines); j++ {
			m := deniedLine.FindStringSubmatch(strings.TrimRight(lines[j], " \r"))
			if m == nil {
				break
			}
			cands, compound := permissions.Candidates(m[1], m[2])
			if cands == nil {
				cands = []string{}
			}
			out = append(out, Denial{Tool: m[1], Command: m[2], Candidates: cands, Compound: compound})
		}
		break
	}
	return out
}

func isDeniedHeading(l string) bool {
	for _, h := range deniedHeadings {
		if l == h {
			return true
		}
	}
	return false
}
