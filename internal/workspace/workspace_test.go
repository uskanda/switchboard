package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var ctx = context.Background()

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// repo makes a bare origin with one commit on main and a clone of it, and returns the clone and the origin.
func repo(t *testing.T) (clone, origin string) {
	t.Helper()
	root := t.TempDir()
	origin = filepath.Join(root, "origin.git")
	seed := filepath.Join(root, "seed")
	git(t, root, "init", "-q", "--bare", "-b", "main", origin)
	git(t, root, "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "AGENTS.md"), []byte("rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, seed, "add", "AGENTS.md")
	git(t, seed, "commit", "-q", "-m", "init")
	git(t, seed, "push", "-q", origin, "main")
	clone = filepath.Join(root, "clone")
	git(t, root, "clone", "-q", origin, clone)
	return clone, origin
}

func space(t *testing.T) (*Space, string) {
	clone, origin := repo(t)
	return &Space{Repo: clone, Root: t.TempDir(), Base: "main", Number: 12, Env: os.Environ()}, origin
}

func commit(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", "change "+name)
}

func TestPrepareCreatesAndReusesTheWorktree(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := git(t, dir, "branch", "--show-current"); got != "switchboard/issue-12" {
		t.Fatalf("branch = %q", got)
	}
	commit(t, dir, "a.txt", "a")
	again, err := s.Prepare(ctx)
	if err != nil || again != dir {
		t.Fatalf("second Prepare = %q, %v", again, err)
	}
	if _, err := os.Stat(filepath.Join(again, "a.txt")); err != nil {
		t.Fatal("the reused worktree lost its commit")
	}
}

