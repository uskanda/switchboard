package workspace

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func hasBranch(dir, b string) bool {
	return exec.Command("git", "-C", dir, "rev-parse", "-q", "--verify", "refs/heads/"+b).Run() == nil
}

func TestRemoveDeletesTheWorktreeAndTheLocalBranch(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "a.txt", "a") // committed, so nothing is lost that git does not keep elsewhere
	removed, err := s.Remove(ctx)
	if err != nil || !removed {
		t.Fatalf("Remove = %v, %v", removed, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the worktree is still there: %v", err)
	}
	if hasBranch(s.Repo, "switchboard/issue-12") {
		t.Fatal("the local branch is still there")
	}
	// Nothing to remove the second time.
	if removed, err := s.Remove(ctx); err != nil || removed {
		t.Fatalf("second Remove = %v, %v", removed, err)
	}
}

func TestRemoveKeepsUncommittedChanges(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "draft.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := s.Remove(ctx)
	if !errors.Is(err, ErrUncommitted) || removed {
		t.Fatalf("Remove = %v, %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "draft.txt")); err != nil {
		t.Fatal("the uncommitted file is gone")
	}
	if !hasBranch(s.Repo, "switchboard/issue-12") {
		t.Fatal("the branch is gone")
	}
}
