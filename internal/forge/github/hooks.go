package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/uskanda/switchboard/internal/forge"
)

// CreateHook implements forge.Hooks: a JSON webhook for issues and issue_comment, signed with h.Secret.
func (c *Client) CreateHook(ctx context.Context, owner, repo string, h forge.Hook) (int64, error) {
	in := map[string]any{
		"name": "web", "active": true, "events": []string{"issues", "issue_comment"},
		"config": map[string]any{"url": h.URL, "content_type": "json", "secret": h.Secret, "insecure_ssl": "0"},
	}
	var out struct {
		ID int64 `json:"id"`
	}
	if _, err := c.do(ctx, http.MethodPost, c.url(fmt.Sprintf("/repos/%s/%s/hooks", owner, repo), nil), in, &out, nil); err != nil {
		return 0, err
	}
	return out.ID, nil
}

// DeleteHook implements forge.Hooks.
func (c *Client) DeleteHook(ctx context.Context, owner, repo, hookURL string) (bool, error) {
	var hooks []struct {
		ID     int64 `json:"id"`
		Config struct {
			URL string `json:"url"`
		} `json:"config"`
	}
	path := fmt.Sprintf("/repos/%s/%s/hooks", owner, repo)
	if _, err := c.do(ctx, http.MethodGet, c.url(path, url.Values{"per_page": {"100"}}), nil, &hooks, nil); err != nil {
		return false, err
	}
	for _, h := range hooks {
		if h.Config.URL == hookURL {
			_, err := c.do(ctx, http.MethodDelete, c.url(fmt.Sprintf("%s/%d", path, h.ID), nil), nil, nil, nil)
			return err == nil, err
		}
	}
	return false, nil
}
