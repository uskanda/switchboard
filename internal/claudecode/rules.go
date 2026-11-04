package claudecode

import "strings"

// This file holds the grammar of Claude Code's permission rules and result subtypes, so that the packages that build
// settings or read results do not spell them out.

// Commands returns the rules that match each command alone and with any arguments: "Bash(git log)" and
// "Bash(git log *)" for "git log".
func Commands(cmds ...string) []string {
	out := make([]string, 0, 2*len(cmds))
	for _, c := range cmds {
		out = append(out, "Bash("+c+")", "Bash("+c+" *)")
	}
	return out
}

// ToolName returns the tool a rule names: Bash for "Bash(openspec *)", Skill for "Skill". It is empty for a blank rule.
func ToolName(rule string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(rule), "(")
	return strings.TrimSpace(name)
}

// MaxTurns reports whether the run ended because it reached the limit of turns.
func (r *Result) MaxTurns() bool { return r.Subtype == "error_max_turns" }
