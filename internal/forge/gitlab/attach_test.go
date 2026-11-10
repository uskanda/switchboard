package gitlab

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestAttachCommentUploadsEachFileThenPostsOneNote(t *testing.T) {
	s, c := newServer(t)
	n := 0
	s.on("POST /api/v4/projects/nas%2Fsb/uploads", func(w http.ResponseWriter, r *http.Request) {
		n++
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		name := []string{"a.png", "b.webp"}[n-1]
		jsonReply(map[string]any{"alt": name, "url": "/uploads/x/" + name, "markdown": "![" + name + "](/uploads/x/" + name + ")"})(w, r)
	})
	s.on("POST /api/v4/projects/nas%2Fsb/issues/3/notes", jsonReply(map[string]any{"id": 91, "body": "b", "author": map[string]any{"id": 2, "username": "agent"}}))
	dir := t.TempDir()
	var files []string
	for _, name := range []string{"a.png", "b.webp"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("img-"+name), 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	ref := forge.IssueRef{Forge: forge.GitLab, Owner: "nas", Repo: "sb", Number: 3}
	got, err := c.AttachComment(ctx, ref, "### Screenshots\n\n<!-- switchboard v=1 state=attachment -->\n", files)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 91 || n != 2 {
		t.Fatalf("comment = %+v, uploads = %d", got, n)
	}
	s.mu.Lock()
	upload := s.bodies[0]
	s.mu.Unlock()
	if !strings.Contains(upload, `filename="a.png"`) || !strings.Contains(upload, "img-a.png") {
		t.Errorf("upload body = %q", upload)
	}
	want := "### Screenshots\n\n![a.png](/uploads/x/a.png)\n![b.webp](/uploads/x/b.webp)\n\n<!-- switchboard v=1 state=attachment -->\n"
	if body := s.body(2)["body"]; body != want {
		t.Fatalf("note = %q, want %q", body, want)
	}
}

func TestAttachCommentStopsAtAFailedUpload(t *testing.T) {
	s, c := newServer(t)
	p := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(p, []byte("img"), 0o600); err != nil {
		t.Fatal(err)
	}
	ref := forge.IssueRef{Forge: forge.GitLab, Owner: "nas", Repo: "sb", Number: 3}
	if _, err := c.AttachComment(ctx, ref, "x\n<!-- m -->", []string{p}); err == nil {
		t.Fatal("a failed upload did not fail")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.log) != 1 {
		t.Fatalf("requests after a failed upload = %d, want only the upload", len(s.log))
	}
}

func TestUploadReturnsTheMarkdown(t *testing.T) {
	s, c := newServer(t)
	s.on("POST /api/v4/projects/nas%2Fsb/uploads", jsonReply(map[string]any{"markdown": "![q.png](/uploads/abc/q.png)"}))
	dir := t.TempDir()
	p := filepath.Join(dir, "q.png")
	if err := os.WriteFile(p, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	md, err := c.Upload(ctx, forge.IssueRef{Forge: forge.GitLab, Owner: "nas", Repo: "sb", Number: 3}, p)
	if err != nil || md != "![q.png](/uploads/abc/q.png)" {
		t.Fatalf("Upload = %q, %v", md, err)
	}
}
