package github

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

var png = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 16)...)

// avatarHost serves body as any image, failing the test when a request carries a token.
func avatarHost(t *testing.T, body []byte) (*httptest.Server, *[]string) {
	var queries []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("the avatar host got the token")
		}
		queries = append(queries, r.URL.RawQuery)
		w.Write(body)
	}))
	t.Cleanup(ts.Close)
	return ts, &queries
}

func TestProjectAvatarIsTheOwnersWithoutTheToken(t *testing.T) {
	s, c := newServer(t)
	img, queries := avatarHost(t, png)
	s.on("GET /users/o", jsonReply(map[string]any{"login": "o", "id": 1, "avatar_url": img.URL + "/u/1?v=4"}))
	got, err := c.ProjectAvatar(ctx, "o", "r")
	if err != nil || !bytes.Equal(got, png) {
		t.Fatalf("ProjectAvatar = %d bytes, %v", len(got), err)
	}
	if len(*queries) != 1 || (*queries)[0] != "s=64&v=4" {
		t.Fatalf("queries = %q", *queries)
	}
}

func TestProjectAvatarRefusesSVG(t *testing.T) {
	s, c := newServer(t)
	img, _ := avatarHost(t, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	s.on("GET /users/o", jsonReply(map[string]any{"avatar_url": img.URL + "/u/1"}))
	if _, err := c.ProjectAvatar(ctx, "o", "r"); !errors.Is(err, forge.ErrBadImage) {
		t.Fatalf("err = %v, want ErrBadImage", err)
	}
}

func TestProjectAvatarOfAnOwnerWithout(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /users/o", jsonReply(map[string]any{"login": "o"}))
	if _, err := c.ProjectAvatar(ctx, "o", "r"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
