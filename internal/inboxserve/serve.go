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
	"github.com/uskanda/switchboard/internal/automerge"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/instructions"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/webauth"
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
	// Projects, when set, serves /v1/projects.
	Projects *Projects
	// NewIssues, when set, serves /v1/issue-targets and /v1/issues.
	NewIssues *NewIssues
	// Merges, when set, serves /v1/merge: it merges change requests with each forge's inbox token, by forge kind
	// (ADR-043). The Inbox must be a Merger.
	Merges map[string]forge.Merges
	// IssueImages, when set, serves /v1/issue-image.
	IssueImages *IssueImages
	// Icons, when set, serves /v1/project-icon and /v1/project-icons.
	Icons *Icons
	// CI, when set, serves /v1/merge-status: it reads the change requests and the CI of done issues with each forge's
	// inbox token, by forge kind, for the merge dialog (ADR-050). The Inbox must be a MergeStatusReader.
	CI map[string]forge.CI
	// Relay reports that the service follows the relay's events (inbox.relay is set): the listings then say
	// whether it is connected, and RelayState and RelayEvent are called as the stream goes.
	Relay bool
	// AutoMerge, when set with Merges, keeps the merges the forge refused for a conflict, which the inbox service
	// makes once the agent has fixed it (ADR-048), and serves /v1/automerge/cancel. RunAutoMerge looks at them.
	AutoMerge *automerge.Store
	// Audit, when set, records what happens to those merges; Logf, when set, what went wrong looking at them.
	Audit state.Auditor
	Logf  func(format string, args ...any)

	// kick wakes RunAutoMerge; made once.
	kickOnce sync.Once
	kick     chan struct{}

	// Devices, when set, serves /v1/devices: the apps register their FCM tokens there.
	Devices Registry
	// Observer, when set, gets every listing read from the forges, in order. Watch keeps them read with no page.
	Observer Observer
	// Auth, when set (inbox.auth), logs people in with GitLab and refuses every request without a live session but
	// those under /v1/auth/ (issue #15). GitLab is the base URL people log in on, for the app's WebView.
	Auth   *webauth.Auth
	GitLab string
	// Users, set with Auth, tells the people apart: what each one sees and writes with (issue #146).
	Users *Users
	// observeSession, when set, gets the session of each request let through; for the tests.
	observeSession func(*webauth.Session)

	// vmu guards Inbox, Instructions.Projects, Projects.Config, NewIssues.Config and Icons.Config, which Reload
	// replaces while requests read them.
	vmu sync.RWMutex

	mu     sync.Mutex
	cached *inboxapi.Inbox

	// The listeners of /v1/events, the loop that reads the forges while there is one, and the last listing sent.
	subMu   sync.Mutex
	subs    map[chan *inboxapi.Inbox]bool
	stop    context.CancelFunc
	refresh chan struct{}
	last    *inboxapi.Inbox
	// Under subMu too: whether the relay is connected, the read a relay event has asked for and not yet made, and
	// whether Watch keeps the loop running with no listener.
	relayUp  bool
	watching bool
	pending  *time.Timer
	// The debounce and the gap of relay events; RelayDebounce and config.MinRefreshInterval when zero.
	debounce, gap time.Duration

	// The listings waiting for the Observer.
	obsOnce sync.Once
	obsMu   sync.Mutex
	obs     chan *inboxapi.Inbox
}

// inbox is the inbox in use.
func (s *Server) inbox() Inbox {
	s.vmu.RLock()
	defer s.vmu.RUnlock()
	return s.Inbox
}

// Reload takes a configuration with new projects and the inbox b made for it: the listing, the instructions, the
// projects, the targets of new issues and the icons follow it from the next request, and the cached listing is dropped.
func (s *Server) Reload(cfg *config.Config, b Inbox) {
	s.vmu.Lock()
	s.Inbox = b
	if s.Instructions != nil {
		s.Instructions.Projects = refs(cfg.Projects)
	}
	if s.Projects != nil {
		s.Projects.Config = cfg
	}
	if s.NewIssues != nil {
		s.NewIssues.Config = cfg
	}
	if s.Icons != nil {
		s.Icons.Config = cfg
	}
	if s.IssueImages != nil {
		s.IssueImages.Config = cfg
	}
	s.vmu.Unlock()
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	s.readAgain()
}

