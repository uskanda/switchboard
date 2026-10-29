// Package labels keeps exactly one state label (ready, running, waiting, done) on each issue.
package labels

import (
	"context"
	"fmt"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/state"
)

// colors and descriptions of the state labels, in the order ready, running, waiting, done.
var meta = [4]struct{ color, description string }{
	{"0e8a16", "switchboard: the agent may start"},
	{"1d76db", "switchboard: the agent is running"},
	{"d93f0b", "switchboard: the agent waits for a human reply"},
	{"5319e7", "switchboard: the agent finished"},
}

// ordered returns the state label names from the highest priority to the lowest: running, waiting, ready, done.
func ordered(n config.Labels) []string { return []string{n.Running, n.Waiting, n.Ready, n.Done} }

// Sync creates the four state labels in every project and returns how many it created.
func Sync(ctx context.Context, f forge.Forge, projects []config.Project, n config.Labels) (int, error) {
	created := 0
	for _, p := range projects {
		for i, name := range []string{n.Ready, n.Running, n.Waiting, n.Done} {
			ok, err := f.EnsureLabel(ctx, p.Owner, p.Repo, name, meta[i].color, meta[i].description)
			if err != nil {
				return created, fmt.Errorf("%s: label %s: %w", p.FullName(), name, err)
			}
			if ok {
				created++
			}
		}
	}
	return created, nil
}

func isState(l string, n config.Labels) bool {
	return l == n.Ready || l == n.Running || l == n.Waiting || l == n.Done
}

// StateOf returns the highest-priority state label in labels, or "".
func StateOf(labels []string, n config.Labels) string {
	for _, s := range ordered(n) {
		for _, l := range labels {
			if l == s {
				return s
			}
		}
	}
	return ""
}

// Normalize keeps only the highest-priority state label and every other label, and reports whether it removed one.
func Normalize(labels []string, n config.Labels) ([]string, bool) {
	keep := StateOf(labels, n)
	out := make([]string, 0, len(labels))
	changed := false
	for _, l := range labels {
		if isState(l, n) && l != keep {
			changed = true
			continue
		}
		if l == keep && keep != "" {
			continue // appended last, once
		}
		out = append(out, l)
	}
	if keep != "" {
		out = append(out, keep)
	}
	return out, changed
}

// Swapper changes the state label of issues.
type Swapper struct {
	Forge forge.Forge
	Names config.Labels
	// Audit records events such as a normalization; it must not be nil.
	Audit state.Auditor
}

// Swap replaces the state labels of ref with to, keeps every other label, and then re-reads the issue. When the
// issue then carries more than one state label (someone changed labels meanwhile), it normalizes them and records
// the event.
func (s Swapper) Swap(ctx context.Context, ref forge.IssueRef, to string) error {
	is, _, err := s.Forge.Thread(ctx, ref)
	if err != nil {
		return err
	}
	next := []string{}
	for _, l := range is.Labels {
		if !isState(l, s.Names) {
			next = append(next, l)
		}
	}
	next = append(next, to)
	if err := s.Forge.SetLabels(ctx, ref, next); err != nil {
		return fmt.Errorf("%s: set labels: %w", ref, err)
	}
	after, _, err := s.Forge.Thread(ctx, ref)
	if err != nil {
		return err
	}
	fixed, changed := Normalize(after.Labels, s.Names)
	if !changed {
		return nil
	}
	if err := s.Forge.SetLabels(ctx, ref, fixed); err != nil {
		return fmt.Errorf("%s: normalize labels: %w", ref, err)
	}
	s.Audit.Log("labels.normalized", map[string]any{"issue": ref.String(), "found": after.Labels, "kept": StateOf(fixed, s.Names)})
	return nil
}
