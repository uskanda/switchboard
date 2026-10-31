// Package inboxserve serves the inbox (internal/inbox) as the HTTP API of package inboxapi, for switchboard inbox
// serve. It runs with the inbox tokens; what calls it gets lists, details and replies, never a token.
package inboxserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
)

// Inbox is what the server needs of *inbox.Inbox.
type Inbox interface {
	Collect(ctx context.Context) ([]inbox.Item, []inbox.ForgeError)
	ParseRef(arg string) (forge.IssueRef, error)
	Detail(ctx context.Context, ref forge.IssueRef) (*inbox.Detail, error)
	Reply(ctx context.Context, ref forge.IssueRef, text string, force bool) error
}

// TTL is how long a listing is served without asking the forges again. It bounds how late an open web inbox shows a
// change, and keeps a GitHub inbox at two searches a minute, well under the Search API's 30.
const TTL = 30 * time.Second

// Server is the http.Handler of the API.
type Server struct {
	Inbox Inbox
	Now   func() time.Time // time.Now when nil

	mu     sync.Mutex
	cached *inboxapi.Inbox
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := map[string]struct {
		method string
		h      func(http.ResponseWriter, *http.Request)
	}{
		"/v1/inbox": {http.MethodGet, s.list},
		"/v1/issue": {http.MethodGet, s.issue},
		"/v1/reply": {http.MethodPost, s.reply},
	}[r.URL.Path]
	switch {
	case route.h == nil:
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "no such endpoint")
	case r.Method != route.method:
		w.Header().Set("Allow", route.method)
		fail(w, http.StatusMethodNotAllowed, inboxapi.CodeInvalid, "use "+route.method)
	default:
		route.h(w, r)
	}
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached == nil || s.now().Sub(s.cached.FetchedAt) >= TTL {
		items, errs := s.Inbox.Collect(r.Context())
		out := &inboxapi.Inbox{Items: []inboxapi.Item{}, Errors: []inboxapi.ForgeError{}, FetchedAt: s.now()}
		for _, it := range items {
			out.Items = append(out.Items, inboxapi.Item(it))
		}
		for _, e := range errs {
			out.Errors = append(out.Errors, inboxapi.ForgeError{Forge: e.Forge, Message: e.Err.Error()})
		}
		s.cached = out
	}
	send(w, http.StatusOK, s.cached)
}

func (s *Server) issue(w http.ResponseWriter, r *http.Request) {
	ref, err := s.Inbox.ParseRef(r.URL.Query().Get("ref"))
	if err != nil {
		failWith(w, err)
		return
	}
	d, err := s.Inbox.Detail(r.Context(), ref)
	if err != nil {
		failWith(w, err)
		return
	}
	send(w, http.StatusOK, inboxapi.Detail(*d))
}

func (s *Server) reply(w http.ResponseWriter, r *http.Request) {
	var req inboxapi.ReplyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not a reply request")
		return
	}
	ref, err := s.Inbox.ParseRef(req.Ref)
	if err != nil {
		failWith(w, err)
		return
	}
	if err := s.Inbox.Reply(r.Context(), ref, req.Text, req.Force); err != nil {
		failWith(w, err)
		return
	}
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// failWith answers with the code the inbox's error stands for.
func failWith(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, inbox.ErrInvalid):
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, err.Error())
	case errors.Is(err, inbox.ErrNotFound):
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, err.Error())
	case errors.Is(err, inbox.ErrNotWaiting):
		fail(w, http.StatusConflict, inboxapi.CodeNotWaiting, err.Error())
	default:
		fail(w, http.StatusBadGateway, inboxapi.CodeForge, err.Error())
	}
}

func fail(w http.ResponseWriter, status int, code, msg string) {
	send(w, status, inboxapi.Error{Code: code, Message: msg})
}

func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
