package webauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeGitLab is the OAuth side of a GitLab: it hands out a code for a user, swaps it for tokens, refreshes them,
// answers the userinfo of a token and revokes tokens.
type fakeGitLab struct {
	t  *testing.T
	mu sync.Mutex
	// users maps a code to the user it logs in.
	codes      map[string]string
	challenges map[string]string // code -> code_challenge the authorize request carried
	groups     map[string][]string
	tokens     map[string]string // access token -> user
	refreshes  map[string]string // refresh token -> user
	revoked    []string
	n          int
	expiresIn  int
}

func newFakeGitLab(t *testing.T) (*fakeGitLab, *httptest.Server) {
	f := &fakeGitLab{t: t, codes: map[string]string{}, challenges: map[string]string{}, groups: map[string][]string{},
		tokens: map[string]string{}, refreshes: map[string]string{}, expiresIn: 7200}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

// authorize stands for the person logging in on GitLab: it reads the authorize URL and returns the code GitLab
// would send back, remembering the challenge.
func (f *fakeGitLab) authorize(rawURL, user string) (code, state string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		f.t.Fatal(err)
	}
	q := u.Query()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	code = "code-" + user + "-" + string(rune('a'+f.n))
	f.codes[code] = user
	f.challenges[code] = q.Get("code_challenge")
	return code, q.Get("state")
}

func (f *fakeGitLab) issue(user string) map[string]any {
	f.n++
	access, refresh := "at-"+user+"-"+string(rune('a'+f.n)), "rt-"+user+"-"+string(rune('a'+f.n))
	f.tokens[access], f.refreshes[refresh] = user, user
	return map[string]any{"access_token": access, "refresh_token": refresh, "token_type": "Bearer", "expires_in": f.expiresIn}
}

func (f *fakeGitLab) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/oauth/token":
		r.ParseForm()
		if r.PostForm.Get("client_id") != "cid" || r.PostForm.Get("client_secret") != "secret" {
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			code := r.PostForm.Get("code")
			user, ok := f.codes[code]
			sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
			if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenges[code] ||
				r.PostForm.Get("redirect_uri") != "http://192.168.0.82:8783/auth/callback" {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			delete(f.codes, code)
			json.NewEncoder(w).Encode(f.issue(user))
		case "refresh_token":
			user, ok := f.refreshes[r.PostForm.Get("refresh_token")]
			if !ok {
				http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
				return
			}
			delete(f.refreshes, r.PostForm.Get("refresh_token"))
			json.NewEncoder(w).Encode(f.issue(user))
		}
	case "/oauth/userinfo":
		user, ok := f.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"sub": "1", "nickname": user, "groups": f.groups[user]})
	case "/oauth/revoke":
		r.ParseForm()
		f.revoked = append(f.revoked, r.PostForm.Get("token"))
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

const redirect = "http://192.168.0.82:8783/auth/callback"

func newAuth(t *testing.T, srv *httptest.Server, now *time.Time) *Auth {
	a := &Auth{
		Provider: &GitLab{BaseURL: srv.URL, ClientID: "cid", ClientSecret: "secret"},
		Allow:    Allow{MemberGroups: []string{"team"}, AdminGroups: []string{"ops/leads"}, Owner: "olivia"},
	}
	if now != nil {
		a.Now = func() time.Time { return *now }
	}
	return a
}

// login runs the whole flow for user and returns the session id.
func login(t *testing.T, a *Auth, f *fakeGitLab, user string) (string, error) {
	t.Helper()
	ctx := context.Background()
	u, err := a.Start(redirect)
	if err != nil {
		t.Fatal(err)
	}
	code, state := f.authorize(u, user)
	return a.Callback(ctx, state, code, redirect)
}

func TestStartBuildsTheAuthorizeURL(t *testing.T) {
	_, srv := newFakeGitLab(t)
	a := newAuth(t, srv, nil)
	raw, err := a.Start(redirect)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	if u.Path != "/oauth/authorize" || q.Get("client_id") != "cid" || q.Get("redirect_uri") != redirect ||
		q.Get("response_type") != "code" || q.Get("scope") != "openid profile api" ||
		q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || len(q.Get("state")) < 32 {
		t.Fatalf("authorize URL = %s", raw)
	}
}

