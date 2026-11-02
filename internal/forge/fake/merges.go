package fake

import (
	"context"
	"fmt"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Merges = (*Forge)(nil)

// Change is the state of one pull request (merge request) as the merge sees it.
type Change struct {
	Draft  bool
	Head   string
	Merged bool
	// Refuse, when set, is the reason the forge refuses to merge it.
	Refuse string
}

func changeKey(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s#%d", owner, repo, number)
}

// SetChange records the state of change request number of owner/repo.
func (f *Forge) SetChange(owner, repo string, number int, c Change) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.changes == nil {
		f.changes = map[string]*Change{}
	}
	f.changes[changeKey(owner, repo, number)] = &c
}

// ChangeState returns the state of change request number of owner/repo, and whether there is one.
func (f *Forge) ChangeState(owner, repo string, number int) (Change, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.changes[changeKey(owner, repo, number)]
	if !ok {
		return Change{}, false
	}
	return *c, true
}

// MergeChangeRequest implements forge.Merges. FailMerge, when set, is returned instead.
func (f *Forge) MergeChangeRequest(_ context.Context, owner, repo string, number int, sha string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailMerge != nil {
		return f.FailMerge
	}
	c, ok := f.changes[changeKey(owner, repo, number)]
	switch {
	case !ok:
		return forge.ErrNotFound
	case c.Merged:
		return &forge.NotMergeableError{Reason: fmt.Sprintf("#%d is already merged", number)}
	case c.Head != sha:
		return forge.ErrHeadMoved
	case c.Refuse != "":
		return &forge.NotMergeableError{Reason: c.Refuse}
	}
	c.Draft, c.Merged = false, true
	return nil
}
