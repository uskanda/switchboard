// Package inboxserve serves the inbox (internal/inbox) as the HTTP API of package inboxapi, for switchboard inbox
// serve. It runs with the inbox tokens; what calls it gets lists, details and replies, never a token.
package inboxserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/instructions"
)

// Inbox is what the server needs of *inbox.Inbox.
type Inbox interface {
	Collect(ctx context.Context) ([]inbox.Item, []inbox.ForgeError)
	ParseRef(arg string) (forge.IssueRef, error)
	Detail(ctx context.Context, ref forge.IssueRef) (*inbox.Detail, error)
	Reply(ctx context.Context, ref forge.IssueRef, text string, force bool) error
}

// TTL is how long a complete listing is served without asking the forges again.
const TTL = time.Minute

// FetchTimeout bounds one listing from the forges.
const FetchTimeout = 30 * time.Second

// Instructions is where the API reads and writes the additional instructions of the plan runs: the store, the
// configured projects in order, and the name of this machine.
type Instructions struct {
	Store    instructions.Store
	Projects []string
	Host     string
}

// Server is the http.Handler of the API.
type Server struct {
	Inbox Inbox
	Now   func() time.Time // time.Now when nil
	// Instructions, when set, serves /v1/instructions.
	Instructions *Instructions

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
	type route struct {
		method string
		h      func(http.ResponseWriter, *http.Request)
	}
	routes := map[string]route{
		"/v1/inbox": {http.MethodGet, s.list},
		"/v1/issue": {http.MethodGet, s.issue},
		"/v1/reply": {http.MethodPost, s.reply},
	}
	if s.Instructions != nil {
		routes["/v1/instructions"] = route{"", s.instructions}
	}
	rt := routes[r.URL.Path]
	switch {
	case rt.h == nil:
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "no such endpoint")
	case rt.method != "" && r.Method != rt.method:
		w.Header().Set("Allow", rt.method)
		fail(w, http.StatusMethodNotAllowed, inboxapi.CodeInvalid, "use "+rt.method)
	default:
		rt.h(w, r)
	}
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && s.now().Sub(s.cached.FetchedAt) < TTL {
		send(w, http.StatusOK, s.cached)
		return
	}
	// The forges are asked apart from the request: a caller that gives up (a browser reloading the page) must not
	// cut the answers short, or the listing would hold its own cancellation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), FetchTimeout)
	defer cancel()
	items, errs := s.Inbox.Collect(ctx)
	out := &inboxapi.Inbox{Items: []inboxapi.Item{}, Errors: []inboxapi.ForgeError{}, FetchedAt: s.now()}
	for _, it := range items {
		out.Items = append(out.Items, inboxapi.Item(it))
	}
	for _, e := range errs {
		out.Errors = append(out.Errors, inboxapi.ForgeError{Forge: e.Forge, Message: e.Err.Error()})
	}
	// A listing missing a forge is shown once, not kept: the next request asks again.
	s.cached = nil
	if len(errs) == 0 {
		s.cached = out
	}
	send(w, http.StatusOK, out)
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

// instructions serves GET (the global and every project's instructions) and PUT (save one) of /v1/instructions.
func (s *Server) instructions(w http.ResponseWriter, r *http.Request) {
	in := s.Instructions
	switch r.Method {
	case http.MethodGet:
		out := inboxapi.Instructions{Host: in.Host, Items: []inboxapi.Instruction{}}
		for _, t := range append([]string{instructions.Global}, in.Projects...) {
			text, err := in.Store.Read(t)
			if err != nil {
				fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
				return
			}
			out.Items = append(out.Items, inboxapi.Instruction{Target: t, Content: text})
		}
		send(w, http.StatusOK, out)
	case http.MethodPut:
		var req inboxapi.SaveInstructionsRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not a save of instructions")
			return
		}
		if req.Target != instructions.Global && !slices.Contains(in.Projects, req.Target) {
			fail(w, http.StatusUnprocessableEntity, inboxapi.CodeUnknownProject, req.Target+" is not a project in the configuration")
			return
		}
		switch err := in.Store.Save(req.Target, req.Content); {
		case errors.Is(err, instructions.ErrTooLarge):
			fail(w, http.StatusRequestEntityTooLarge, inboxapi.CodeTooLarge, err.Error())
		case errors.Is(err, instructions.ErrSecret):
			fail(w, http.StatusUnprocessableEntity, inboxapi.CodeSecret, err.Error())
		case err != nil:
			fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	default:
		w.Header().Set("Allow", "GET, PUT")
		fail(w, http.StatusMethodNotAllowed, inboxapi.CodeInvalid, "use GET or PUT")
	}
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
