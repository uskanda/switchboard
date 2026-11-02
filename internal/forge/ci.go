package forge

import (
	"context"
	"regexp"
	"strconv"
)

// The conclusions of a finished check, as GitHub names them; GitLab's job statuses are mapped onto these.
const (
	Success   = "success"
	Failure   = "failure"
	TimedOut  = "timed_out"
	Cancelled = "cancelled"
	Skipped   = "skipped"
	Neutral   = "neutral"
)

// Check is one CI result on a commit: a GitHub Actions job, a commit status, or a GitLab job.
type Check struct {
	Name        string
	Done        bool   // it finished; Conclusion is empty until then
	Conclusion  string // one of the conclusions above, or another the forge names
	URL         string
	Description string   // what a commit status says about itself
	FailedSteps []string // the steps of a job that failed
	Job         int64    // the job whose log CheckLog reads; 0 for a check without a log, such as a commit status
}

// Failed reports whether c finished in a way the agent counts as a CI failure.
func (c Check) Failed() bool {
	return c.Done && (c.Conclusion == Failure || c.Conclusion == TimedOut)
}

// ChangeRequestState is where a change request stands: open, closed or merged, and the commit its head is at.
type ChangeRequestState struct {
	State string
	Head  string
}

// The states of a change request.
const (
	CROpen   = "open"
	CRClosed = "closed"
	CRMerged = "merged"
)

// CI reads the CI of a commit. It is kept apart from Forge: only the agent watches CI, and the inbox service never
// does. It needs Actions: Read and Commit statuses: Read on GitHub; ErrForbidden reports a token without them.
type CI interface {
	// CommitChecks lists the checks of the commit sha: the jobs of its runs or pipelines and its commit statuses.
	CommitChecks(ctx context.Context, owner, repo, sha string) ([]Check, error)
	// CheckLog returns the log of the job of c, as the forge serves it.
	CheckLog(ctx context.Context, owner, repo string, c Check) (string, error)
	// ChangeRequestHead returns the state and the head commit of the change request number.
	ChangeRequestHead(ctx context.Context, owner, repo string, number int) (*ChangeRequestState, error)
}

var crNumber = regexp.MustCompile(`/(?:pull|merge_requests)/(\d+)/?$`)

// ChangeRequestNumber reads the number of a change request from its URL, or 0.
func ChangeRequestNumber(url string) int {
	m := crNumber.FindStringSubmatch(url)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
