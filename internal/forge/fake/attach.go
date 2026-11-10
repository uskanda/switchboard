package fake

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
)

// AttachComment implements forge.Attacher: it reads each file, then posts body with a link to each file before its
// last line, as PostComment does.
func (f *Forge) AttachComment(ctx context.Context, ref forge.IssueRef, body string, files []string) (*forge.Comment, error) {
	f.mu.Lock()
	failure := f.FailAttach
	f.mu.Unlock()
	if failure != nil {
		return nil, failure
	}
	var names, links []string
	for _, p := range files {
		if _, err := os.ReadFile(p); err != nil {
			return nil, fmt.Errorf("fake: attach: %w", err)
		}
		name := filepath.Base(p)
		names = append(names, name)
		links = append(links, fmt.Sprintf("![%s](https://fake.test/attachments/%s)", name, name))
	}
	head, last := forge.SplitLastLine(body)
	c, err := f.PostComment(ctx, ref, head+"\n\n"+strings.Join(links, "\n")+"\n\n"+last+"\n")
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.Attached = append(f.Attached, names)
	f.mu.Unlock()
	return c, nil
}

// Upload implements forge.Uploader as GitLab does: it reads the file and returns Markdown with a project-relative
// /uploads/ link to it.
func (f *Forge) Upload(ctx context.Context, ref forge.IssueRef, path string) (string, error) {
	f.mu.Lock()
	failure := f.FailUpload
	f.mu.Unlock()
	if failure != nil {
		return "", failure
	}
	if _, err := os.ReadFile(path); err != nil {
		return "", fmt.Errorf("fake: upload: %w", err)
	}
	name := filepath.Base(path)
	f.mu.Lock()
	f.Uploaded = append(f.Uploaded, name)
	n := len(f.Uploaded)
	f.mu.Unlock()
	return fmt.Sprintf("![%s](/uploads/%032x/%s)", name, n, name), nil
}
