package inboxserve

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// fakeUploads is a forge.UploadReader that records what it was asked for.
type fakeUploads struct {
	asked []string
	img   []byte
}

func (u *fakeUploads) ReadUpload(_ context.Context, owner, repo, secret, name string) ([]byte, error) {
	u.asked = append(u.asked, owner+"/"+repo+" "+secret+" "+name)
	if u.img == nil {
		return nil, forge.ErrNotFound
	}
	return u.img, nil
}

func withIssueImages(t *testing.T) (*Server, *fakeUploads) {
	t.Helper()
	u := &fakeUploads{img: pngImage}
	cfg := &config.Config{Projects: []config.Project{
		{Ref: "gh:o/r", Forge: "gh", Owner: "o", Repo: "r"},
		{Ref: "gl:nas/sb", Forge: "gl", Owner: "nas", Repo: "sb"},
	}}
	cfg.Forges.GitLab.BaseURL = "https://gitlab.example.com"
	return &Server{IssueImages: &IssueImages{Config: cfg, Uploads: map[string]forge.UploadReader{forge.GitLab: u},
		Dir: filepath.Join(t.TempDir(), "issue-images")}}, u
}

func getIssueImage(s *Server, issue, src string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	q := url.Values{"issue": {issue}, "src": {src}}
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/issue-image?"+q.Encode(), nil))
	return rec
}

func TestIssueImageOfTheProjectsUpload(t *testing.T) {
	s, u := withIssueImages(t)
	for _, src := range []string{
		"/uploads/0123456789abcdef0123456789abcdef/shot.png",
		"https://gitlab.example.com/nas/sb/uploads/0123456789abcdef0123456789abcdef/shot.png",
	} {
		rec := getIssueImage(s, "gl:nas/sb#12", src)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pngImage) || rec.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("%s: GET = %d %q %q", src, rec.Code, rec.Header().Get("Content-Type"), rec.Body)
		}
	}
	if len(u.asked) != 1 || u.asked[0] != "nas/sb 0123456789abcdef0123456789abcdef shot.png" {
		t.Fatalf("asked %v, want one read, then the kept copy", u.asked)
	}
}

func TestIssueImageRefusesOtherPlaces(t *testing.T) {
	s, u := withIssueImages(t)
	for _, tc := range []struct{ issue, src string }{
		{"gl:nas/sb#12", "https://evil.example.net/x.png"},
		{"gl:nas/sb#12", "https://evil.example.net/nas/sb/uploads/0123456789abcdef0123456789abcdef/x.png"},
		{"gl:nas/sb#12", "https://gitlab.example.com/other/proj/uploads/0123456789abcdef0123456789abcdef/x.png"},
		{"gl:nas/sb#12", "/uploads/0123456789abcdef0123456789abcdef/../x.png"},
		{"gl:nas/sb#12", "/uploads/0123456789abcdef0123456789abcdef/a/b.png"},
		{"gl:nas/sb#12", "/uploads/not-hex/x.png"},
		{"gl:nas/sb#12", "//evil.example.net/uploads/0123456789abcdef0123456789abcdef/x.png"},
		{"gh:o/r#1", "/uploads/0123456789abcdef0123456789abcdef/x.png"},
		{"gl:no/such#1", "/uploads/0123456789abcdef0123456789abcdef/x.png"},
	} {
		if rec := getIssueImage(s, tc.issue, tc.src); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: GET = %d, want 404", tc.issue, tc.src, rec.Code)
		}
	}
	if len(u.asked) != 0 {
		t.Fatalf("the forge was asked for %v", u.asked)
	}
}

func TestIssueImageThatIsNotAnImage(t *testing.T) {
	s, u := withIssueImages(t)
	u.img = []byte("<svg/>")
	if rec := getIssueImage(s, "gl:nas/sb#12", "/uploads/0123456789abcdef0123456789abcdef/x.svg"); rec.Code != http.StatusNotFound {
		t.Fatalf("GET = %d, want 404", rec.Code)
	}
}
