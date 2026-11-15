package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// advance commits name with body on branch of origin from a clone of its own.
func advance(t *testing.T, origin, branch, name, body string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "other")
	git(t, filepath.Dir(dir), "clone", "-q", "-b", branch, origin, dir)
	commit(t, dir, name, body)
	git(t, dir, "push", "-q", "origin", branch)
	return git(t, dir, "rev-parse", "HEAD")
}

// pushed is a space whose issue branch has one commit, pushed to origin, and that commit.
func pushed(t *testing.T) (*Space, string, string) {
	t.Helper()
	sp, origin := space(t)
	// The merge commit is made with the clone's identity, as the agent's commits are; CI has no global one.
	git(t, sp.Repo, "config", "user.name", "agent")
	git(t, sp.Repo, "config", "user.email", "agent@example.test")
	dir, err := sp.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "flag.txt", "dry-run\n")
	if err := sp.Push(ctx); err != nil {
		t.Fatal(err)
	}
	sha, err := sp.Head(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return sp, origin, sha
}

func TestMergeBaseTakesInTheBase(t *testing.T) {
	sp, origin, sha := pushed(t)
	base := advance(t, origin, "main", "fix.txt", "fixed\n")
	got, err := sp.MergeBase(ctx, sha)
	if err != nil || got != MergeDone {
		t.Fatalf("MergeBase = %q, %v", got, err)
	}
	head, _ := sp.Head(ctx)
	if head == sha || git(t, sp.Dir(), "merge-base", "--is-ancestor", base, head) != "" || git(t, sp.Dir(), "status", "--porcelain") != "" {
		t.Fatalf("head %s does not hold the base %s", head, base)
	}
}

func TestMergeBaseWithNothingToTakeIn(t *testing.T) {
	sp, _, sha := pushed(t)
	got, err := sp.MergeBase(ctx, sha)
	head, _ := sp.Head(ctx)
	if err != nil || got != MergeNone || head != sha {
		t.Fatalf("MergeBase = %q, %v; head %s", got, err, head)
	}
}

func TestMergeBaseStopsAtAConflict(t *testing.T) {
	sp, origin, sha := pushed(t)
	advance(t, origin, "main", "flag.txt", "verbose\n")
	got, err := sp.MergeBase(ctx, sha)
	head, _ := sp.Head(ctx)
	if err != nil || got != MergeConflict || head != sha || git(t, sp.Dir(), "status", "--porcelain") != "" {
		t.Fatalf("MergeBase = %q, %v; head %s, status %q", got, err, head, git(t, sp.Dir(), "status", "--porcelain"))
	}
}

func TestMergeBaseRefusesABranchThatMoved(t *testing.T) {
	sp, origin, sha := pushed(t)
	advance(t, origin, sp.IssueBranch(), "person.txt", "a person's commit\n")
	if _, err := sp.MergeBase(ctx, sha); !errors.Is(err, ErrMoved) {
		t.Fatalf("err = %v, want ErrMoved", err)
	}
}

// A worktree removed after the done post is made again on the pushed commit.
func TestMergeBaseRecreatesTheWorktree(t *testing.T) {
	sp, origin, sha := pushed(t)
	if _, err := sp.Remove(ctx); err != nil {
		t.Fatal(err)
	}
	advance(t, origin, "main", "fix.txt", "fixed\n")
	got, err := sp.MergeBase(ctx, sha)
	if err != nil || got != MergeDone {
		t.Fatalf("MergeBase = %q, %v", got, err)
	}
	if git(t, sp.Dir(), "merge-base", "--is-ancestor", sha, "HEAD") != "" {
		t.Fatal("the merge does not hold the pushed commit")
	}
}

func TestMergeBaseKeepsUncommittedChanges(t *testing.T) {
	sp, _, sha := pushed(t)
	commitless := filepath.Join(sp.Dir(), "notes.txt")
	if err := os.WriteFile(commitless, []byte("draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := sp.MergeBase(ctx, sha); !errors.Is(err, ErrUncommitted) {
		t.Fatalf("err = %v, want ErrUncommitted", err)
	}
	if _, err := os.Stat(commitless); err != nil {
		t.Fatal("the uncommitted file is gone")
	}
}
