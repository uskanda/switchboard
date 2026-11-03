package inboxapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestTheSessionOfTheContextGoesInAHeader(t *testing.T) {
	var got []string
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get(SessionHeader))
		reply(w, 200, Inbox{})
	})
	if _, err := c.List(WithSession(ctx, "s3cr3t")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.List(ctx); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "s3cr3t" || got[1] != "" {
		t.Fatalf("session headers = %q, want [s3cr3t \"\"]", got)
	}
}

func TestSessionOf(t *testing.T) {
	if s := SessionOf(WithSession(ctx, "abc")); s != "abc" {
		t.Errorf("SessionOf = %q, want abc", s)
	}
	if s := SessionOf(ctx); s != "" {
		t.Errorf("SessionOf(no session) = %q, want empty", s)
	}
}

func TestAuthCalls(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/auth/status":
			reply(w, 200, AuthStatus{Enabled: true, User: "alice", GitLab: "https://gl.example"})
		case "POST /v1/auth/start":
			var req AuthStartRequest
			json.NewDecoder(r.Body).Decode(&req)
			reply(w, 200, AuthStartResponse{URL: "https://gl.example/oauth/authorize?redirect=" + req.Redirect})
		case "POST /v1/auth/callback":
			var req AuthCallbackRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.Code == "bad" {
				reply(w, 403, Error{Code: CodeNotAllowed, Message: "mallory may not", User: "mallory"})
				return
			}
			reply(w, 200, AuthCallbackResponse{Session: "sid", User: "alice"})
		case "POST /v1/auth/logout":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("%s %s", r.Method, r.URL)
		}
	})
	st, err := c.AuthStatus(ctx)
	if err != nil || !st.Enabled || st.User != "alice" || st.GitLab != "https://gl.example" {
		t.Fatalf("status = %+v, %v", st, err)
	}
	start, err := c.AuthStart(ctx, AuthStartRequest{Redirect: "http://w/auth/callback"})
	if err != nil || start.URL != "https://gl.example/oauth/authorize?redirect=http://w/auth/callback" {
		t.Fatalf("start = %+v, %v", start, err)
	}
	cb, err := c.AuthCallback(ctx, AuthCallbackRequest{State: "st", Code: "ok", Redirect: "http://w/auth/callback"})
	if err != nil || cb.Session != "sid" || cb.User != "alice" {
		t.Fatalf("callback = %+v, %v", cb, err)
	}
	_, err = c.AuthCallback(ctx, AuthCallbackRequest{State: "st", Code: "bad"})
	var e *Error
	if !errors.Is(err, ErrNotAllowed) || !errors.As(err, &e) || e.User != "mallory" {
		t.Fatalf("err = %v, want ErrNotAllowed for mallory", err)
	}
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestUnauthenticatedIsAnError(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, r *http.Request) {
		reply(w, 401, Error{Code: CodeUnauthenticated, Message: "log in"})
	})
	if _, err := c.List(ctx); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}
