package gitlab

import (
	"context"
	"net/http"
	"net/url"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.UploadReader = (*Client)(nil)

// ReadUpload implements forge.UploadReader with GET /projects/:id/uploads/:secret/:filename (GitLab 17.4), which takes
// a token where the web address of an upload of a private project may not. The token goes to the instance only.
func (c *Client) ReadUpload(ctx context.Context, owner, repo, secret, name string) ([]byte, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	auth := http.Header{}
	c.authorize(auth)
	u := c.url(project(owner, repo)+"/uploads/"+url.PathEscape(secret)+"/"+url.PathEscape(name), nil)
	return forge.GetImageUpTo(ctx, c.http, u, base.Host, auth, forge.MaxQuestionImageBytes)
}
