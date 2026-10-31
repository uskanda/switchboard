// Package server is the back end of the web inbox: it passes the browser's reads and replies on to the inbox
// service through inboxapi, and serves the built front end. It holds no forge token.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

// Inbox is what the server needs of *inboxapi.Client.
type Inbox interface {
	List(ctx context.Context) (*inboxapi.Inbox, error)
	Issue(ctx context.Context, ref string) (*inboxapi.Detail, error)
	Reply(ctx context.Context, ref, text string, force bool) error
	Instructions(ctx context.Context) (*inboxapi.Instructions, error)
	SaveInstructions(ctx context.Context, target, content string) error
	Events(ctx context.Context, fn func(*inboxapi.Inbox)) error
}

// CodeUnavailable is the error code the web inbox answers with when the inbox service cannot be reached.
const CodeUnavailable = "unavailable"

// Server is the http.Handler of the web inbox.
type Server struct {
	Inbox Inbox
	// Listen is the address served on (web.yaml's listen). A reply is taken only from a page of this origin
	// reached by this host name.
	Listen string
	// Assets is the built front end, with index.html at its root.
	Assets fs.FS
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	switch {
	case r.URL.Path == "/api/inbox" && r.Method == http.MethodGet:
		out, err := s.Inbox.List(r.Context())
		s.answer(w, out, err)
	case r.URL.Path == "/api/issue" && r.Method == http.MethodGet:
		out, err := s.Inbox.Issue(r.Context(), r.URL.Query().Get("ref"))
		s.answer(w, out, err)
	case r.URL.Path == "/api/reply" && r.Method == http.MethodPost:
		s.reply(w, r)
	case r.URL.Path == "/api/events" && r.Method == http.MethodGet:
		s.events(w, r)
	case r.URL.Path == "/api/instructions" && r.Method == http.MethodGet:
		out, err := s.Inbox.Instructions(r.Context())
		s.answer(w, out, err)
	case r.URL.Path == "/api/instructions" && r.Method == http.MethodPost:
		s.saveInstructions(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/"):
		send(w, http.StatusNotFound, inboxapi.Error{Code: inboxapi.CodeNotFound, Message: "no such endpoint"})
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		s.static(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// sameOrigin tells whether a reply or a save comes from the web inbox's own page: a JSON body (a page elsewhere cannot send
// one without a preflight, which this server never answers), an Origin of exactly this address, and a Host of
// exactly this address (a page that rebinds its own name to this address still sends its own name).
func (s *Server) sameOrigin(r *http.Request) (int, string) {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		return http.StatusUnsupportedMediaType, "a reply or a save is sent as application/json"
	}
	if r.Header.Get("Origin") != "http://"+s.Listen || r.Host != s.Listen {
		return http.StatusForbidden, "a reply or a save is taken only from the web inbox's own page at http://" + s.Listen
	}
	return 0, ""
}

func (s *Server) reply(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	var req inboxapi.ReplyRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		send(w, http.StatusBadRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the body is not a reply"})
		return
	}
	// The browser never forces a reply onto an issue that stopped waiting.
	if err := s.Inbox.Reply(r.Context(), req.Ref, req.Text, false); err != nil {
		s.answer(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// saveInstructions passes a save of the plan runs' instructions on, from the web inbox's own page only.
func (s *Server) saveInstructions(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	var req inboxapi.SaveInstructionsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		send(w, http.StatusBadRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the body is not a save of instructions"})
		return
	}
	if err := s.Inbox.SaveInstructions(r.Context(), req.Target, req.Content); err != nil {
		s.answer(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// events passes the inbox service's listings on to the browser as server-sent events (event inbox, the Inbox as
// data). When the service cannot be reached, or its stream breaks, it sends event unavailable and ends: the
// browser's EventSource reconnects after the retry it was given, which it would not do after an error status.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		send(w, http.StatusInternalServerError, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the connection cannot stream"})
		return
	}
	// The stream outlives the server's WriteTimeout, which is for the other answers.
	http.NewResponseController(w).SetWriteDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 5000\n\n")
	fl.Flush()
	err := s.Inbox.Events(r.Context(), func(in *inboxapi.Inbox) {
		data, _ := json.Marshal(in)
		fmt.Fprintf(w, "event: inbox\ndata: %s\n\n", data)
		fl.Flush()
	})
	if r.Context().Err() != nil {
		return
	}
	msg := "the inbox service (switchboard inbox serve) is not reachable"
	if err != nil && !errors.Is(err, inboxapi.ErrUnavailable) {
		msg = err.Error()
	}
	data, _ := json.Marshal(inboxapi.Error{Code: CodeUnavailable, Message: msg})
	fmt.Fprintf(w, "event: unavailable\ndata: %s\n\n", data)
	fl.Flush()
}

var status = map[string]int{
	inboxapi.CodeInvalid:        http.StatusBadRequest,
	inboxapi.CodeNotFound:       http.StatusNotFound,
	inboxapi.CodeNotWaiting:     http.StatusConflict,
	inboxapi.CodeTooLarge:       http.StatusRequestEntityTooLarge,
	inboxapi.CodeSecret:         http.StatusUnprocessableEntity,
	inboxapi.CodeUnknownProject: http.StatusUnprocessableEntity,
	inboxapi.CodeForge:          http.StatusBadGateway,
}

func (s *Server) answer(w http.ResponseWriter, out any, err error) {
	var e *inboxapi.Error
	switch {
	case err == nil:
		send(w, http.StatusOK, out)
	case errors.As(err, &e):
		code := status[e.Code]
		if code == 0 {
			code = http.StatusBadGateway
		}
		send(w, code, e)
	case errors.Is(err, inboxapi.ErrUnavailable):
		send(w, http.StatusServiceUnavailable, inboxapi.Error{Code: CodeUnavailable, Message: "the inbox service (switchboard inbox serve) is not reachable"})
	default:
		send(w, http.StatusBadGateway, inboxapi.Error{Code: inboxapi.CodeForge, Message: err.Error()})
	}
}

// static serves a file of the front end, and index.html for every other path: the front end routes them.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name != "" && name != "index.html" {
		if st, err := fs.Stat(s.Assets, name); err == nil && !st.IsDir() {
			http.ServeFileFS(w, r, s.Assets, name)
			return
		}
	}
	page, err := fs.ReadFile(s.Assets, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("The front end is not built: run pnpm build in web/ui, then build switchboard-web again.\n"))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(page)
}

func send(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
