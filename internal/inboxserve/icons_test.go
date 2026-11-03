package inboxserve

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

var (
	pngImage = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)
	gifImage = append([]byte("GIF89a"), make([]byte, 16)...)
)

// countingAvatars is a forge.Avatars that counts its calls and answers with img, or err.
type countingAvatars struct {
	calls int
	img   []byte
	err   error
}

func (a *countingAvatars) ProjectAvatar(_ context.Context, owner, repo string) ([]byte, error) {
	a.calls++
	if a.err != nil {
		return nil, a.err
	}
	return a.img, nil
}

type iconsWorld struct {
	srv *Server
	gh  *countingAvatars
	gl  *countingAvatars
	now time.Time
	dir string
}

func withIcons(t *testing.T, projects ...config.Project) *iconsWorld {
	t.Helper()
	w := &iconsWorld{gh: &countingAvatars{img: pngImage}, gl: &countingAvatars{err: forge.ErrNotFound},
		now: time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC), dir: filepath.Join(t.TempDir(), "project-icons")}
	if len(projects) == 0 {
		projects = []config.Project{
			{Ref: "gh:o/r", Forge: "gh", Owner: "o", Repo: "r"},
			{Ref: "gl:nas/sb", Forge: "gl", Owner: "nas", Repo: "sb"},
		}
	}
	w.srv = &Server{Now: func() time.Time { return w.now },
		Icons: &Icons{Config: &config.Config{Projects: projects}, Avatars: map[string]forge.Avatars{forge.GitHub: w.gh, forge.GitLab: w.gl}, Dir: w.dir}}
	return w
}

func (w *iconsWorld) get(ref string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	w.srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/project-icon?ref="+url.QueryEscape(ref), nil))
	return rec
}

func TestProjectIconIsTheForgesAvatar(t *testing.T) {
	w := withIcons(t)
	rec := w.get("gh:o/r")
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pngImage) {
		t.Fatalf("GET = %d %q", rec.Code, rec.Body)
	}
	if ct, cc := rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"); ct != "image/png" || cc != "max-age=3600" {
		t.Fatalf("Content-Type %q, Cache-Control %q", ct, cc)
	}
}

func TestProjectIconIsKeptForADay(t *testing.T) {
	w := withIcons(t)
	w.get("gh:o/r")
	w.now = w.now.Add(23 * time.Hour)
	if rec := w.get("gh:o/r"); rec.Code != http.StatusOK || w.gh.calls != 1 {
		t.Fatalf("within the day: %d, %d calls", rec.Code, w.gh.calls)
	}
	// A restarted service finds the image on disk.
	w2 := withIcons(t)
	w2.dir, w2.srv.Icons.Dir, w2.now = w.dir, w.dir, w.now
	if rec := w2.get("gh:o/r"); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pngImage) || w2.gh.calls != 0 {
		t.Fatalf("after a restart: %d, %d calls", rec.Code, w2.gh.calls)
	}
	w.now = w.now.Add(2 * time.Hour)
	w.get("gh:o/r")
	if w.gh.calls != 2 {
		t.Fatalf("after the day: %d calls, want 2", w.gh.calls)
	}
}

func TestAFailureIsRememberedForAnHour(t *testing.T) {
	w := withIcons(t)
	if rec := w.get("gl:nas/sb"); rec.Code != http.StatusNotFound || code(t, rec) != inboxapi.CodeNotFound {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	w.now = w.now.Add(50 * time.Minute)
	w.get("gl:nas/sb")
	if w.gl.calls != 1 {
		t.Fatalf("within the hour: %d calls", w.gl.calls)
	}
	w.now = w.now.Add(20 * time.Minute)
	w.get("gl:nas/sb")
	if w.gl.calls != 2 {
		t.Fatalf("after the hour: %d calls, want 2", w.gl.calls)
	}
}

func TestABadAvatarIsNotServed(t *testing.T) {
	w := withIcons(t)
	w.gh.img = []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	if rec := w.get("gh:o/r"); rec.Code != http.StatusNotFound {
		t.Fatalf("GET = %d", rec.Code)
	}
}

func TestTheConfiguredIconComesFirst(t *testing.T) {
	img := httptest.NewTLSServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("PRIVATE-TOKEN") != "" {
			t.Errorf("the icon's host got a token")
		}
		rw.Write(gifImage)
	}))
	t.Cleanup(img.Close)
	file := filepath.Join(t.TempDir(), "sb.png")
	if err := os.WriteFile(file, pngImage, 0o600); err != nil {
		t.Fatal(err)
	}
	w := withIcons(t,
		config.Project{Ref: "gh:o/r", Forge: "gh", Owner: "o", Repo: "r", Icon: img.URL + "/a.gif"},
		config.Project{Ref: "gl:nas/sb", Forge: "gl", Owner: "nas", Repo: "sb", Icon: file},
		config.Project{Ref: "gh:o/e", Forge: "gh", Owner: "o", Repo: "e", Icon: "🚀"},
	)
	w.srv.Icons.HTTP = img.Client()
	if rec := w.get("gh:o/r"); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/gif" {
		t.Fatalf("url: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := w.get("gl:nas/sb"); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), pngImage) {
		t.Fatalf("file: %d", rec.Code)
	}
	if rec := w.get("gh:o/e"); rec.Code != http.StatusNotFound {
		t.Fatalf("emoji: %d", rec.Code)
	}
	if w.gh.calls != 0 || w.gl.calls != 0 {
		t.Fatalf("the forges were asked: %d, %d", w.gh.calls, w.gl.calls)
	}
}

func TestProjectIconOfAnUnknownProject(t *testing.T) {
	w := withIcons(t)
	if rec := w.get("gh:o/nope"); rec.Code != http.StatusNotFound || w.gh.calls != 0 {
		t.Fatalf("GET = %d, %d calls", rec.Code, w.gh.calls)
	}
}

func TestProjectIconsListsTheEmoji(t *testing.T) {
	w := withIcons(t,
		config.Project{Ref: "gh:o/r", Forge: "gh", Owner: "o", Repo: "r", Icon: "🚀"},
		config.Project{Ref: "gl:nas/sb", Forge: "gl", Owner: "nas", Repo: "sb"},
		config.Project{Ref: "gh:o/f", Forge: "gh", Owner: "o", Repo: "f", Icon: "/srv/f.png"},
	)
	rec := httptest.NewRecorder()
	w.srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/project-icons", nil))
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	if len(got) != 1 || got["gh:o/r"] != "🚀" {
		t.Fatalf("emoji = %v", got)
	}
}

func TestIconsFollowAReload(t *testing.T) {
	w := withIcons(t)
	w.srv.Reload(&config.Config{Projects: []config.Project{{Ref: "gh:o/r", Forge: "gh", Owner: "o", Repo: "r", Icon: "🚀"}}}, nil)
	if rec := w.get("gh:o/r"); rec.Code != http.StatusNotFound || w.gh.calls != 0 {
		t.Fatalf("GET = %d, %d calls", rec.Code, w.gh.calls)
	}
}

func TestIconsAreServedOnlyWithIcons(t *testing.T) {
	w := withIcons(t)
	w.srv.Icons = nil
	if rec := w.get("gh:o/r"); rec.Code != http.StatusNotFound || code(t, rec) != inboxapi.CodeNotFound {
		t.Fatalf("GET = %d", rec.Code)
	}
}
