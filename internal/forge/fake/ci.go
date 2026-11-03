package fake

import (
	"context"
	"fmt"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.CI = (*Forge)(nil)

// ci is the CI state of the fake: the checks of each commit, the logs of jobs, and where each change request stands.
type ci struct {
	checks    map[string][]forge.Check // "owner/repo@sha" -> checks
	logs      map[int64]string
	heads     map[int]string // index into PRs -> head commit
	merged    map[int]bool
	conflicts map[int]bool // index into PRs -> conflicts with its base
	calls     int          // calls of CommitChecks
	prCalls   int          // calls of ChangeRequestHead
}

func (f *Forge) ciState() *ci {
	if f.ci == nil {
		f.ci = &ci{checks: map[string][]forge.Check{}, logs: map[int64]string{}, heads: map[int]string{}, merged: map[int]bool{}, conflicts: map[int]bool{}}
	}
	return f.ci
}

// SetChecks replaces the checks of the commit sha of owner/repo.
func (f *Forge) SetChecks(owner, repo, sha string, checks ...forge.Check) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ciState().checks[owner+"/"+repo+"@"+sha] = checks
}

// SetLog sets the log CheckLog returns for job.
func (f *Forge) SetLog(job int64, log string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ciState().logs[job] = log
}

// SetPRHead moves the head of the i-th change request in PRs, counted from 0, to sha.
func (f *Forge) SetPRHead(i int, sha string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ciState().heads[i] = sha
}

// MergePR merges the i-th change request in PRs, counted from 0.
func (f *Forge) MergePR(i int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ciState().merged[i] = true
}

// SetConflict sets whether the i-th change request in PRs, counted from 0, conflicts with its base.
func (f *Forge) SetConflict(i int, conflict bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ciState().conflicts[i] = conflict
}

// PRCalls is how many times ChangeRequestHead was called.
func (f *Forge) PRCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ciState().prCalls
}

// CICalls is how many times CommitChecks was called.
func (f *Forge) CICalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ciState().calls
}

// CommitChecks implements forge.CI.
func (f *Forge) CommitChecks(_ context.Context, owner, repo, sha string) ([]forge.Check, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.ciState()
	s.calls++
	if err := f.FailCI; err != nil {
		return nil, err
	}
	return append([]forge.Check(nil), s.checks[owner+"/"+repo+"@"+sha]...), nil
}

// CheckLog implements forge.CI.
func (f *Forge) CheckLog(_ context.Context, _, _ string, c forge.Check) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.FailCI; err != nil {
		return "", err
	}
	return f.ciState().logs[c.Job], nil
}

// ChangeRequestHead implements forge.CI and forge.Merges. A change request recorded with SetChange is read from there;
// any other from PRs.
func (f *Forge) ChangeRequestHead(_ context.Context, owner, repo string, number int) (*forge.ChangeRequestState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FailHead != nil {
		return nil, f.FailHead
	}
	if st := f.changeHead(owner, repo, number); st != nil {
		return st, nil
	}
	i := number - 1
	if i < 0 || i >= len(f.PRs) || f.PRs[i].Owner != owner || f.PRs[i].Repo != repo {
		return nil, fmt.Errorf("fake: change request %s/%s#%d: %w", owner, repo, number, forge.ErrNotFound)
	}
	s := f.ciState()
	s.prCalls++
	st := &forge.ChangeRequestState{State: forge.CROpen, Head: s.heads[i], Conflict: s.conflicts[i]}
	switch {
	case s.merged[i]:
		st.State = forge.CRMerged
	case f.closed[i]:
		st.State = forge.CRClosed
	}
	return st, nil
}
