package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.TokenProber = (*Client)(nil)

// TokenScopes implements forge.TokenProber. A classic token lists its scopes in X-OAuth-Scopes; a fine-grained token
// sends no such header, and its permissions are not known.
func (c *Client) TokenScopes(ctx context.Context) (*forge.TokenScopes, error) {
	resp, err := c.do(ctx, http.MethodGet, c.url("/user", nil), nil, nil, nil)
	if err != nil {
		return nil, err
	}
	h, ok := resp.Header["X-Oauth-Scopes"]
	if !ok {
		return &forge.TokenScopes{}, nil
	}
	out := &forge.TokenScopes{Known: true}
	for _, s := range strings.Split(strings.Join(h, ","), ",") {
		if s = strings.TrimSpace(s); s != "" {
			out.Scopes = append(out.Scopes, s)
		}
	}
	return out, nil
}

// ProbeReads implements forge.TokenProber: one page of one item each of the issues and the pull requests, and the
// commit at the tip of branch (Contents: Read).
func (c *Client) ProbeReads(ctx context.Context, owner, repo, branch string) []forge.Probe {
	one := url.Values{"per_page": {"1"}}
	reads := []struct{ what, path string }{
		{"issues", fmt.Sprintf("/repos/%s/%s/issues", owner, repo)},
		{"pull requests", fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)},
		{"contents", fmt.Sprintf("/repos/%s/%s/commits/%s", owner, repo, url.PathEscape(branch))},
	}
	var out []forge.Probe
	for _, r := range reads {
		q := one
		if r.what == "contents" {
			q = nil
		}
		_, err := c.do(ctx, http.MethodGet, c.url(r.path, q), nil, nil, nil)
		out = append(out, forge.Probe{What: r.what, Err: err})
	}
	return out
}
