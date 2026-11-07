package server

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

// fakeAuth is the login of the inbox service: code "alice" logs alice in as session "sid-alice"; any other code is
// someone not allowed.
type fakeAuth struct {
	started   []string
	callbacks []inboxapi.AuthCallbackRequest
	loggedOut []string
}

func (f *fakeInbox) AuthStatus(ctx context.Context) (*inboxapi.AuthStatus, error) {
	if f.auth == nil {
		return &inboxapi.AuthStatus{}, nil
	}
	out := &inboxapi.AuthStatus{Enabled: true, GitLab: "https://gl.example"}
	if inboxapi.SessionOf(ctx) == "sid-alice" {
		out.User = "alice"
	}
	return out, nil
}

func (f *fakeInbox) AuthStart(_ context.Context, req inboxapi.AuthStartRequest) (*inboxapi.AuthStartResponse, error) {
	if f.auth == nil {
		return nil, &inboxapi.Error{Code: inboxapi.CodeNotFound, Message: "off"}
	}
	f.auth.started = append(f.auth.started, req.Redirect)
	return &inboxapi.AuthStartResponse{URL: "https://gl.example/oauth/authorize?state=st"}, nil
}

func (f *fakeInbox) AuthCallback(_ context.Context, req inboxapi.AuthCallbackRequest) (*inboxapi.AuthCallbackResponse, error) {
	f.auth.callbacks = append(f.auth.callbacks, req)
	if req.Code != "alice" {
		return nil, &inboxapi.Error{Code: inboxapi.CodeNotAllowed, Message: "not allowed", User: req.Code}
	}
	return &inboxapi.AuthCallbackResponse{Session: "sid-alice", User: "alice"}, nil
}

func (f *fakeInbox) Logout(ctx context.Context) error {
	f.auth.loggedOut = append(f.auth.loggedOut, inboxapi.SessionOf(ctx))
	return nil
}

func TestLoginGoesToGitLab(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	w := do(newServer(f), "GET", "/auth/login", "", nil)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://gl.example/oauth/authorize?state=st" {
		t.Fatalf("login = %d %s", w.Code, w.Header().Get("Location"))
	}
	if len(f.auth.started) != 1 || f.auth.started[0] != "http://"+listen+"/auth/callback" {
		t.Errorf("redirect = %v", f.auth.started)
	}
}

