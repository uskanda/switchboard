// Package inboxserve serves the inbox (internal/inbox) as the HTTP API of package inboxapi, for switchboard inbox
// serve. It runs with the inbox tokens; what calls it gets lists, details and replies, never a token.
package inboxserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// DefaultInterval is how often the forges are read while someone listens on /v1/events, when Interval is unset.
const DefaultInterval = 30 * time.Second

// Ping is how often an event stream with nothing to send gets a comment line, so that a broken one is noticed.
const Ping = 15 * time.Second

// Server is the http.Handler of the API.
type Server struct {
	Inbox Inbox
	Now   func() time.Time // time.Now when nil
	// Instructions, when set, serves /v1/instructions.
	Instructions *Instructions
	// Interval is how often the forges are read while someone listens on /v1/events; DefaultInterval when zero.
	Interval time.Duration

	mu     sync.Mutex
	cached *inboxapi.Inbox

	// The listeners of /v1/events, the loop that reads the forges while there is one, and the last listing sent.
	subMu   sync.Mutex
	subs    map[chan *inboxapi.Inbox]bool
	stop    context.CancelFunc
	refresh chan struct{}
	last    *inboxapi.Inbox
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
		"/v1/inbox":  {http.MethodGet, s.list},
		"/v1/issue":  {http.MethodGet, s.issue},
		"/v1/reply":  {http.MethodPost, s.reply},
		"/v1/events": {http.MethodGet, s.events},
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
	send(w, http.StatusOK, s.fetchLocked(context.WithoutCancel(r.Context())))
}

// fetch reads the forges, keeps a complete listing as the cache, and sends the listing to the listeners of
// /v1/events when it differs from the last one they got. One fetch runs at a time.
func (s *Server) fetch(ctx context.Context) *inboxapi.Inbox {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.fetchLocked(ctx)
}

// fetchLocked is fetch with s.mu held. A listing cut short because ctx ended (the last listener left) is returned
// but neither kept nor sent.
func (s *Server) fetchLocked(ctx context.Context) *inboxapi.Inbox {
	ctx, cancel := context.WithTimeout(ctx, FetchTimeout)
	defer cancel()
	items, errs := s.Inbox.Collect(ctx)
	out := &inboxapi.Inbox{Items: []inboxapi.Item{}, Errors: []inboxapi.ForgeError{}, FetchedAt: s.now()}
	for _, it := range items {
		out.Items = append(out.Items, inboxapi.Item(it))
	}
	for _, e := range errs {
		out.Errors = append(out.Errors, inboxapi.ForgeError{Forge: e.Forge, Message: e.Err.Error()})
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return out
	}
	// A listing missing a forge is shown once, not kept: the next request asks again.
	s.cached = nil
	if len(errs) == 0 {
		s.cached = out
	}
	s.publish(out)
	return out
}

// publish sends in to every listener when it differs from the last listing sent, the time it was read aside.
func (s *Server) publish(in *inboxapi.Inbox) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if s.last != nil && same(s.last, in) {
		s.last = in
		return
	}
	s.last = in
	for ch := range s.subs {
		offer(ch, in)
	}
}

// offer puts in on ch, replacing a listing the listener has not taken yet: a slow listener gets the newest listing
// and never holds up the loop.
func offer(ch chan *inboxapi.Inbox, in *inboxapi.Inbox) {
	select {
	case <-ch:
	default:
	}
	ch <- in
}

func same(a, b *inboxapi.Inbox) bool {
	x, _ := json.Marshal(inboxapi.Inbox{Items: a.Items, Errors: a.Errors})
	y, _ := json.Marshal(inboxapi.Inbox{Items: b.Items, Errors: b.Errors})
	return string(x) == string(y)
}

func (s *Server) interval() time.Duration {
	if s.Interval > 0 {
		return s.Interval
	}
	return DefaultInterval
}

// subscribe adds a listener and starts the loop when it is the first. The listener gets the last listing at once
// when it is younger than the interval; otherwise the caller fetches one.
func (s *Server) subscribe(ch chan *inboxapi.Inbox) (fresh bool) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if s.subs == nil {
		s.subs = map[chan *inboxapi.Inbox]bool{}
	}
	s.subs[ch] = true
	if s.stop == nil {
		ctx, cancel := context.WithCancel(context.Background())
		s.stop, s.refresh = cancel, make(chan struct{}, 1)
		go s.loop(ctx, s.refresh)
	}
	if s.last != nil && s.now().Sub(s.last.FetchedAt) < s.interval() {
		offer(ch, s.last)
		return true
	}
	return false
}

// unsubscribe removes a listener and stops the loop when it was the last.
func (s *Server) unsubscribe(ch chan *inboxapi.Inbox) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	delete(s.subs, ch)
	if len(s.subs) == 0 && s.stop != nil {
		s.stop()
		s.stop, s.refresh = nil, nil
	}
}

// loop reads the forges every interval, and at once when asked on refresh, until ctx ends.
func (s *Server) loop(ctx context.Context, refresh <-chan struct{}) {
	t := time.NewTicker(s.interval())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-refresh:
		}
		s.fetch(ctx)
	}
}

// readAgain asks the loop, when it runs, to read the forges without waiting for the interval.
func (s *Server) readAgain() {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if s.refresh != nil {
		select {
		case s.refresh <- struct{}{}:
		default:
		}
	}
}

// events serves GET /v1/events: the listing at once, then each listing that differs, as server-sent events.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		fail(w, http.StatusInternalServerError, inboxapi.CodeInvalid, "the connection cannot stream")
		return
	}
	ch := make(chan *inboxapi.Inbox, 1)
	fresh := s.subscribe(ch)
	defer s.unsubscribe(ch)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	fl.Flush()
	if !fresh {
		// A listener that arrives with nothing recent to show gets a listing of its own, sent to it even when it
		// matches the last one the others got.
		offer(ch, s.fetch(context.WithoutCancel(r.Context())))
	}
	ping := time.NewTicker(Ping)
	defer ping.Stop()
	for {
		var err error
		select {
		case <-r.Context().Done():
			return
		case in := <-ch:
			data, _ := json.Marshal(in)
			_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", inboxapi.EventInbox, data)
		case <-ping.C:
			_, err = fmt.Fprint(w, ": ping\n\n")
		}
		if err != nil {
			return
		}
		fl.Flush()
	}
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
	send(w, http.StatusOK, detailOf(d))
}

// detailOf is d as the API's type: the same fields, the comments copied one by one.
func detailOf(d *inbox.Detail) inboxapi.Detail {
	out := inboxapi.Detail{
		Ref: d.Ref, Title: d.Title, URL: d.URL, Waiting: d.Waiting, Host: d.Host, State: d.State, Reason: d.Reason,
		Since: d.Since, Post: d.Post, ApproveWord: d.ApproveWord, Author: d.Author, CreatedAt: d.CreatedAt, Body: d.Body,
		Comments: []inboxapi.Comment{},
	}
	for _, c := range d.Comments {
		out.Comments = append(out.Comments, inboxapi.Comment(c))
	}
	return out
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
	s.readAgain()
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
