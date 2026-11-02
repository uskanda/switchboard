package inbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

// The values of MergeStatus.Mergeable.
const (
	MergeClean     = "clean"     // the forge found no conflict with the base
	MergeConflict  = "conflict"  // the change request conflicts with its base
	MergeComputing = "computing" // the forge still computes it
)

// The values of MergeStatus.CI.
const (
	CIRead      = "ok"        // the checks of the commit were read, and there is at least one
	CINone      = "none"      // the commit has no checks
	CIForbidden = "forbidden" // the inbox token may not read CI (Actions and Commit statuses on GitHub)
)

// The values of CheckStatus.Status.
const (
	CheckSuccess = "success"
	CheckFailure = "failure" // failed or timed out
	CheckRunning = "running" // not finished
	CheckOther   = "other"   // skipped, cancelled, neutral and the like
)

// CheckStatus is one check of the commit, as the merge dialog shows it. Its name is the forge's data.
type CheckStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// MergeStatus is where the change request of a done issue stands before a merge: its state on the forge, whether
// its head is still the commit the page showed, whether it conflicts with its base, and the checks of that commit.
type MergeStatus struct {
	State     string        `json:"state"` // forge.CROpen, CRClosed or CRMerged
	HeadMoved bool          `json:"head_moved"`
	Mergeable string        `json:"mergeable"`
	CI        string        `json:"ci"`
	Checks    []CheckStatus `json:"checks"`
}

// MergeStatus reads the change request the agent's latest done post names, under the same conditions as Merge: the
// issue is open and done, and that post's commit is sha (ErrStale otherwise). c reads with the inbox token (ADR-047).
// A token that may not read CI is not an error: CI is then CIForbidden and only the change request is reported.
func (b *Inbox) MergeStatus(ctx context.Context, c forge.CI, ref forge.IssueRef, sha string) (*MergeStatus, error) {
	if strings.TrimSpace(sha) == "" {
		return nil, wrap(ErrInvalid, errors.New("the commit is empty"))
	}
	if c == nil {
		return nil, fmt.Errorf("no client reads change requests on %s", forgeName(ref.Kind()))
	}
	dm, err := b.doneChange(ctx, ref, sha, "it cannot be merged from the inbox")
	if err != nil {
		return nil, err
	}
	// The change request and the checks are read at the same time: neither needs the other.
	var checks []forge.Check
	var ciErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		checks, ciErr = c.CommitChecks(ctx, ref.Owner, ref.Repo, dm.SHA)
	}()
	cr, err := c.ChangeRequestHead(ctx, ref.Owner, ref.Repo, dm.CR)
	<-done
	if err != nil {
		return nil, err
	}
	out := &MergeStatus{State: cr.State, HeadMoved: cr.Head != dm.SHA, Mergeable: MergeClean, CI: CIRead, Checks: []CheckStatus{}}
	switch {
	case cr.Conflict:
		out.Mergeable = MergeConflict
	case cr.Computing:
		out.Mergeable = MergeComputing
	}
	switch {
	case errors.Is(ciErr, forge.ErrForbidden):
		out.CI = CIForbidden
		return out, nil
	case ciErr != nil:
		return nil, ciErr
	case len(checks) == 0:
		out.CI = CINone
	}
	for _, ch := range checks {
		out.Checks = append(out.Checks, CheckStatus{Name: ch.Name, Status: checkStatus(ch), URL: ch.URL})
	}
	return out, nil
}

// checkStatus sorts a check into the four kinds the dialog counts.
func checkStatus(c forge.Check) string {
	switch {
	case !c.Done:
		return CheckRunning
	case c.Failed():
		return CheckFailure
	case c.Conclusion == forge.Success:
		return CheckSuccess
	}
	return CheckOther
}
