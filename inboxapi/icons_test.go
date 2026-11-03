package inboxapi

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
)

func TestProjectIconReadsTheImage(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n....")
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/project-icon" || r.URL.Query().Get("ref") != "gl:nas/sb" {
			t.Errorf("%s", r.URL)
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	})
	got, err := c.ProjectIcon(ctx, "gl:nas/sb")
	if err != nil || got.ContentType != "image/png" || !bytes.Equal(got.Data, png) {
		t.Fatalf("ProjectIcon = %+v, %v", got, err)
	}
}

func TestProjectIconWithoutOne(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, 404, Error{Code: CodeNotFound, Message: "no icon"})
	})
	if _, err := c.ProjectIcon(ctx, "gh:o/r"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestProjectIcons(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/project-icons" {
			t.Errorf("%s", r.URL)
		}
		reply(w, 200, ProjectIcons{"gh:o/r": "🚀"})
	})
	got, err := c.ProjectIcons(ctx)
	if err != nil || len(got) != 1 || got["gh:o/r"] != "🚀" {
		t.Fatalf("ProjectIcons = %v, %v", got, err)
	}
}