// refs lists the references of projects, in order.
func refs(projects []config.Project) []string {
	out := make([]string, 0, len(projects))
	for _, p := range projects {
		out = append(out, p.Ref)
	}
	return out
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
		"/v1/inbox":   {http.MethodGet, s.list},
		"/v1/issue":   {http.MethodGet, s.issue},
		"/v1/reply":   {http.MethodPost, s.reply},
		"/v1/events":  {http.MethodGet, s.events},
		"/v1/approve": {http.MethodPost, s.approve},
		// The login (issue #15); without inbox.auth, start and callback answer not found.
		"/v1/auth/status":   {http.MethodGet, s.authStatus},
		"/v1/auth/start":    {http.MethodPost, s.authStart},
		"/v1/auth/callback": {http.MethodPost, s.authCallback},
		"/v1/auth/logout":   {http.MethodPost, s.authLogout},
	}
	if s.Devices != nil {
		routes["/v1/devices"] = route{http.MethodPost, s.devices}
	}
	if s.Instructions != nil {
		routes["/v1/instructions"] = route{"", s.instructions}
	}
	if s.Projects != nil {
		routes["/v1/projects"] = route{"", s.projects}
	}
	if s.NewIssues != nil {
		routes["/v1/issue-targets"] = route{http.MethodGet, s.issueTargets}
		routes["/v1/issues"] = route{http.MethodPost, s.createIssue}
	}
	if s.Merges != nil {
		routes["/v1/merge"] = route{http.MethodPost, s.merge}
		if s.AutoMerge != nil {
			routes["/v1/automerge/cancel"] = route{http.MethodPost, s.cancelAutoMerge}
		}
	}
	if s.IssueImages != nil {
		routes["/v1/issue-image"] = route{http.MethodGet, s.issueImage}
	}
	if s.Icons != nil {
		routes["/v1/project-icon"] = route{http.MethodGet, s.projectIcon}
		routes["/v1/project-icons"] = route{http.MethodGet, s.projectIcons}
	}
	if s.CI != nil {
		routes["/v1/merge-status"] = route{http.MethodGet, s.mergeStatus}
	}
	rt := routes[r.URL.Path]
	r, ok := s.authorize(w, r)
	if !ok {
		return
	}
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
		// The relay's state is the one now, not the one when the listing was read.
		c := *s.cached
		c.Relay = s.relayState()
		send(w, http.StatusOK, s.visible(r.Context(), &c))
		return
	}
	// The forges are asked apart from the request: a caller that gives up (a browser reloading the page) must not
	// cut the answers short, or the listing would hold its own cancellation.
	send(w, http.StatusOK, s.visible(r.Context(), s.fetchLocked(context.WithoutCancel(r.Context()))))
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
	items, errs := s.inbox().Collect(ctx)
	out := &inboxapi.Inbox{Items: []inboxapi.Item{}, Errors: []inboxapi.ForgeError{}, FetchedAt: s.now(),
		Relay: s.relayState(), RefreshIntervalSeconds: int(s.interval() / time.Second)}
	for _, it := range items {
		out.Items = append(out.Items, inboxapi.Item(it))
	}
	s.withAutoMerge(out)
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
	s.observe(out)
	return out
}

// publish sends in to every listener when it differs from the last listing sent, the time it was read aside.
func (s *Server) publish(in *inboxapi.Inbox) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	// The relay's state may have changed while the forges were read.
	in.Relay = s.relayLocked()
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
	x, _ := json.Marshal(inboxapi.Inbox{Items: a.Items, Errors: a.Errors, Relay: a.Relay, AutoMergeResults: a.AutoMergeResults})
	y, _ := json.Marshal(inboxapi.Inbox{Items: b.Items, Errors: b.Errors, Relay: b.Relay, AutoMergeResults: b.AutoMergeResults})
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
	if len(s.subs) == 0 && !s.watching && s.stop != nil {
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
			data, _ := json.Marshal(s.visible(r.Context(), in))
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
	ref, err := s.inbox().ParseRef(r.URL.Query().Get("ref"))
	if err != nil {
		failWith(w, err)
		return
	}
	if !s.sees(r.Context(), ref.String()) {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, ref.String()+" is not one you may see")
		return
	}
	d, err := s.inbox().Detail(r.Context(), ref)
	if err != nil {
		failWith(w, err)
		return
	}
	out := detailOf(d)
	s.followDetail(out)
	if d.Done {
		out.AutoMerge = s.autoMergeOf(out.Ref)
	}
	if sess := SessionFrom(r.Context()); s.Users != nil && sess != nil {
		t := s.Users.trusted(sess, ref.Kind())
		out.Trusted = &t
	}
	send(w, http.StatusOK, out)
}

