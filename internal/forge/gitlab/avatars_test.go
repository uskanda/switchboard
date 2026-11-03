package gitlab

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

var png = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)

func TestProjectAvatarIsTheProjectsOwn(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/projects/nas%2Fsb/avatar", func(w http.ResponseWriter, _ *http.Request) { w.Write(png) })
	got, err := c.ProjectAvatar(ctx, "nas", "sb")
	if err != nil || !bytes.Equal(got, png) {
		t.Fatalf("ProjectAvatar = %d bytes, %v", len(got), err)
	}
}

func TestProjectAvatarFallsBackToTheGroupOnTheInstance(t *testing.T) {
	s, c := newServer(t)
	// The group's avatar is on the instance, so it gets the token (newServer checks every request for it).
	s.on("GET /api/v4/projects/nas%2Fsb", jsonReply(map[string]any{"id": 7, "namespace": map[string]any{"avatar_url": "/uploads/-/system/group/avatar/3/g.png"}}))
	s.on("GET /uploads/-/system/group/avatar/3/g.png", func(w http.ResponseWriter, _ *http.Request) { w.Write(png) })
	got, err := c.ProjectAvatar(ctx, "nas", "sb")
	if err != nil || !bytes.Equal(got, png) {
		t.Fatalf("ProjectAvatar = %d bytes, %v", len(got), err)
	}
}

func TestProjectAvatarSendsNoTokenToAnotherHost(t *testing.T) {
	s, c := newServer(t)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "" {
			t.Errorf("another host got the token")
		}
		w.Write(png)
	}))
	t.Cleanup(other.Close)
	s.on("GET /api/v4/projects/me%2Fsb", jsonReply(map[string]any{"namespace": map[string]any{"avatar_url": other.URL + "/avatar/abc?s=80"}}))
	if _, err := c.ProjectAvatar(ctx, "me", "sb"); err != nil {
		t.Fatal(err)
	}
}

func TestProjectAvatarWithoutAny(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/projects/nas%2Fsb", jsonReply(map[string]any{"namespace": map[string]any{"avatar_url": nil}}))
	if _, err := c.ProjectAvatar(ctx, "nas", "sb"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestProjectAvatarRefusesSVG(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/projects/nas%2Fsb/avatar", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	})
	if _, err := c.ProjectAvatar(ctx, "nas", "sb"); !errors.Is(err, forge.ErrBadImage) {
		t.Fatalf("err = %v, want ErrBadImage", err)
	}
}
