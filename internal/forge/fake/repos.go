package fake

import (
	"context"
	"sort"

	"github.com/uskanda/switchboard/internal/forge"
)

// SetDefaultBranch sets the default branch of a repository; it is main until set.
func (f *Forge) SetDefaultBranch(owner, name, branch string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos[owner+"/"+name].branch = branch
}

// DefaultBranch implements forge.Repos.
func (f *Forge) DefaultBranch(_ context.Context, owner, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.repos[owner+"/"+name]
	if !ok {
		return "", forge.ErrNotFound
	}
	if r.branch == "" {
		return "main", nil
	}
	return r.branch, nil
}

// LabelNames implements forge.Repos, in name order.
func (f *Forge) LabelNames(_ context.Context, owner, name string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.repos[owner+"/"+name]
	if !ok {
		return nil, forge.ErrNotFound
	}
	var out []string
	for l := range r.labels {
		out = append(out, l)
	}
	sort.Strings(out)
	return out, nil
}
