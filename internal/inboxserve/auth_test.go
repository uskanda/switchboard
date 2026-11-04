package inboxserve

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/webauth"
)

// provider is a GitLab that logs in whoever the code names: code "alice" is alice, in the groups of groups.
type provider struct {
	groups  map[string][]string
	revoked int
}

func (p *provider) AuthURL(redirect, state, challenge string) string {
	return "https://gl.example/oauth/authorize?" + url.Values{"state": {state}, "redirect_uri": {redirect}}.Encode()
}
func (p *provider) Exchange(ctx context.Context, code, verifier, redirect string) (webauth.Token, error) {
	return webauth.Token{Access: "at-" + code, Refresh: "rt-" + code, ExpiresIn: 1 << 62}, nil
}
func (p *provider) Refresh(ctx context.Context, refresh string) (webauth.Token, error) {
	return webauth.Token{}, errors.New("no refresh in these tests")
}
func (p *provider) UserInfo(ctx context.Context, access string) (webauth.User, error) {
	name := access[len("at-"):]
	return webauth.User{Name: name, Groups: p.groups[name]}, nil
}
func (p *provider) Revoke(ctx context.Context, token string) { p.revoked++ }

const callback = "http://192.168.0.82:8783/auth/callback"

// withAuth turns the login on for w's server and gives the provider.
func withAuth(w *world) *provider {
	p := &provider{groups: map[string][]string{"alice": {"team"}}}
	w.srv.Auth = &webauth.Auth{Provider: p, Allow: webauth.Allow{MemberGroups: []string{"team"}}}
	w.srv.GitLab = "https://gl.example"
	return p
}

// logIn runs a login for user through the API and gives the session id.
func logIn(t *testing.T, w *world, user string) (string, error) {
	t.Helper()
	start, err := w.c.AuthStart(ctx, inboxapi.AuthStartRequest{Redirect: callback})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(start.URL)
	out, err := w.c.AuthCallback(ctx, inboxapi.AuthCallbackRequest{State: u.Query().Get("state"), Code: user, Redirect: callback})
	if err != nil {
		return "", err
	}
	if out.User != user {
		t.Errorf("callback user = %q, want %q", out.User, user)
	}
	return out.Session, nil
}

func TestWithoutAuthNothingAsksForALogin(t *testing.T) {
	w := newWorld(t)
	st, err := w.c.AuthStatus(ctx)
	if err != nil || st.Enabled {
		t.Fatalf("status = %+v, %v; want disabled", st, err)
	}
	if _, err := w.c.List(ctx); err != nil {
		t.Fatalf("list without a session: %v", err)
	}
}

func TestWithAuthARequestWithoutASessionIsRefused(t *testing.T) {
	w := newWorld(t)
	withAuth(w)
	for name, call := range map[string]func(context.Context) error{
		"list":   func(c context.Context) error { _, err := w.c.List(c); return err },
		"issue":  func(c context.Context) error { _, err := w.c.Issue(c, w.ref.String()); return err },
		"reply":  func(c context.Context) error { return w.c.Reply(c, w.ref.String(), "B", false) },
		"events": func(c context.Context) error { return w.c.Events(c, func(*inboxapi.Inbox) {}) },
	} {
		if err := call(ctx); !errors.Is(err, inboxapi.ErrUnauthenticated) {
			t.Errorf("%s without a session: err = %v, want ErrUnauthenticated", name, err)
		}
		if err := call(inboxapi.WithSession(ctx, "made-up")); !errors.Is(err, inboxapi.ErrUnauthenticated) {
			t.Errorf("%s with an unknown session: err = %v, want ErrUnauthenticated", name, err)
		}
	}
}

func TestWithAuthAMemberLogsInAndReads(t *testing.T) {
	w := newWorld(t)
	withAuth(w)
	id, err := logIn(t, w, "alice")
	if err != nil {
		t.Fatal(err)
	}
	sctx := inboxapi.WithSession(ctx, id)
	if _, err := w.c.List(sctx); err != nil {
		t.Fatalf("list with alice's session: %v", err)
	}
	st, err := w.c.AuthStatus(sctx)
	if err != nil || !st.Enabled || st.User != "alice" || st.GitLab != "https://gl.example" {
		t.Fatalf("status = %+v, %v", st, err)
	}
	if st, _ := w.c.AuthStatus(ctx); st.User != "" || !st.Enabled {
		t.Fatalf("status without a session = %+v", st)
	}
}

func TestWithAuthSomeoneOutsideTheGroupsIsRefused(t *testing.T) {
	w := newWorld(t)
	p := withAuth(w)
	_, err := logIn(t, w, "mallory")
	var e *inboxapi.Error
	if !errors.Is(err, inboxapi.ErrNotAllowed) || !errors.As(err, &e) || e.User != "mallory" {
		t.Fatalf("err = %v, want ErrNotAllowed for mallory", err)
	}
	if p.revoked != 1 {
		t.Errorf("revoked %d tokens, want 1", p.revoked)
	}
}

func TestWithAuthAnUnknownStateIsRefused(t *testing.T) {
	w := newWorld(t)
	withAuth(w)
	_, err := w.c.AuthCallback(ctx, inboxapi.AuthCallbackRequest{State: "made-up", Code: "alice", Redirect: callback})
	if !errors.Is(err, inboxapi.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestLogoutEndsTheSession(t *testing.T) {
	w := newWorld(t)
	withAuth(w)
	id, err := logIn(t, w, "alice")
	if err != nil {
		t.Fatal(err)
	}
	sctx := inboxapi.WithSession(ctx, id)
	if err := w.c.Logout(sctx); err != nil {
		t.Fatal(err)
	}
	if _, err := w.c.List(sctx); !errors.Is(err, inboxapi.ErrUnauthenticated) {
		t.Fatalf("list after the logout: err = %v, want ErrUnauthenticated", err)
	}
}

func TestTheSessionIsInTheRequestContext(t *testing.T) {
	w := newWorld(t)
	withAuth(w)
	id, _ := logIn(t, w, "alice")
	var seen *webauth.Session
	w.srv.observeSession = func(s *webauth.Session) { seen = s }
	if _, err := w.c.List(inboxapi.WithSession(ctx, id)); err != nil {
		t.Fatal(err)
	}
	if seen == nil || seen.User != "alice" {
		t.Fatalf("session in the context = %+v, want alice's", seen)
	}
}
