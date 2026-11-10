package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

var iconPNG = []byte("\x89PNG\r\n\x1a\n....")

func (f *fakeInbox) ProjectIcon(_ context.Context, ref string) (*inboxapi.Icon, error) {
	if f.err != nil {
		return nil, f.err
	}
	if ref != "gh:o/r" {
		return nil, &inboxapi.Error{Code: inboxapi.CodeNotFound, Message: "no icon"}
	}
	return &inboxapi.Icon{Data: iconPNG, ContentType: "image/png"}, nil
}

func (f *fakeInbox) ProjectIcons(context.Context) (inboxapi.ProjectIcons, error) {
	if f.err != nil {
		return nil, f.err
	}
	return inboxapi.ProjectIcons{"gl:nas/sb": "🚀"}, nil
}

func TestProjectIconIsPassedOn(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/api/project-icon?ref=gh%3Ao%2Fr", "", nil)
	if w.Code != http.StatusOK || w.Body.String() != string(iconPNG) {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body)
	}
	h := w.Header()
	if h.Get("Content-Type") != "image/png" || h.Get("Cache-Control") != "max-age=3600" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers = %v", h)
	}
	// The icon comes from this server, so the policy stays the one every answer has.
	if h.Get("Content-Security-Policy") != csp {
		t.Fatalf("CSP = %q, want it unchanged", h.Get("Content-Security-Policy"))
	}
}

func TestProjectIconWithoutOne(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/api/project-icon?ref=gh%3Ao%2Fnone", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
	w = do(newServer(&fakeInbox{err: inboxapi.ErrUnavailable}), "GET", "/api/project-icon?ref=gh%3Ao%2Fr", "", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable: status = %d", w.Code)
	}
}

func TestProjectIconsArePassedOn(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/api/project-icons", "", nil)
	var got map[string]string
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil || got["gl:nas/sb"] != "🚀" {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
}

func (f *fakeInbox) IssueImage(_ context.Context, issue, src string) (*inboxapi.Icon, error) {
	if f.err != nil {
		return nil, f.err
	}
	if issue != "gl:nas/sb#3" || src != "/uploads/abc/q.png" {
		return nil, &inboxapi.Error{Code: inboxapi.CodeNotFound, Message: "no image"}
	}
	return &inboxapi.Icon{Data: iconPNG, ContentType: "image/png"}, nil
}

func TestIssueImageIsPassedOn(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/api/issue-image?issue=gl%3Anas%2Fsb%233&src=%2Fuploads%2Fabc%2Fq.png", "", nil)
	if w.Code != http.StatusOK || w.Body.String() != string(iconPNG) {
		t.Fatalf("status = %d, body = %q", w.Code, w.Body)
	}
	if h := w.Header(); h.Get("Content-Type") != "image/png" || h.Get("Content-Security-Policy") != csp {
		t.Fatalf("headers = %v", h)
	}
	w = do(newServer(&fakeInbox{}), "GET", "/api/issue-image?issue=gl%3Anas%2Fsb%233&src=https%3A%2F%2Fevil.example%2Fx.png", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("another image: status = %d", w.Code)
	}
}
