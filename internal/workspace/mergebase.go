package workspace

import (
	"context"
	"errors"
	"fmt"
)

// The results of MergeBase.
const (
	MergeDone     = "merged"   // the base is merged into the issue branch, in a new commit
	MergeNone     = "none"     // the issue branch already holds the base
	MergeConflict = "conflict" // the base conflicts with the issue branch; nothing is merged
)

// ErrMoved reports an issue branch on origin that is no longer at the commit the caller watches.
var ErrMoved = errors.New("the issue branch on origin moved on")

// MergeBase merges the base branch, as origin has it, into the issue branch at sha, the commit of the issue branch
// on origin, without claude (ADR-075). It makes the worktree when it is gone and puts it on sha; a worktree with
// changes nobody committed is left alone (ErrUncommitted). A conflict is aborted, leaving the worktree on sha.
// Nothing is pushed.
func (s *Space) MergeBase(ctx context.Context, sha string) (string, error) {
	dir, err := s.Prepare(ctx) // fetches the base
	if err != nil {
		return "", err
	}
	if err := s.FetchIssueBranch(ctx); err != nil {
		return "", err
	}
	at, err := s.git(ctx, dir, nil, "rev-parse", "refs/remotes/origin/"+s.branch())
	if err != nil {
		return "", err
	}
	if at != sha {
		return "", fmt.Errorf("%w: %s is at %s, not %s", ErrMoved, s.branch(), shortSHA(at), shortSHA(sha))
	}
	status, err := s.git(ctx, dir, nil, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	if status != "" {
		return "", ErrUncommitted // a change nobody committed is not thrown away
	}
	if _, err := s.git(ctx, dir, nil, "reset", "-q", "--hard", sha); err != nil {
		return "", err
	}
	if _, err := s.git(ctx, dir, nil, "merge", "-q", "--no-edit", s.base()); err != nil {
		unmerged, uerr := s.git(ctx, dir, nil, "diff", "--name-only", "--diff-filter=U")
		if _, aerr := s.git(ctx, dir, nil, "merge", "--abort"); aerr != nil {
			return "", errors.Join(err, aerr)
		}
		if uerr == nil && unmerged != "" {
			return MergeConflict, nil
		}
		return "", err
	}
	head, err := s.Head(ctx)
	if err != nil {
		return "", err
	}
	if head == sha {
		return MergeNone, nil
	}
	return MergeDone, nil
}
