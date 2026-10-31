// Package labels keeps exactly one state label (ready, running, waiting, done) on each issue. The three start labels
// (ready, plan, go) all mean ready; which one is on the issue decides whether its run plans first.
package labels

import (
	"context"
	"fmt"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/state"
)

// colors and descriptions of the state labels, in the order ready, plan, go, running, waiting, done.
var meta = [6]struct{ color, description string }{
	{"0e8a16", "switchboard: the agent may start"},
	{"bfd4f2", "switchboard: the agent may start, with a plan to approve first"},
	{"2cbe4e", "switchboard: the agent may start, without a plan"},
	{"1d76db", "switchboard: the agent is running"},
	{"d93f0b", "switchboard: the agent waits for a human reply"},
	{"5319e7", "switchboard: the agent finished"},
}

// ordered returns the state label names from the highest priority to the lowest: running, waiting, the start
// labels, done.
func ordered(n config.Labels) []string {
	return []string{n.Running, n.Waiting, n.Ready, n.Plan, n.Go, n.Done}
}

// Sync creates the six state labels in every project and returns how many it created. An empty name is skipped.
func Sync(ctx context.Context, f forge.Forge, projects []config.Project, n config.Labels) (int, error) {
	created := 0
	for _, p := range projects {
		for i, name := range []string{n.Ready, n.Plan, n.Go, n.Running, n.Waiting, n.Done} {
			if name == "" {
				continue
			}
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
	return l != "" && (IsStart(l, n) || l == n.Running || l == n.Waiting || l == n.Done)
}

// IsStart reports whether l is one of the start labels: ready, plan or go.
func IsStart(l string, n config.Labels) bool {
	return l != "" && (l == n.Ready || l == n.Plan || l == n.Go)
}

// StateOf returns the highest-priority state label in labels, or "". The start labels share one rank; among them,
// ready comes before plan and plan before go.
func StateOf(labels []string, n config.Labels) string {
	for _, s := range ordered(n) {
		for _, l := range labels {
			if l == s && s != "" {
				return s
			}
		}
	}
	return ""
}

// MultipleStarts reports whether labels hold two or more start labels and no running or waiting label above them:
// the agent cannot tell which run was asked for.
func MultipleStarts(labels []string, n config.Labels) bool {
	if !IsStart(StateOf(labels, n), n) {
		return false
	}
	starts := 0
	for _, l := range labels {
		if IsStart(l, n) {
			starts++
		}
	}
	return starts > 1
}

// Normalize keeps only the highest-priority state label and every other label, and reports whether it removed one.
// When that label is one of several start labels, it keeps all of them, for the agent to report (MultipleStarts).
func Normalize(labels []string, n config.Labels) ([]string, bool) {
	keep := StateOf(labels, n)
	out := make([]string, 0, len(labels))
	changed := false
	if MultipleStarts(labels, n) {
		for _, l := range labels {
			if isState(l, n) && !IsStart(l, n) {
				changed = true
				continue
			}
			out = append(out, l)
		}
		return out, changed
	}
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