func TestCallbackLogsAMemberIn(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["alice"] = []string{"team/app"}
	a := newAuth(t, srv, nil)
	id, err := login(t, a, f, "alice")
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.Lookup(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if s.User != "alice" || len(s.Groups) != 1 || s.Groups[0] != "team/app" {
		t.Fatalf("session = %+v", s)
	}
	tok, err := s.Token(context.Background())
	if err != nil || !strings.HasPrefix(tok, "at-alice-") {
		t.Fatalf("token = %q, %v", tok, err)
	}
}

func TestCallbackRefusesAnUnknownState(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["alice"] = []string{"team"}
	a := newAuth(t, srv, nil)
	u, _ := a.Start(redirect)
	code, _ := f.authorize(u, "alice")
	if _, err := a.Callback(context.Background(), "not-a-state", code, redirect); !errors.Is(err, ErrState) {
		t.Fatalf("err = %v, want ErrState", err)
	}
}

func TestCallbackRefusesAStateOlderThanTenMinutes(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["alice"] = []string{"team"}
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	a := newAuth(t, srv, &now)
	u, _ := a.Start(redirect)
	code, state := f.authorize(u, "alice")
	now = now.Add(11 * time.Minute)
	if _, err := a.Callback(context.Background(), state, code, redirect); !errors.Is(err, ErrState) {
		t.Fatalf("err = %v, want ErrState", err)
	}
}

func TestCallbackRefusesSomeoneOutsideTheGroupsAndRevokes(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["mallory"] = []string{"teammates", "other"}
	a := newAuth(t, srv, nil)
	_, err := login(t, a, f, "mallory")
	var na *NotAllowedError
	if !errors.As(err, &na) || na.User != "mallory" {
		t.Fatalf("err = %v, want NotAllowedError for mallory", err)
	}
	if len(f.revoked) != 1 || !strings.HasPrefix(f.revoked[0], "at-mallory-") {
		t.Errorf("revoked = %v, want mallory's access token", f.revoked)
	}
}

func TestAllowed(t *testing.T) {
	allow := Allow{MemberGroups: []string{"team"}, AdminGroups: []string{"ops/leads"}, Owner: "olivia"}
	for _, c := range []struct {
		user   string
		groups []string
		want   bool
	}{
		{"alice", []string{"team"}, true},
		{"bob", []string{"team/app"}, true},
		{"carol", []string{"ops/leads/infra"}, true},
		{"dave", []string{"ops"}, false},
		{"erin", []string{"teammates"}, false},
		{"olivia", nil, true},
		{"frank", nil, false},
	} {
		if got := allow.Allowed(c.user, c.groups); got != c.want {
			t.Errorf("Allowed(%s, %v) = %v, want %v", c.user, c.groups, got, c.want)
		}
	}
}

func TestLookupRefusesAnUnknownSession(t *testing.T) {
	_, srv := newFakeGitLab(t)
	a := newAuth(t, srv, nil)
	if _, err := a.Lookup(context.Background(), "nope"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("err = %v, want ErrNoSession", err)
	}
}

func TestSessionEndsAfterADayIdle(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["alice"] = []string{"team"}
	f.expiresIn = 30 * 24 * 3600 // no refresh in the way
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	a := newAuth(t, srv, &now)
	id, err := login(t, a, f, "alice")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(23 * time.Hour)
	if _, err := a.Lookup(context.Background(), id); err != nil {
		t.Fatalf("after 23h: %v", err)
	}
	now = now.Add(24*time.Hour + time.Second)
	if _, err := a.Lookup(context.Background(), id); !errors.Is(err, ErrNoSession) {
		t.Fatalf("after a day idle: err = %v, want ErrNoSession", err)
	}
}

func TestSessionEndsSevenDaysAfterLogin(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["alice"] = []string{"team"}
	f.expiresIn = 30 * 24 * 3600
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	a := newAuth(t, srv, &now)
	id, _ := login(t, a, f, "alice")
	for i := 0; i < 7; i++ {
		now = now.Add(23 * time.Hour)
		if _, err := a.Lookup(context.Background(), id); err != nil && i < 6 {
			t.Fatalf("day %d: %v", i, err)
		}
	}
	now = now.Add(23 * time.Hour)
	if _, err := a.Lookup(context.Background(), id); !errors.Is(err, ErrNoSession) {
		t.Fatalf("after 7 days: err = %v, want ErrNoSession", err)
	}
}

func TestSessionRefreshesTheTokenAndChecksTheGroupsAgain(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["bob"] = []string{"team"}
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	a := newAuth(t, srv, &now)
	id, _ := login(t, a, f, "bob")
	s, _ := a.Lookup(context.Background(), id)
	first, _ := s.Token(context.Background())

	now = now.Add(2 * time.Hour) // the token expires in 2h: refreshed
	s, err := a.Lookup(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := s.Token(context.Background())
	if second == first {
		t.Fatalf("token was not refreshed: %q", second)
	}

	f.mu.Lock()
	f.groups["bob"] = nil // bob left the team
	f.mu.Unlock()
	now = now.Add(2 * time.Hour)
	if _, err := a.Lookup(context.Background(), id); !errors.Is(err, ErrNoSession) {
		t.Fatalf("after leaving the group: err = %v, want ErrNoSession", err)
	}
}

func TestLogoutEndsTheSessionAndRevokes(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["alice"] = []string{"team"}
	a := newAuth(t, srv, nil)
	id, _ := login(t, a, f, "alice")
	a.Logout(context.Background(), id)
	if _, err := a.Lookup(context.Background(), id); !errors.Is(err, ErrNoSession) {
		t.Fatalf("err = %v, want ErrNoSession", err)
	}
	if len(f.revoked) != 1 {
		t.Errorf("revoked = %v, want the access token", f.revoked)
	}
}

func TestSweepDropsEndedSessions(t *testing.T) {
	f, srv := newFakeGitLab(t)
	f.groups["alice"] = []string{"team"}
	f.expiresIn = 30 * 24 * 3600
	now := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	a := newAuth(t, srv, &now)
	login(t, a, f, "alice")
	now = now.Add(25 * time.Hour)
	a.Sweep()
	if n := a.Len(); n != 0 {
		t.Errorf("sessions = %d after the sweep, want 0", n)
	}
}
