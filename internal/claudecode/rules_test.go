package claudecode

import (
	"slices"
	"testing"
)

func TestCommands(t *testing.T) {
	got := Commands("git log", "make")
	want := []string{"Bash(git log)", "Bash(git log *)", "Bash(make)", "Bash(make *)"}
	if !slices.Equal(got, want) {
		t.Fatalf("Commands = %q, want %q", got, want)
	}
}

func TestToolName(t *testing.T) {
	for rule, want := range map[string]string{"Bash(openspec *)": "Bash", " Skill ": "Skill", "mcp__x__y": "mcp__x__y", "  ": ""} {
		if got := ToolName(rule); got != want {
			t.Errorf("ToolName(%q) = %q, want %q", rule, got, want)
		}
	}
}

func TestMaxTurns(t *testing.T) {
	if !(&Result{Subtype: "error_max_turns"}).MaxTurns() || (&Result{Subtype: "success"}).MaxTurns() {
		t.Fatal("MaxTurns reads the subtype wrong")
	}
}
