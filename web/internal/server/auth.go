package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/uskanda/switchboard/inboxapi"
)

// CookieName is the cookie that holds the session id the inbox service gave at the login (issue #15). It holds
// nothing else: the tokens stay in the inbox service.
const CookieName = "sb_session"

// callbackURL is where GitLab sends the person back: the GitLab application is registered with this URL.
func (s *Server) callbackURL() string { return "http://" + s.Listen + "/auth/callback" }

// withSession puts the cookie's session id in the request's context, for the calls to the inbox service.
func withSession(r *http.Request) *http.Request {
	if c, err := r.Cookie(CookieName); err == nil && c.Value != "" {
		return r.WithContext(inboxapi.WithSession(r.Context(), c.Value))
	}
	return r
}

// toLogin sends the browser to the front end's login page with why the login failed.
func toLogin(w http.ResponseWriter, r *http.Request, why, user string) {
	q := url.Values{"error": {why}}
	if user != "" {
		q.Set("user", user)
	}
	http.Redirect(w, r, "/login?"+q.Encode(), http.StatusFound)
}

// nextCookie holds, through the visit to GitLab, the page someone opened before logging in (issue #171).
const nextCookie = "sb_next"

// localPath tells whether next is a path of this site, so the login never sends the browser elsewhere: it starts
// with one slash, not two or a backslash after it, holds no control characters, and is not long.
func localPath(next string) bool {
	if len(next) < 1 || len(next) > 1024 || next[0] != '/' || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return false
	}
	return !strings.ContainsFunc(next, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

// login serves GET /auth/login: it starts a login on the inbox service and sends the browser to GitLab. Without
// inbox.auth there is nothing to log in to, and the browser goes to the inbox. A next that is a path of this site is
// kept in a cookie for ten minutes, for the callback to go back to.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	out, err := s.Inbox.AuthStart(r.Context(), inboxapi.AuthStartRequest{Redirect: s.callbackURL()})
	switch {
	case errors.Is(err, inboxapi.ErrNotFound):
		http.Redirect(w, r, "/", http.StatusFound)
	case err != nil:
		toLogin(w, r, "failed", "")
	default:
		if next := r.URL.Query().Get("next"); localPath(next) {
			http.SetCookie(w, &http.Cookie{Name: nextCookie, Value: url.QueryEscape(next), Path: "/auth/", MaxAge: 600, HttpOnly: true, SameSite: http.SameSiteLaxMode})
		}
		http.Redirect(w, r, out.URL, http.StatusFound)
	}
}

// takeNext reads and clears the page to go back to after the login; "/" when there is none or it is not of this site.
func takeNext(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie(nextCookie)
	if err != nil {
		return "/"
	}
	http.SetCookie(w, &http.Cookie{Name: nextCookie, Value: "", Path: "/auth/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	if next, err := url.QueryUnescape(c.Value); err == nil && localPath(next) {
		return next
	}
	return "/"
}

// callback serves GET /auth/callback, where GitLab sends the person back: the inbox service swaps the code for a
// session, whose id becomes the cookie.
func (s *Server) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("error") != "" {
		toLogin(w, r, "denied", "")
		return
	}
	out, err := s.Inbox.AuthCallback(r.Context(), inboxapi.AuthCallbackRequest{State: q.Get("state"), Code: q.Get("code"), Redirect: s.callbackURL()})
	var e *inboxapi.Error
	switch {
	case errors.As(err, &e) && e.Code == inboxapi.CodeNotAllowed:
		toLogin(w, r, "not_allowed", e.User)
		return
	case err != nil:
		toLogin(w, r, "failed", "")
		return
	}
	// No Secure: the web inbox serves plain HTTP on the LAN for now (issue #155).
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: out.Session, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, takeNext(w, r), http.StatusFound)
}

// logout serves POST /api/logout, from the web inbox's own page only: the session ends and its cookie is cleared.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	if err := s.Inbox.Logout(r.Context()); err != nil {
		s.answer(w, nil, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}
