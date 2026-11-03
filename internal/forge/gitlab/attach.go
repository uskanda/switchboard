package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

// AttachComment implements forge.Attacher: it uploads each file to the project (POST /projects/:id/uploads), then
// posts one note with the markdown GitLab returned for each, before the body's last line (ADR-051, V48).
func (c *Client) AttachComment(ctx context.Context, ref forge.IssueRef, body string, files []string) (*forge.Comment, error) {
	var links []string
	for _, f := range files {
		md, err := c.upload(ctx, ref, f)
		if err != nil {
			return nil, err
		}
		links = append(links, md)
	}
	head, last := forge.SplitLastLine(body)
	return c.PostComment(ctx, ref, head+"\n\n"+strings.Join(links, "\n")+"\n\n"+last+"\n")
}

// upload sends one file as the multipart field "file" and returns its markdown.
func (c *Client) upload(ctx context.Context, ref forge.IssueRef, path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(project(ref.Owner, ref.Repo)+"/uploads", nil), &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("User-Agent", "switchboard")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gitlab: POST %s: %w", req.URL.Path, err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", &apiError{method: http.MethodPost, path: req.URL.Path, status: resp.StatusCode, message: message(out)}
	}
	var u struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(out, &u); err != nil || u.Markdown == "" {
		return "", fmt.Errorf("gitlab: POST %s: the answer has no markdown", req.URL.Path)
	}
	return u.Markdown, nil
}
