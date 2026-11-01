package gitlab

import (
	"context"
	"net/http"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Issues = (*Client)(nil)

// CreateIssue implements forge.Issues. GitLab takes the labels as one comma-separated list.
func (c *Client) CreateIssue(ctx context.Context, owner, repo, title, body string, labels []string) (*forge.IssueSummary, error) {
	var gi glIssue
	if _, err := c.do(ctx, http.MethodPost, c.url(project(owner, repo)+"/issues", nil),
		map[string]any{"title": title, "description": body, "labels": strings.Join(labels, ",")}, &gi); err != nil {
		return nil, err
	}
	s := gi.summary(owner, repo)
	return &s, nil
}
