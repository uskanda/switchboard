// Package webauth logs people in to the web inbox with GitLab's OpenID Connect (issue #15) and keeps their
// sessions. It runs inside the inbox service: the tokens it gets never leave it, and the web inbox only ever holds a
// session id (ADR-037).
package webauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"
)

// The lifetimes of a login. A state waits StateTTL for GitLab to send the person back; a session ends IdleTTL after
// its last request or MaxTTL after the login, whichever comes first. A token closer than RefreshBefore to its expiry
// is refreshed before a request uses it.
const (
	StateTTL      = 10 * time.Minute
	IdleTTL       = 24 * time.Hour
	MaxTTL        = 7 * 24 * time.Hour
	RefreshBefore = 5 * time.Minute
)

var (
	// ErrState is returned by Callback for a state this service did not hand out, or one older than StateTTL.
	ErrState = errors.New("the login was not started here or took too long: log in again")
	// ErrNoSession is returned by Lookup for a session that does not exist or has ended.
	ErrNoSession = errors.New("not logged in")
)

// NotAllowedError is returned by Callback when GitLab logged in someone who may not use the inbox.
type NotAllowedError struct{ User string }

func (e *NotAllowedError) Error() string {
	return e.User + " may not use this inbox: they are in none of inbox.auth's groups"
}

// Token is what the provider answers a code or a refresh with.
type Token struct {
	Access, Refresh string
	ExpiresIn       time.Duration
}

// User is who a token belongs to: the GitLab user name and the full paths of their groups.
type User struct {
	Name   string
	Groups []string
}

// Provider is the OpenID Connect side of GitLab.
type Provider interface {
	AuthURL(redirect, state, challenge string) string
	Exchange(ctx context.Context, code, verifier, redirect string) (Token, error)
	Refresh(ctx context.Context, refresh string) (Token, error)
	UserInfo(ctx context.Context, access string) (User, error)
	Revoke(ctx context.Context, token string)
}

// Allow says who may log in: a member of one of the groups, or of a group below one, and the owner.
type Allow struct {
	MemberGroups, AdminGroups []string
	Owner                     string
}

// Allowed tells whether user, in groups, may log in.
func (a Allow) Allowed(user string, groups []string) bool {
	if a.Owner != "" && user == a.Owner {
		return true
	}
	for _, g := range groups {
		for _, want := range slices.Concat(a.MemberGroups, a.AdminGroups) {
			if g == want || strings.HasPrefix(g, want+"/") {
				return true
			}
		}
	}
	return false
}

// Session is one logged-in person.
type Session struct {
	User   string
	Groups []string

	auth    *Auth
	id      string
	mu      sync.Mutex
	token   Token
	expiry  time.Time
	created time.Time
	used    time.Time
}

// Token gives the session's access token for GitLab, refreshed first when it is about to expire.
func (s *Session) Token(ctx context.Context) (string, error) {
	if err := s.auth.fresh(ctx, s); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token.Access, nil
}

type pending struct {
	verifier, redirect string
	at                 time.Time
}

// Auth runs the logins and keeps the sessions, in memory only: a restart of the inbox service logs everyone out.
type Auth struct {
	Provider Provider
	Allow    Allow
	Now      func() time.Time // time.Now when nil

	mu       sync.Mutex
	states   map[string]pending
	sessions map[string]*Session
}

