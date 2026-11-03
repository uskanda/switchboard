package gitlab

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Avatars = (*Client)(nil)

// ProjectAvatar implements forge.Avatars: the project's own avatar, else the avatar of the group (or user) it is in.
// The token goes to the instance only; an avatar elsewhere, such as Gravatar's, gets none.
func (c *Client) ProjectAvatar(ctx context.Context, owner, repo string) ([]byte, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	auth := http.Header{"PRIVATE-TOKEN": {c.token}}
	data, err := forge.GetImage(ctx, c.http, c.url(project(owner, repo)+"/avatar", nil), base.Host, auth)
	if !errors.Is(err, forge.ErrNotFound) {
		return data, err
	}
	p, err := c.project(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	if p.Namespace.AvatarURL == "" {
		return nil, forge.ErrNotFound
	}
	// Older instances give the path of the upload, without the host.
	u, err := base.Parse(p.Namespace.AvatarURL)
	if err != nil {
		return nil, err
	}
	return forge.GetImage(ctx, c.http, u.String(), base.Host, auth)
}
