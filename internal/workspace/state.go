package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// GitState is what git sees of a worktree: the commit it is on, the tracked files that differ from it, and the
// untracked files that are not ignored. Ignored files are no part of it.
type GitState struct {
	head   string
	status string // git status --porcelain --untracked-files=all
	diff   string // the SHA-256 of git diff HEAD: a file that already differed and is rewritten keeps its status line
}

// State returns the worktree's GitState.
func (s *Space) State(ctx context.Context) (GitState, error) {
	head, err := s.Head(ctx)
	if err != nil {
		return GitState{}, err
	}
	status, err := s.git(ctx, s.Dir(), nil, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return GitState{}, err
	}
	diff, err := s.git(ctx, s.Dir(), nil, "diff", "--binary", "HEAD")
	if err != nil {
		return GitState{}, err
	}
	sum := sha256.Sum256([]byte(diff))
	return GitState{head: head, status: status, diff: hex.EncodeToString(sum[:])}, nil
}

// Changed reports whether after differs from before, and the paths whose status line is new in after. The paths are
// empty when only the commit or the content of files that already differed changed.
func Changed(before, after GitState) (paths []string, changed bool) {
	if before == after {
		return nil, false
	}
	had := map[string]bool{}
	for _, l := range strings.Split(before.status, "\n") {
		had[l] = true
	}
	for _, l := range strings.Split(after.status, "\n") {
		if len(l) > 3 && !had[l] {
			paths = append(paths, l[3:])
		}
	}
	return paths, true
}
