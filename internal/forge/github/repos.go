package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Repos = (*Client)(nil)

// DefaultBranch implements forge.Repos.
func (c *Client) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	_, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s", owner, repo), nil), nil, &out, nil)
	return out.DefaultBranch, err
}

// LabelNames implements forge.Repos.
func (c *Client) LabelNames(ctx context.Context, owner, repo string) ([]string, error) {
	var out []string
	err := pages(ctx, c, c.url(fmt.Sprintf("/repos/%s/%s/labels", owner, repo), url.Values{"per_page": {"100"}}), func(p []ghLabel) {
		for _, l := range p {
			out = append(out, l.Name)
		}
	})
	return out, err
}
