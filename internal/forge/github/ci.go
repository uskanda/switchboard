package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.CI = (*Client)(nil)

type ghJob struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
	Steps      []struct {
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
	} `json:"steps"`
}

// CommitChecks reads the jobs of the Actions runs of sha (Actions: Read) and its commit statuses (Commit statuses:
// Read). A run without jobs yet counts as one unfinished check. Check runs of other apps are not read (V35).
func (c *Client) CommitChecks(ctx context.Context, owner, repo, sha string) ([]forge.Check, error) {
	var runs struct {
		WorkflowRuns []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			Status  string `json:"status"`
			HTMLURL string `json:"html_url"`
		} `json:"workflow_runs"`
	}
	q := url.Values{"head_sha": {sha}, "per_page": {"100"}}
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/actions/runs", owner, repo), q), nil, &runs, nil); err != nil {
		return nil, err
	}
	var checks []forge.Check
	for _, run := range runs.WorkflowRuns {
		var jobs struct {
			Jobs []ghJob `json:"jobs"`
		}
		path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", owner, repo, run.ID)
		if _, err := c.do(ctx, http.MethodGet, c.url(path, url.Values{"per_page": {"100"}}), nil, &jobs, nil); err != nil {
			return nil, err
		}
		if len(jobs.Jobs) == 0 {
			checks = append(checks, forge.Check{Name: run.Name, Done: false, URL: run.HTMLURL})
			continue
		}
		for _, j := range jobs.Jobs {
			ch := forge.Check{Name: run.Name + " / " + j.Name, Done: j.Status == "completed", URL: j.HTMLURL, Job: j.ID}
			if ch.Done {
				ch.Conclusion = j.Conclusion
			}
			for _, s := range j.Steps {
				if s.Conclusion == forge.Failure || s.Conclusion == forge.TimedOut {
					ch.FailedSteps = append(ch.FailedSteps, s.Name)
				}
			}
			checks = append(checks, ch)
		}
	}
	var status struct {
		Statuses []struct {
			Context     string `json:"context"`
			State       string `json:"state"`
			Description string `json:"description"`
			TargetURL   string `json:"target_url"`
		} `json:"statuses"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/commits/%s/status", owner, repo, sha), nil), nil, &status, nil); err != nil {
		return nil, err
	}
	for _, s := range status.Statuses {
		ch := forge.Check{Name: s.Context, URL: s.TargetURL, Description: s.Description}
		switch s.State {
		case "success":
			ch.Done, ch.Conclusion = true, forge.Success
		case "failure", "error":
			ch.Done, ch.Conclusion = true, forge.Failure
		}
		checks = append(checks, ch)
	}
	return checks, nil
}

// CheckLog downloads the plain-text log of an Actions job; GitHub redirects to storage, and the client drops the
// token on that redirect.
func (c *Client) CheckLog(ctx context.Context, owner, repo string, ch forge.Check) (string, error) {
	if ch.Job == 0 {
		return "", nil
	}
	var raw []byte
	path := fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/logs", owner, repo, ch.Job)
	if _, err := c.do(ctx, http.MethodGet, c.url(path, nil), nil, &raw, nil); err != nil {
		return "", err
	}
	return string(raw), nil
}

// ChangeRequestHead reads the state and the head commit of pull request number.
func (c *Client) ChangeRequestHead(ctx context.Context, owner, repo string, number int) (*forge.ChangeRequestState, error) {
	var pr struct {
		State          string `json:"state"`
		Merged         bool   `json:"merged"`
		Mergeable      *bool  `json:"mergeable"`
		MergeableState string `json:"mergeable_state"`
		Head           struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number), nil), nil, &pr, nil); err != nil {
		return nil, err
	}
	// mergeable is null while GitHub computes it; dirty is the one mergeable_state that means a conflict.
	conflict := pr.Mergeable != nil && !*pr.Mergeable && pr.MergeableState == "dirty"
	st := &forge.ChangeRequestState{State: forge.CROpen, Head: pr.Head.SHA, Conflict: conflict}
	switch {
	case pr.Merged:
		st.State = forge.CRMerged
	case pr.State != "open":
		st.State = forge.CRClosed
	}
	return st, nil
}