func TestCheckClaudeConfig(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CheckClaudeConfig(ctx); err != nil {
		t.Fatalf("a fresh worktree must pass: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claude", "settings.json"), []byte(`{"hooks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err = s.CheckClaudeConfig(ctx)
	if err == nil || !strings.Contains(err.Error(), ".claude/settings.json") {
		t.Fatalf("an untracked .claude/settings.json must fail: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".claude")); err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "AGENTS.md", "new rules\n")
	if err := s.CheckClaudeConfig(ctx); err == nil || !strings.Contains(err.Error(), "AGENTS.md") {
		t.Fatalf("a committed AGENTS.md change must fail: %v", err)
	}
}

func TestCheckOutputNeedsCommits(t *testing.T) {
	s, _ := space(t)
	if _, err := s.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	problems, err := s.CheckOutput(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "no commit") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestCheckOutputRefusesProtectedPaths(t *testing.T) {
	s, _ := space(t)
	dir, _ := s.Prepare(ctx)
	commit(t, dir, ".github/workflows/ci.yml", "on: push\n")
	commit(t, dir, "docs/x.md", "x\n")
	problems, err := s.CheckOutput(ctx, []string{"docs/"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, ".github/workflows/ci.yml") || !strings.Contains(joined, "docs/x.md") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestCheckOutputFindsSecretsWithoutEchoingThem(t *testing.T) {
	s, _ := space(t)
	dir, _ := s.Prepare(ctx)
	token := "ghp_" + strings.Repeat("a1B2", 9)
	commit(t, dir, "config.go", "const key = \""+token+"\"\n")
	problems, err := s.CheckOutput(ctx, nil, []string{"a clean comment", "another " + token})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(problems, "\n")
	if !strings.Contains(joined, "config.go") || !strings.Contains(joined, "text 2") {
		t.Fatalf("problems = %v", problems)
	}
	if strings.Contains(joined, token) || strings.Contains(joined, "a1B2") {
		t.Fatalf("a problem echoes the secret: %v", problems)
	}
}

func TestCheckOutputPassesCleanWork(t *testing.T) {
	s, _ := space(t)
	dir, _ := s.Prepare(ctx)
	commit(t, dir, "main.go", "package main\n")
	problems, err := s.CheckOutput(ctx, nil, []string{"done"})
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems = %v, %v", problems, err)
	}
}

func TestPushSendsOnlyTheIssueBranch(t *testing.T) {
	s, origin := space(t)
	dir, _ := s.Prepare(ctx)
	commit(t, dir, "main.go", "package main\n")
	if err := s.Push(ctx); err != nil {
		t.Fatal(err)
	}
	refs := git(t, origin, "for-each-ref", "--format=%(refname)")
	if !strings.Contains(refs, "refs/heads/switchboard/issue-12") {
		t.Fatalf("origin refs = %s", refs)
	}
	if got := git(t, origin, "log", "--format=%s", "-1", "main"); got != "init" {
		t.Fatalf("main moved: %s", got)
	}
}

func TestPushRefusesTheBaseBranch(t *testing.T) {
	s, _ := space(t)
	s.Branch = "main"
	if err := s.Push(ctx); err == nil {
		t.Fatal("pushing the base branch must be refused")
	}
}

func TestCheckOutputRefusesAnotherBranch(t *testing.T) {
	s, _ := space(t)
	dir, _ := s.Prepare(ctx)
	git(t, dir, "switch", "-q", "-c", "elsewhere")
	commit(t, dir, "main.go", "package main\n")
	problems, err := s.CheckOutput(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "elsewhere") {
		t.Fatalf("problems = %v", problems)
	}
}

// Runs of one project prepare their worktrees from the same clone at once; git locks the clone's config, so
// unserialized, one of them fails.
func TestPrepareOfOneCloneAtOnce(t *testing.T) {
	clone, _ := repo(t)
	root := t.TempDir()
	const n = 6
	errs := make(chan error, n)
	for i := range n {
		s := &Space{Repo: clone, Root: root, Base: "main", Number: 100 + i, Env: os.Environ()}
		go func() {
			_, err := s.Prepare(ctx)
			errs <- err
		}()
	}
	for range n {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

// placeholder writes what Claude Code's sandbox puts at a path it denies writes to: an empty file nobody may write.
func placeholder(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o444); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareWritesNoConfig(t *testing.T) {
	s, _ := space(t)
	// A running sandbox of another run of the clone holds .git/config.lock (#128).
	placeholder(t, filepath.Join(s.Repo, ".git", "config.lock"))
	if _, err := s.Prepare(ctx); err != nil {
		t.Fatalf("Prepare must not need .git/config: %v", err)
	}
}

func TestPrepareRemovesTheSandboxPlaceholdersOfAKilledRun(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "empty.txt", "")
	if err := os.Chmod(filepath.Join(dir, "empty.txt"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	left := []string{".mcp.json", ".bashrc", ".claude/settings.json", ".claude/agents"}
	for _, p := range left {
		placeholder(t, filepath.Join(dir, p))
	}
	if _, err := s.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	for _, p := range left {
		if _, err := os.Lstat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Errorf("the placeholder %s is still there: %v", p, err)
		}
	}
	for _, p := range []string{"empty.txt", "notes"} {
		if _, err := os.Lstat(filepath.Join(dir, p)); err != nil {
			t.Errorf("%s is not a placeholder and must stay: %v", p, err)
		}
	}
	if err := s.CheckClaudeConfig(ctx); err != nil {
		t.Fatalf("the worktree must pass once the placeholders are gone: %v", err)
	}
}

func TestClearConfigLock(t *testing.T) {
	clone, _ := repo(t)
	lock := filepath.Join(clone, ".git", "config.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if removed, err := ClearConfigLock(ctx, clone); err != nil || removed {
		t.Fatalf("git's own lock must stay: %v, %v", removed, err)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	placeholder(t, lock)
	if removed, err := ClearConfigLock(ctx, clone); err != nil || !removed {
		t.Fatalf("the placeholder must go: %v, %v", removed, err)
	}
	if _, err := os.Lstat(lock); !os.IsNotExist(err) {
		t.Fatalf("config.lock is still there: %v", err)
	}
	if removed, err := ClearConfigLock(ctx, clone); err != nil || removed {
		t.Fatalf("no lock: %v, %v", removed, err)
	}
}
