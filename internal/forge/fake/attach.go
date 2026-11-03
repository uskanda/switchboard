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
