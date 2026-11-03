package github

import (
	"context"
	"net/http"
	"net/url"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Avatars = (*Client)(nil)

// ProjectAvatar implements forge.Avatars. A GitHub repository has no image of its own, so it is the owner's avatar,
// read through /users/{owner}, which also serves organizations. The token goes to the API host only; the avatar host
// gets none.
func (c *Client) ProjectAvatar(ctx context.Context, owner, _ string) ([]byte, error) {
	var u struct {
		AvatarURL string `json:"avatar_url"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.url("/users/"+url.PathEscape(owner), nil), nil, &u, nil); err != nil {
		return nil, err
	}
	if u.AvatarURL == "" {
		return nil, forge.ErrNotFound
	}
	a, err := url.Parse(u.AvatarURL)
	if err != nil {
		return nil, err
	}
	q := a.Query()
	q.Set("s", "64")
	a.RawQuery = q.Encode()
	api, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	return forge.GetImage(ctx, c.http, a.String(), api.Host, http.Header{"Authorization": {"Bearer " + c.token}})
}
