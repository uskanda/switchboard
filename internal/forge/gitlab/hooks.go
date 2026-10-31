package gitlab

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/uskanda/switchboard/internal/forge"
)

// CreateHook implements forge.Hooks: issue and note events, confidential ones included, signed with the signing
// token h.Secret (V16), or carrying it as X-Gitlab-Token when h.SecretToken is set.
func (c *Client) CreateHook(ctx context.Context, owner, repo string, h forge.Hook) (int64, error) {
	in := map[string]any{
		"url": h.URL, "enable_ssl_verification": true,
		"issues_events": true, "confidential_issues_events": true, "note_events": true, "confidential_note_events": true,
		"push_events": false, "merge_requests_events": false,
	}
	if h.SecretToken {
		in["token"] = h.Secret
	} else {
		in["signing_token"] = h.Secret
	}
	var out struct {
		ID int64 `json:"id"`
	}
	if _, err := c.do(ctx, http.MethodPost, c.url(project(owner, repo)+"/hooks", nil), in, &out); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// DeleteHook implements forge.Hooks.
func (c *Client) DeleteHook(ctx context.Context, owner, repo, hookURL string) (bool, error) {
	var hooks []struct {
		ID  int64  `json:"id"`
		URL string `json:"url"`
	}
	path := project(owner, repo) + "/hooks"
	if _, err := c.do(ctx, http.MethodGet, c.url(path, url.Values{"per_page": {"100"}}), nil, &hooks); err != nil {
		return false, err
	}
	for _, h := range hooks {
		if h.URL == hookURL {
			_, err := c.do(ctx, http.MethodDelete, c.url(path+"/"+strconv.FormatInt(h.ID, 10), nil), nil, nil)
			return err == nil, err
		}
	}
	return false, nil
}
