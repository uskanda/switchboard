package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.TokenProber = (*Client)(nil)

// TokenScopes implements forge.TokenProber with GET /personal_access_tokens/self, which a personal, a group or a
// project access token may call about itself.
func (c *Client) TokenScopes(ctx context.Context) (*forge.TokenScopes, error) {
	var out struct {
		Scopes []string `json:"scopes"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.url("/personal_access_tokens/self", nil), nil, &out); err != nil {
		return nil, err
	}
	return &forge.TokenScopes{Known: true, Scopes: out.Scopes}, nil
}

// ProbeReads implements forge.TokenProber: one item each of the issues and the merge requests, and the tip of branch.
func (c *Client) ProbeReads(ctx context.Context, owner, repo, branch string) []forge.Probe {
	one := url.Values{"per_page": {strconv.Itoa(1)}}
	reads := []struct {
		what, path string
		q          url.Values
	}{
		{"issues", project(owner, repo) + "/issues", one},
		{"merge requests", project(owner, repo) + "/merge_requests", one},
		{"contents", project(owner, repo) + "/repository/branches/" + url.PathEscape(branch), nil},
	}
	var out []forge.Probe
	for _, r := range reads {
		_, err := c.do(ctx, http.MethodGet, c.url(r.path, r.q), nil, nil)
		out = append(out, forge.Probe{What: r.what, Err: err})
	}
	return out
}
