package forge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Avatars reads the image a forge shows for a project. It is kept apart from Forge: only the inbox service reads
// avatars, with the inbox token, for the web inbox (ADR-049).
type Avatars interface {
	// ProjectAvatar returns the project's avatar image, checked by ImageType; ErrNotFound when it has none.
	ProjectAvatar(ctx context.Context, owner, repo string) ([]byte, error)
}

// MaxImageBytes bounds an avatar or icon image.
const MaxImageBytes = 256 << 10

// ErrBadImage reports an image that is not PNG, JPEG, GIF or WebP, or larger than MaxImageBytes. SVG is refused: it
// can carry script.
var ErrBadImage = errors.New("not a PNG, JPEG, GIF or WebP image of at most 256 KiB")

// ImageType tells the content type of data from its first bytes: image/png, image/jpeg, image/gif or image/webp.
// Anything else, and anything larger than MaxImageBytes, is ErrBadImage.
func ImageType(data []byte) (string, error) {
	if len(data) > MaxImageBytes {
		return "", ErrBadImage
	}
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", nil
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", nil
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return "image/gif", nil
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp", nil
	}
	return "", ErrBadImage
}

// GetImage fetches the image at rawURL with hc and checks it with ImageType. auth (the forge's credential) is sent
// only while the request goes to authHost, the forge's own host: not to another host, nor to one a redirect leads to.
// A 404 is ErrNotFound.
func GetImage(ctx context.Context, hc *http.Client, rawURL, authHost string, auth http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "switchboard")
	if authHost != "" && req.URL.Host == authHost {
		for k, v := range auth {
			req.Header[http.CanonicalHeaderKey(k)] = v
		}
	}
	c := *hc
	c.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		if next.URL.Host != authHost {
			for k := range auth {
				next.Header.Del(k)
			}
		}
		return nil
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("image %s: %w", req.URL.Redacted(), err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return nil, fmt.Errorf("image %s: %s", req.URL.Redacted(), resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if _, err := ImageType(data); err != nil {
		return nil, err
	}
	return data, nil
}
