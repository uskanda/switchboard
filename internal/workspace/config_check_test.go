package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// advanceBase commits name with body on main of origin, from a clone of its own.
func advanceBase(t *testing.T, origin, name, body string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(other), "clone", "-q", origin, other)
	commit(t, other, name, body)
	git(t, other, "push", "-q", "origin", "main")
}

func TestCheckClaudeConfigPassesAfterTheBaseChangedItsConfiguration(t *testing.T) {
	s, origin := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "work.txt", "w\n")
	advanceBase(t, origin, "AGENTS.md", "newer rules\n")
	advanceBase(t, origin, "sub/CLAUDE.md", "nested\n")
	if _, err := s.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckClaudeConfig(ctx); err != nil {
		t.Fatalf("a base branch that moved on must not stop the issue: %v", err)
	}
	// What the issue branch itself changes is still caught.
	commit(t, dir, "AGENTS.md", "branch rules\n")
	if err := s.CheckClaudeConfig(ctx); err == nil || !strings.Contains(err.Error(), "AGENTS.md") {
		t.Fatalf("a committed AGENTS.md change must fail: %v", err)
	}
}

func TestCheckClaudeConfigLooksBelowTheRoot(t *testing.T) {
	for _, name := range []string{"sub/CLAUDE.md", "a/b/AGENTS.md", "pkg/.claude/settings.json"} {
		t.Run(name, func(t *testing.T) {
			s, _ := space(t)
			dir, err := s.Prepare(ctx)
			if err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := s.CheckClaudeConfig(ctx); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("an untracked %s must fail: %v", name, err)
			}
			git(t, dir, "add", name)
			git(t, dir, "commit", "-q", "-m", "add "+name)
			if err := s.CheckClaudeConfig(ctx); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("a committed %s must fail: %v", name, err)
			}
		})
	}
}

func TestCheckClaudeConfigLeavesIgnoredFilesBelowTheRootAlone(t *testing.T) {
	s, origin := space(t)
	advanceBase(t, origin, ".gitignore", "node_modules/\n")
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "node_modules", "pkg", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckClaudeConfig(ctx); err != nil {
		t.Fatalf("an ignored nested AGENTS.md must pass: %v", err)
	}
}

func TestCheckOutputRefusesNestedConfigurationAndRenamesOutOfAProtectedPath(t *testing.T) {
	s, origin := space(t)
	advanceBase(t, origin, ".github/workflows/ci.yml", "on: push\n")
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "sub/CLAUDE.md", "x\n")
	commit(t, dir, "pkg/.claude/settings.json", "{}\n")
	git(t, dir, "mv", ".github/workflows/ci.yml", "ci.yml")
	git(t, dir, "commit", "-q", "-m", "move")
	problems, err := s.CheckOutput(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(problems, "\n")
	for _, want := range []string{"sub/CLAUDE.md", "pkg/.claude/settings.json", ".github/workflows/ci.yml"} {
		if !strings.Contains(joined, "protected path "+want) {
			t.Errorf("problems do not name %s: %v", want, problems)
		}
	}
}

func TestIsProtected(t *testing.T) {
	for path, want := range map[string]bool{
		"CLAUDE.md": true, "x/CLAUDE.md": true, "x/AGENTS.md": true, ".claude/a": true, "x/.claude/a": true,
		".mcp.json": true, "x/.mcp.json": false, "x/.github/workflows/a.yml": false, "CLAUDE.md.bak": false,
		"x/claude/a": false, "docs/a.md": false,
	} {
		if got := isProtected(path, nil); got != want {
			t.Errorf("isProtected(%q) = %v, want %v", path, got, want)
		}
	}
}
