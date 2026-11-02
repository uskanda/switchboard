package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Merges = (*Client)(nil)

// draftPrefix is what marks a merge request as a draft at the start of its title.
var draftPrefix = regexp.MustCompile(`(?i)^\s*(draft:|\[draft\]|\(draft\))\s*`)

// undrafted is title without the draft marker at its start.
func undrafted(title string) string { return draftPrefix.ReplaceAllString(title, "") }

// MergeChangeRequest implements forge.Merges. GitLab marks a draft by its title, so the marker is taken off the title
// before the merge. The head is checked first, and the merge carries sha too (V39).
func (c *Client) MergeChangeRequest(ctx context.Context, owner, repo string, number int, sha string) error {
	var m struct {
		State string `json:"state"` // "opened", "closed", "locked" or "merged"
		Draft bool   `json:"draft"`
		Title string `json:"title"`
		SHA   string `json:"sha"`
	}
	path := fmt.Sprintf("%s/merge_requests/%d", project(owner, repo), number)
	if _, err := c.do(ctx, http.MethodGet, c.url(path, nil), nil, &m); err != nil {
		return err
	}
	switch {
	case m.State == "merged":
		return &forge.NotMergeableError{Reason: fmt.Sprintf("merge request !%d is already merged", number)}
	case m.State != "opened":
		return &forge.NotMergeableError{Reason: fmt.Sprintf("merge request !%d is %s", number, m.State)}
	case m.SHA != sha:
		return fmt.Errorf("gitlab: merge request !%d: %w", number, forge.ErrHeadMoved)
	}
	if m.Draft {
		if _, err := c.do(ctx, http.MethodPut, c.url(path, nil), map[string]any{"title": undrafted(m.Title)}, nil); err != nil {
			return err
		}
	}
	_, err := c.do(ctx, http.MethodPut, c.url(path+"/merge", nil), map[string]any{"sha": sha}, nil)
	var ae *apiError
	if errors.As(err, &ae) {
		switch ae.status {
		case http.StatusConflict:
			return fmt.Errorf("%w: %w", forge.ErrHeadMoved, err)
		case http.StatusMethodNotAllowed, http.StatusNotAcceptable, http.StatusUnprocessableEntity:
			return &forge.NotMergeableError{Reason: ae.message}
		}
	}
	return err
}