// followDetail asks the loop to read the forges again when d, just read from the forge, is not in the state the
// listing last sent shows it in (issue #82): the listing then catches up and reaches the pages as an event. Only
// running, queued, done, waiting and not listed are told apart; the reasons are not. Without a listing sent, the
// loop does not run and nothing is asked.
func (s *Server) followDetail(d inboxapi.Detail) {
	s.subMu.Lock()
	last := s.last
	s.subMu.Unlock()
	if last == nil {
		return
	}
	listed := ""
	for _, it := range last.Items {
		if it.Ref == d.Ref {
			listed = it.State
			break
		}
	}
	if listed != listState(d) {
		s.readAgain()
	}
}

// listState is the state a row of the listing would show for d, or "" when d would not be listed. The order is the
// one of the web UI's IssuePane: a running or queued issue may still be marked waiting or done.
func listState(d inboxapi.Detail) string {
	switch {
	case d.State == inbox.Running:
		return inbox.Running
	case d.State == inbox.Queued:
		return inbox.Queued
	case d.Done:
		return inbox.Done
	case d.Waiting:
		return inbox.Waiting
	}
	return ""
}

// detailOf is d as the API's type: the same fields, the comments copied one by one.
func detailOf(d *inbox.Detail) inboxapi.Detail {
	out := inboxapi.Detail{
		Ref: d.Ref, Title: d.Title, URL: d.URL, Waiting: d.Waiting, Done: d.Done, Host: d.Host, State: d.State, Reason: d.Reason,
		Since: d.Since, Post: d.Post, ApproveWord: d.ApproveWord, ChangeURL: d.ChangeURL, ChangeNumber: d.ChangeNumber,
		ChangeSHA: d.ChangeSHA, Author: d.Author, CreatedAt: d.CreatedAt, Body: d.Body,
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
	ref, err := s.inbox().ParseRef(req.Ref)
	if err != nil {
		failWith(w, err)
		return
	}
	if !s.sees(r.Context(), ref.String()) {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, ref.String()+" is not one you may see")
		return
	}
	if err := s.replyAs(r.Context(), ref, req.Text, req.Force); err != nil {
		if !failWrite(w, writeErr(err)) {
			failWith(w, err)
		}
		return
	}
	s.act(r.Context(), "reply", ref.String())
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	s.readAgain()
	w.WriteHeader(http.StatusNoContent)
}

// instructions serves GET (the global and every project's instructions, of every kind) and PUT (save one) of
// /v1/instructions.
func (s *Server) instructions(w http.ResponseWriter, r *http.Request) {
	if !s.adminOnly(w, r) {
		return
	}
	in := s.Instructions
	s.vmu.RLock()
	projects := slices.Clone(in.Projects)
	s.vmu.RUnlock()
	switch r.Method {
	case http.MethodGet:
		out := inboxapi.Instructions{Host: in.Host, Items: []inboxapi.Instruction{}}
		for _, t := range append([]string{instructions.Global}, projects...) {
			for _, k := range instructions.Kinds {
				text, err := in.Store.Of(k).Read(t)
				if err != nil {
					fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
					return
				}
				out.Items = append(out.Items, inboxapi.Instruction{Target: t, Kind: string(k), Content: text})
			}
		}
		send(w, http.StatusOK, out)
	case http.MethodPut:
		var req inboxapi.SaveInstructionsRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not a save of instructions")
			return
		}
		if req.Target != instructions.Global && !slices.Contains(projects, req.Target) {
			fail(w, http.StatusUnprocessableEntity, inboxapi.CodeUnknownProject, req.Target+" is not a project in the configuration")
			return
		}
		kind, err := instructions.ParseKind(req.Kind)
		if err != nil {
			fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, err.Error())
			return
		}
		switch err := in.Store.Of(kind).Save(req.Target, req.Content); {
		case errors.Is(err, instructions.ErrTooLarge):
			fail(w, http.StatusRequestEntityTooLarge, inboxapi.CodeTooLarge, err.Error())
		case errors.Is(err, instructions.ErrSecret):
			fail(w, http.StatusUnprocessableEntity, inboxapi.CodeSecret, err.Error())
		case err != nil:
			fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
		default:
			s.act(r.Context(), "instructions.save", req.Target+" "+req.Kind)
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