func (a *Auth) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// random is n random bytes in hex.
func random(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Start begins a login that comes back to redirect, and gives the URL of GitLab's authorize page.
func (a *Auth) Start(redirect string) (string, error) {
	state, verifier := random(32), random(32)
	sum := sha256.Sum256([]byte(verifier))
	a.mu.Lock()
	if a.states == nil {
		a.states = map[string]pending{}
	}
	a.states[state] = pending{verifier: verifier, redirect: redirect, at: a.now()}
	a.mu.Unlock()
	return a.Provider.AuthURL(redirect, state, base64.RawURLEncoding.EncodeToString(sum[:])), nil
}

// Callback finishes a login: it swaps code for tokens, reads who logged in, and makes a session for someone allowed.
// It gives the session id. Someone not allowed gets a NotAllowedError, and their token is revoked.
func (a *Auth) Callback(ctx context.Context, state, code, redirect string) (string, error) {
	a.mu.Lock()
	p, ok := a.states[state]
	delete(a.states, state)
	a.mu.Unlock()
	if !ok || a.now().Sub(p.at) > StateTTL || p.redirect != redirect {
		return "", ErrState
	}
	tok, err := a.Provider.Exchange(ctx, code, p.verifier, redirect)
	if err != nil {
		return "", err
	}
	u, err := a.Provider.UserInfo(ctx, tok.Access)
	if err != nil {
		a.Provider.Revoke(ctx, tok.Access)
		return "", err
	}
	if !a.Allow.Allowed(u.Name, u.Groups) {
		a.Provider.Revoke(ctx, tok.Access)
		return "", &NotAllowedError{User: u.Name}
	}
	now := a.now()
	s := &Session{User: u.Name, Groups: u.Groups, auth: a, id: random(32), token: tok, expiry: now.Add(tok.ExpiresIn),
		created: now, used: now}
	a.mu.Lock()
	if a.sessions == nil {
		a.sessions = map[string]*Session{}
	}
	a.sessions[s.id] = s
	a.mu.Unlock()
	return s.id, nil
}

// Lookup gives the session of id, counting this as its last request. A session past its time, or whose person is
// no longer allowed when its token is refreshed, is ended: ErrNoSession.
func (a *Auth) Lookup(ctx context.Context, id string) (*Session, error) {
	a.mu.Lock()
	s, ok := a.sessions[id]
	a.mu.Unlock()
	if !ok {
		return nil, ErrNoSession
	}
	now := a.now()
	s.mu.Lock()
	ended := now.Sub(s.used) > IdleTTL || now.Sub(s.created) > MaxTTL
	s.mu.Unlock()
	if ended {
		a.end(ctx, s, false)
		return nil, ErrNoSession
	}
	if err := a.fresh(ctx, s); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.used = now
	s.mu.Unlock()
	return s, nil
}

// fresh refreshes s's token when it is about to expire, and ends s when the refresh fails or its person is no longer
// allowed.
func (a *Auth) fresh(ctx context.Context, s *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.now().Add(RefreshBefore).Before(s.expiry) {
		return nil
	}
	now := a.now()
	tok, err := a.Provider.Refresh(ctx, s.token.Refresh)
	if err == nil {
		var u User
		if u, err = a.Provider.UserInfo(ctx, tok.Access); err == nil && !a.Allow.Allowed(u.Name, u.Groups) {
			err = &NotAllowedError{User: u.Name}
		}
		if err == nil {
			s.token, s.expiry, s.Groups = tok, now.Add(tok.ExpiresIn), u.Groups
			return nil
		}
		a.Provider.Revoke(ctx, tok.Access)
	}
	a.mu.Lock()
	delete(a.sessions, s.id)
	a.mu.Unlock()
	return ErrNoSession
}

// Logout ends the session of id and revokes its token.
func (a *Auth) Logout(ctx context.Context, id string) {
	a.mu.Lock()
	s, ok := a.sessions[id]
	a.mu.Unlock()
	if ok {
		a.end(ctx, s, true)
	}
}

func (a *Auth) end(ctx context.Context, s *Session, revoke bool) {
	a.mu.Lock()
	delete(a.sessions, s.id)
	a.mu.Unlock()
	if revoke {
		s.mu.Lock()
		tok := s.token.Access
		s.mu.Unlock()
		a.Provider.Revoke(ctx, tok)
	}
}

// Sweep drops the sessions past their time and the states GitLab never sent back. The inbox service runs it every
// minute.
func (a *Auth) Sweep() {
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for id, s := range a.sessions {
		s.mu.Lock()
		if now.Sub(s.used) > IdleTTL || now.Sub(s.created) > MaxTTL {
			delete(a.sessions, id)
		}
		s.mu.Unlock()
	}
	for st, p := range a.states {
		if now.Sub(p.at) > StateTTL {
			delete(a.states, st)
		}
	}
}

// Len is the number of sessions kept.
func (a *Auth) Len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sessions)
}
