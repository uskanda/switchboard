package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func prepared(t *testing.T) (*Space, string, string) {
	t.Helper()
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	head, err := s.Head(ctx)
	if err != nil || head == "" {
		t.Fatalf("Head = %q, %v", head, err)
	}
	return s, dir, head
}

func TestRestoreLeavesAnUntouchedWorktree(t *testing.T) {
	s, _, head := prepared(t)
	changed, err := s.Restore(ctx, head)
	if err != nil || changed {
		t.Fatalf("Restore = %v, %v; want no change", changed, err)
	}
}

func TestRestoreUndoesACommit(t *testing.T) {
	s, dir, head := prepared(t)
	commit(t, dir, "a.txt", "a")
	changed, err := s.Restore(ctx, head)
	if err != nil || !changed {
		t.Fatalf("Restore = %v, %v; want a change undone", changed, err)
	}
	if got := git(t, dir, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD = %s, want %s", got, head)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("a.txt is still there")
	}
}

func TestRestoreUndoesEditsAndNewFiles(t *testing.T) {
	s, dir, head := prepared(t)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := s.Restore(ctx, head)
	if err != nil || !changed {
		t.Fatalf("Restore = %v, %v; want a change undone", changed, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md")); string(b) != "rules\n" {
		t.Fatalf("AGENTS.md = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); !os.IsNotExist(err) {
		t.Fatal("the new directory is still there")
	}
}

func TestRestoreKeepsIgnoredFiles(t *testing.T) {
	s, dir, _ := prepared(t)
	commit(t, dir, ".gitignore", "build/\n")
	head, err := s.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build", "out"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.Restore(ctx, head); err != nil || changed {
		t.Fatalf("Restore = %v, %v; an ignored file is no change", changed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "build", "out")); err != nil {
		t.Fatal("the ignored file was removed")
	}
}
