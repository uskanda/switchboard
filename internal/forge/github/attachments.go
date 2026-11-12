package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

// WithWebURL sets the address of GitHub's web pages, where attachments are; https://github.com by default.
func WithWebURL(base string) Option {
	return func(c *Client) { c.webURL = strings.TrimRight(base, "/") }
}

func (c *Client) web() string {
	if c.webURL == "" {
		return "https://github.com"
	}
	return c.webURL
}

// isAttachmentHost says whether host is where GitHub keeps the files of attachments: the S3 buckets of user assets,
// and the host of a private repository's images.
func isAttachmentHost(host string) bool {
	if host == "private-user-images.githubusercontent.com" {
		return true
	}
	bucket, ok := strings.CutSuffix(host, ".s3.amazonaws.com")
	return ok && strings.HasPrefix(bucket, "github-production-user-asset-") && !strings.Contains(bucket, ".")
}

// ReadAttachment implements forge.AttachmentReader (ADR-071): it reads github.com/user-attachments/assets/<id> with the
// client's token, and follows the redirect only to where GitHub keeps the file, without the token.
func (c *Client) ReadAttachment(ctx context.Context, id string) ([]byte, error) {
	allowed := c.attachmentHost
	if allowed == nil {
		allowed = isAttachmentHost
	}
	start := c.web() + "/user-attachments/assets/" + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, start, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "switchboard")
	req.Header.Set("Authorization", "Bearer "+c.token)
	hc := *c.http
	hc.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		// Only https leads to GitHub's store; the stores of tests are plain http.
		if len(via) > 1 || !allowed(next.URL.Host) || c.attachmentHost == nil && next.URL.Scheme != "https" {
			return fmt.Errorf("attachment %s: a redirect to %s", id, next.URL.Host)
		}
		next.Header.Del("Authorization")
		return nil
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("attachment %s: %w", id, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, forge.ErrNotFound
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, fmt.Errorf("attachment %s: %s", id, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, forge.MaxQuestionImageBytes+1))
	if err != nil {
		return nil, err
	}
	if _, err := forge.ImageTypeUpTo(data, forge.MaxQuestionImageBytes); err != nil {
		return nil, errors.New("attachment " + id + ": not a PNG, JPEG, GIF or WebP image of at most 5 MB")
	}
	return data, nil
}
