package inboxapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

var ctx = context.Background()

// serve answers on a Unix socket in a temporary directory and returns a client for it.
func serve(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	dir, err := os.MkdirTemp("", "ia")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "inbox.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return &Client{Socket: sock}
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func TestList(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/inbox" {
			t.Errorf("%s %s", r.Method, r.URL)
		}
		reply(w, 200, Inbox{Items: []Item{{N: 1, Ref: "gh:o/r#12", Title: "Add a flag"}}, Errors: []ForgeError{{Forge: "gitlab", Message: "down"}}})
	})
	got, err := c.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].Ref != "gh:o/r#12" || len(got.Errors) != 1 || got.Errors[0].Forge != "gitlab" {
		t.Fatalf("got %+v", got)
	}
}

func TestIssueSendsTheRef(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/issue" || r.URL.Query().Get("ref") != "gl:nas/sb#6" {
			t.Errorf("%s", r.URL)
		}
		reply(w, 200, Detail{Ref: "gl:nas/sb#6", Waiting: true, Post: "q"})
	})
	d, err := c.Issue(ctx, "gl:nas/sb#6")
	if err != nil || d.Ref != "gl:nas/sb#6" || !d.Waiting || d.Post != "q" {
		t.Fatalf("Issue = %+v, %v", d, err)
	}
}

func TestReplyPostsJSON(t *testing.T) {
	var got ReplyRequest
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/reply" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("%s %s %s", r.Method, r.URL, r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.Reply(ctx, "gh:o/r#12", "Go with B.", false); err != nil {
		t.Fatal(err)
	}
	if got != (ReplyRequest{Ref: "gh:o/r#12", Text: "Go with B."}) {
		t.Fatalf("request = %+v", got)
	}
}

func TestErrorsAreTyped(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusConflict, Error{Code: CodeNotWaiting, Message: "gh:o/r#12 is not waiting"})
	})
	err := c.Reply(ctx, "gh:o/r#12", "x", false)
	var e *Error
	if !errors.Is(err, ErrNotWaiting) || !errors.As(err, &e) || e.Message != "gh:o/r#12 is not waiting" {
		t.Fatalf("err = %v", err)
	}
	if errors.Is(err, ErrUnavailable) {
		t.Fatal("an answer is not unavailability")
	}
}

func TestNoServerIsUnavailable(t *testing.T) {
	c := &Client{Socket: filepath.Join(t.TempDir(), "none.sock")}
	if _, err := c.List(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
}

func TestSocketPaths(t *testing.T) {
	t.Setenv("RUNTIME_DIRECTORY", "/run/switchboard")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if got := ServerSocket(); got != "/run/switchboard/inbox.sock" {
		t.Fatalf("with RUNTIME_DIRECTORY: %q", got)
	}
	t.Setenv("RUNTIME_DIRECTORY", "")
	if got := ServerSocket(); got != "/run/user/1000/switchboard/inbox.sock" {
		t.Fatalf("with XDG_RUNTIME_DIR: %q", got)
	}
	t.Setenv("XDG_RUNTIME_DIR", "")
	if got := ServerSocket(); got != "" {
		t.Fatalf("with neither: %q", got)
	}
}

func TestDefaultSocket(t *testing.T) {
	dir := t.TempDir()
	systemSocket = filepath.Join(dir, "system.sock")
	t.Cleanup(func() { systemSocket = SystemSocket })
	t.Setenv("RUNTIME_DIRECTORY", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if got := DefaultSocket(); got != "/run/user/1000/switchboard/inbox.sock" {
		t.Fatalf("with XDG_RUNTIME_DIR: %q", got)
	}
	// A system service started without XDG_RUNTIME_DIR, before the inbox service made its socket.
	t.Setenv("XDG_RUNTIME_DIR", "")
	if got := DefaultSocket(); got != systemSocket {
		t.Fatalf("with neither: %q", got)
	}
	os.WriteFile(systemSocket, nil, 0o600)
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	if got := DefaultSocket(); got != systemSocket {
		t.Fatalf("with the system socket: %q", got)
	}
}

// A client with no socket looks for it on each request, so it reaches an inbox service that started after it.
func TestClientFindsALaterSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "ia")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	systemSocket = filepath.Join(dir, "inbox.sock")
	t.Cleanup(func() { systemSocket = SystemSocket })
	t.Setenv("RUNTIME_DIRECTORY", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	c := &Client{}
	if _, err := c.List(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("before the service: err = %v", err)
	}
	ln, err := net.Listen("unix", systemSocket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply(w, 200, Inbox{Items: []Item{{Ref: "gh:o/r#1"}}})
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	in, err := c.List(ctx)
	if err != nil || len(in.Items) != 1 {
		t.Fatalf("after the service: %+v, %v", in, err)
	}
}
