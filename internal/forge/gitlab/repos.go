package gitlab

import (
	"context"
	"net/url"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Repos = (*Client)(nil)

// DefaultBranch implements forge.Repos.
func (c *Client) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	p, err := c.project(ctx, owner, repo)
	if err != nil {
		return "", err
	}
	return p.DefaultBranch, nil
}

// LabelNames implements forge.Repos.
func (c *Client) LabelNames(ctx context.Context, owner, repo string) ([]string, error) {
	var out []string
	_, err := pages(ctx, c, c.url(project(owner, repo)+"/labels", url.Values{"per_page": {"100"}}), 0, func(p []struct {
		Name string `json:"name"`
	}) {
		for _, l := range p {
			out = append(out, l.Name)
		}
	})
	return out, err
}
