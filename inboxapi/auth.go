package inboxapi

import (
	"context"
	"net/http"
)

// SessionHeader carries the web inbox's session id to the inbox service, which keeps the sessions (issue #15).
const SessionHeader = "X-Switchboard-Session"

type sessionKey struct{}

// WithSession gives a context whose calls through a Client carry session id.
func WithSession(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionKey{}, id)
}

// SessionOf gives the session id WithSession put in ctx, or "".
func SessionOf(ctx context.Context) string {
	id, _ := ctx.Value(sessionKey{}).(string)
	return id
}

// AuthStatus is GET /v1/auth/status: whether people must log in (inbox.auth is set), who is logged in with the
// request's session, and the GitLab they log in on.
type AuthStatus struct {
	Enabled bool   `json:"enabled"`
	User    string `json:"user,omitempty"`
	GitLab  string `json:"gitlab,omitempty"`
	// Admin tells whether the person logged in may edit the instructions and add projects (issue #146).
	Admin bool `json:"admin,omitempty"`
}

// AuthStartRequest is POST /v1/auth/start: Redirect is the web inbox's /auth/callback.
type AuthStartRequest struct {
	Redirect string `json:"redirect"`
}

// AuthStartResponse is GitLab's authorize page to send the browser to.
type AuthStartResponse struct {
	URL string `json:"url"`
}

// AuthCallbackRequest is POST /v1/auth/callback: what GitLab sent back to the web inbox's /auth/callback.
type AuthCallbackRequest struct {
	State    string `json:"state"`
	Code     string `json:"code"`
	Redirect string `json:"redirect"`
}

// AuthCallbackResponse is the new session's id, for the web inbox's cookie, and who logged in.
type AuthCallbackResponse struct {
	Session string `json:"session"`
	User    string `json:"user"`
}

// AuthStatus tells whether people must log in, and who the context's session is.
func (c *Client) AuthStatus(ctx context.Context) (*AuthStatus, error) {
	var out AuthStatus
	return &out, c.do(ctx, http.MethodGet, "/v1/auth/status", nil, &out)
}

// AuthStart begins a login and gives the URL to send the browser to.
func (c *Client) AuthStart(ctx context.Context, req AuthStartRequest) (*AuthStartResponse, error) {
	var out AuthStartResponse
	return &out, c.do(ctx, http.MethodPost, "/v1/auth/start", req, &out)
}

// AuthCallback finishes a login. Someone GitLab logged in who may not use the inbox gets ErrNotAllowed, with their
// name in the Error's User.
func (c *Client) AuthCallback(ctx context.Context, req AuthCallbackRequest) (*AuthCallbackResponse, error) {
	var out AuthCallbackResponse
	return &out, c.do(ctx, http.MethodPost, "/v1/auth/callback", req, &out)
}

// Logout ends the context's session.
func (c *Client) Logout(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/v1/auth/logout", nil, nil)
}

// sessionTransport puts the request context's session id in SessionHeader.
type sessionTransport struct{ next http.RoundTripper }

func (t sessionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if id := SessionOf(r.Context()); id != "" {
		r = r.Clone(r.Context())
		r.Header.Set(SessionHeader, id)
	}
	return t.next.RoundTrip(r)
}
