package server

import (
	"errors"
	"net/http"
	"net/url"

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

// login serves GET /auth/login: it starts a login on the inbox service and sends the browser to GitLab. Without
// inbox.auth there is nothing to log in to, and the browser goes to the inbox.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	out, err := s.Inbox.AuthStart(r.Context(), inboxapi.AuthStartRequest{Redirect: s.callbackURL()})
	switch {
	case errors.Is(err, inboxapi.ErrNotFound):
		http.Redirect(w, r, "/", http.StatusFound)
	case err != nil:
		toLogin(w, r, "failed", "")
	default:
		http.Redirect(w, r, out.URL, http.StatusFound)
	}
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
	http.Redirect(w, r, "/", http.StatusFound)
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
