package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.CI = (*Client)(nil)

// CommitChecks reads the jobs of the pipelines of sha. A failed job allowed to fail counts as neutral, a manual job
// that nobody started as skipped, and a job stopped by its timeout as timed out. A pipeline without jobs yet counts
// as one unfinished check.
func (c *Client) CommitChecks(ctx context.Context, owner, repo, sha string) ([]forge.Check, error) {
	var pipelines []struct {
		ID     int64  `json:"id"`
		WebURL string `json:"web_url"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.url(project(owner, repo)+"/pipelines", url.Values{"sha": {sha}, "per_page": {"100"}}), nil, &pipelines); err != nil {
		return nil, err
	}
	var checks []forge.Check
	for _, p := range pipelines {
		var jobs []struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			Status        string `json:"status"`
			WebURL        string `json:"web_url"`
			AllowFailure  bool   `json:"allow_failure"`
			FailureReason string `json:"failure_reason"`
		}
		path := fmt.Sprintf("%s/pipelines/%d/jobs", project(owner, repo), p.ID)
		if _, err := c.do(ctx, http.MethodGet, c.url(path, url.Values{"per_page": {"100"}}), nil, &jobs); err != nil {
			return nil, err
		}
		if len(jobs) == 0 {
			checks = append(checks, forge.Check{Name: fmt.Sprintf("pipeline %d", p.ID), URL: p.WebURL})
			continue
		}
		for _, j := range jobs {
			ch := forge.Check{Name: j.Name, Done: true, URL: j.WebURL, Job: j.ID}
			switch j.Status {
			case "success":
				ch.Conclusion = forge.Success
			case "failed":
				switch {
				case j.AllowFailure:
					ch.Conclusion = forge.Neutral
				case j.FailureReason == "job_execution_timeout":
					ch.Conclusion = forge.TimedOut
				default:
					ch.Conclusion = forge.Failure
				}
			case "canceled":
				ch.Conclusion = forge.Cancelled
			case "skipped", "manual":
				ch.Conclusion = forge.Skipped
			default: // created, pending, running, scheduled, preparing, waiting_for_resource
				ch.Done = false
			}
			checks = append(checks, ch)
		}
	}
	return checks, nil
}

// CheckLog reads the trace of the job of ch.
func (c *Client) CheckLog(ctx context.Context, owner, repo string, ch forge.Check) (string, error) {
	if ch.Job == 0 {
		return "", nil
	}
	var raw []byte
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("%s/jobs/%d/trace", project(owner, repo), ch.Job), nil), nil, &raw); err != nil {
		return "", err
	}
	return string(raw), nil
}

// ChangeRequestHead reads the state and the head commit of merge request number.
func (c *Client) ChangeRequestHead(ctx context.Context, owner, repo string, number int) (*forge.ChangeRequestState, error) {
	var mr struct {
		State        string `json:"state"`
		SHA          string `json:"sha"`
		HasConflicts bool   `json:"has_conflicts"`
		MergeStatus  string `json:"detailed_merge_status"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("%s/merge_requests/%d", project(owner, repo), number), nil), nil, &mr); err != nil {
		return nil, err
	}
	// has_conflicts may still hold the old answer while GitLab checks the merge again.
	conflict := mr.HasConflicts && mr.MergeStatus != "checking"
	computing := mr.MergeStatus == "checking" || mr.MergeStatus == "unchecked"
	st := &forge.ChangeRequestState{State: forge.CRClosed, Head: mr.SHA, Conflict: conflict, Computing: computing}
	switch mr.State {
	case "opened":
		st.State = forge.CROpen
	case "merged":
		st.State = forge.CRMerged
	}
	return st, nil
}
