package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Issues = (*Client)(nil)

// CreateIssue implements forge.Issues.
func (c *Client) CreateIssue(ctx context.Context, owner, repo, title, body string, labels []string) (*forge.IssueSummary, error) {
	if labels == nil {
		labels = []string{}
	}
	var gi ghIssue
	if _, err := c.do(ctx, http.MethodPost, c.url(fmt.Sprintf("/repos/%s/%s/issues", owner, repo), nil),
		map[string]any{"title": title, "body": body, "labels": labels}, &gi, nil); err != nil {
		return nil, err
	}
	s := gi.summary(owner, repo)
	return &s, nil
}
