package inboxserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/webauth"
)

type sessionKey struct{}

// SessionFrom gives the session of a request the server took with inbox.auth set, or nil.
func SessionFrom(ctx context.Context) *webauth.Session {
	s, _ := ctx.Value(sessionKey{}).(*webauth.Session)
	return s
}

// authorize lets a request through when the login is off, or when it carries a live session, which it puts in the
// request's context. Everything under /v1/auth/ passes: it is how one logs in. It tells whether to go on.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) (*http.Request, bool) {
	if s.Auth == nil || strings.HasPrefix(r.URL.Path, "/v1/auth/") {
		return r, true
	}
	sess, err := s.Auth.Lookup(r.Context(), r.Header.Get(inboxapi.SessionHeader))
	if err != nil {
		fail(w, http.StatusUnauthorized, inboxapi.CodeUnauthenticated, "log in to the web inbox first")
		return r, false
	}
	if s.observeSession != nil {
		s.observeSession(sess)
	}
	return r.WithContext(context.WithValue(r.Context(), sessionKey{}, sess)), true
}

// authStatus serves GET /v1/auth/status.
func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	out := inboxapi.AuthStatus{Enabled: s.Auth != nil}
	if s.Auth != nil {
		out.GitLab = s.GitLab
		if sess, err := s.Auth.Lookup(r.Context(), r.Header.Get(inboxapi.SessionHeader)); err == nil {
			out.User = sess.User
			out.Admin = s.Users == nil || s.Users.Admin(sess)
		}
	}
	send(w, http.StatusOK, out)
}

// authStart serves POST /v1/auth/start.
func (s *Server) authStart(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "the login is off: inbox.auth is not set")
		return
	}
	var req inboxapi.AuthStartRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil || req.Redirect == "" {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not the start of a login")
		return
	}
	u, err := s.Auth.Start(req.Redirect)
	if err != nil {
		fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
		return
	}
	send(w, http.StatusOK, inboxapi.AuthStartResponse{URL: u})
}

// authCallback serves POST /v1/auth/callback.
func (s *Server) authCallback(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "the login is off: inbox.auth is not set")
		return
	}
	var req inboxapi.AuthCallbackRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not the end of a login")
		return
	}
	id, err := s.Auth.Callback(r.Context(), req.State, req.Code, req.Redirect)
	var na *webauth.NotAllowedError
	switch {
	case errors.As(err, &na):
		send(w, http.StatusForbidden, inboxapi.Error{Code: inboxapi.CodeNotAllowed, Message: err.Error(), User: na.User})
		return
	case errors.Is(err, webauth.ErrState):
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, err.Error())
		return
	case err != nil:
		fail(w, http.StatusBadGateway, inboxapi.CodeForge, err.Error())
		return
	}
	sess, err := s.Auth.Lookup(r.Context(), id)
	if err != nil {
		fail(w, http.StatusBadGateway, inboxapi.CodeForge, err.Error())
		return
	}
	if s.Audit != nil {
		s.Audit.Log("auth.login", map[string]any{"user": sess.User})
	}
	send(w, http.StatusOK, inboxapi.AuthCallbackResponse{Session: id, User: sess.User})
}

// authLogout serves POST /v1/auth/logout.
func (s *Server) authLogout(w http.ResponseWriter, r *http.Request) {
	if s.Auth != nil {
		s.Auth.Logout(r.Context(), r.Header.Get(inboxapi.SessionHeader))
	}
	w.WriteHeader(http.StatusNoContent)
}