func TestLoginWithoutAuthGoesHome(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/auth/login", "", nil)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/" {
		t.Fatalf("login = %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestCallbackSetsTheSessionCookie(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	w := do(newServer(f), "GET", "/auth/callback?state=st&code=alice", "", nil)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/" {
		t.Fatalf("callback = %d %s", w.Code, w.Header().Get("Location"))
	}
	c := w.Result().Cookies()
	if len(c) != 1 || c[0].Name != CookieName || c[0].Value != "sid-alice" || !c[0].HttpOnly ||
		c[0].SameSite != http.SameSiteLaxMode || c[0].Path != "/" || c[0].Secure {
		t.Fatalf("cookies = %+v", c)
	}
	if cb := f.auth.callbacks; len(cb) != 1 || cb[0].State != "st" || cb[0].Code != "alice" || cb[0].Redirect != "http://"+listen+"/auth/callback" {
		t.Errorf("callback request = %+v", cb)
	}
}

func TestCallbackOfSomeoneNotAllowedShowsWho(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	w := do(newServer(f), "GET", "/auth/callback?state=st&code=mallory", "", nil)
	loc, _ := url.Parse(w.Header().Get("Location"))
	if w.Code != http.StatusFound || loc.Path != "/login" || loc.Query().Get("error") != "not_allowed" || loc.Query().Get("user") != "mallory" {
		t.Fatalf("callback = %d %s", w.Code, w.Header().Get("Location"))
	}
	if len(w.Result().Cookies()) != 0 {
		t.Error("a cookie was set for someone not allowed")
	}
}

func TestCallbackDeniedOnGitLab(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	w := do(newServer(f), "GET", "/auth/callback?error=access_denied&state=st", "", nil)
	loc, _ := url.Parse(w.Header().Get("Location"))
	if loc.Path != "/login" || loc.Query().Get("error") != "denied" || len(f.auth.callbacks) != 0 {
		t.Fatalf("callback = %d %s, callbacks %v", w.Code, w.Header().Get("Location"), f.auth.callbacks)
	}
}

func TestTheCookieGoesToTheInboxService(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	do(newServer(f), "GET", "/api/inbox", "", map[string]string{"Cookie": CookieName + "=sid-alice"})
	do(newServer(f), "GET", "/api/inbox", "", nil)
	if len(f.sessions) != 2 || f.sessions[0] != "sid-alice" || f.sessions[1] != "" {
		t.Fatalf("sessions = %q", f.sessions)
	}
}

func TestAuthStatusIsPassedOn(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	w := do(newServer(f), "GET", "/api/auth", "", map[string]string{"Cookie": CookieName + "=sid-alice"})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"user":"alice"`) || !strings.Contains(w.Body.String(), `"enabled":true`) {
		t.Fatalf("status = %d %s", w.Code, w.Body)
	}
}

func TestLogoutEndsTheSessionAndClearsTheCookie(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	h := map[string]string{"Content-Type": "application/json", "Origin": "http://" + listen, "Cookie": CookieName + "=sid-alice"}
	w := do(newServer(f), "POST", "/api/logout", "{}", h)
	if w.Code != http.StatusNoContent || len(f.auth.loggedOut) != 1 || f.auth.loggedOut[0] != "sid-alice" {
		t.Fatalf("logout = %d, logged out %v", w.Code, f.auth.loggedOut)
	}
	c := w.Result().Cookies()
	if len(c) != 1 || c[0].Name != CookieName || c[0].MaxAge >= 0 {
		t.Fatalf("cookies = %+v, want the session cookie cleared", c)
	}
}

func TestLogoutFromElsewhereIsRefused(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	h := map[string]string{"Content-Type": "application/json", "Origin": "http://evil.example", "Cookie": CookieName + "=sid-alice"}
	if w := do(newServer(f), "POST", "/api/logout", "{}", h); w.Code != http.StatusForbidden || len(f.auth.loggedOut) != 0 {
		t.Fatalf("logout = %d, logged out %v", w.Code, f.auth.loggedOut)
	}
}

// The page someone opened before logging in is where the login brings them back (issue #171).
func TestLoginRemembersThePageToGoBackTo(t *testing.T) {
	f := &fakeInbox{auth: &fakeAuth{}}
	page := "/issue/gh%3Ao%2Fr%2312?action=merge"
	w := do(newServer(f), "GET", "/auth/login?next="+url.QueryEscape(page), "", nil)
	if w.Code != http.StatusFound {
		t.Fatalf("login = %d", w.Code)
	}
	c := w.Result().Cookies()
	if len(c) != 1 || c[0].Name != nextCookie || c[0].Value != url.QueryEscape(page) || !c[0].HttpOnly ||
		c[0].SameSite != http.SameSiteLaxMode || c[0].Path != "/auth/" || c[0].MaxAge <= 0 {
		t.Fatalf("cookies = %+v", c)
	}
	back := do(newServer(f), "GET", "/auth/callback?state=st&code=alice", "", map[string]string{"Cookie": c[0].Name + "=" + c[0].Value})
	if back.Code != http.StatusFound || back.Header().Get("Location") != page {
		t.Fatalf("callback = %d %s", back.Code, back.Header().Get("Location"))
	}
	var cleared bool
	for _, k := range back.Result().Cookies() {
		if k.Name == nextCookie && k.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Errorf("the page to go back to was not cleared: %+v", back.Result().Cookies())
	}
}

// Only a path of this site is gone back to: anything else would make the login an open redirect.
func TestLoginGoesBackOnlyToThisSite(t *testing.T) {
	for _, next := range []string{"https://evil.example/", "//evil.example/", "/\\evil.example", "issue/x", "", "/" + strings.Repeat("a", 2048)} {
		f := &fakeInbox{auth: &fakeAuth{}}
		w := do(newServer(f), "GET", "/auth/login?next="+url.QueryEscape(next), "", nil)
		if c := w.Result().Cookies(); len(c) != 0 {
			t.Errorf("next %q: cookies = %+v", next, c)
		}
		back := do(newServer(f), "GET", "/auth/callback?state=st&code=alice", "", map[string]string{"Cookie": nextCookie + "=" + url.QueryEscape(next)})
		if back.Header().Get("Location") != "/" {
			t.Errorf("next %q: callback goes to %s", next, back.Header().Get("Location"))
		}
	}
}

func TestUnauthenticatedIs401(t *testing.T) {
	f := &fakeInbox{err: &inboxapi.Error{Code: inboxapi.CodeUnauthenticated, Message: "log in"}}
	if w := do(newServer(f), "GET", "/api/inbox", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
