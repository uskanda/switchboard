// Package server is the back end of the web inbox: it passes the browser's reads and replies on to the inbox
// service through inboxapi, and serves the built front end. It holds no forge token.
package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

// Inbox is what the server needs of *inboxapi.Client.
type Inbox interface {
	List(ctx context.Context) (*inboxapi.Inbox, error)
	Issue(ctx context.Context, ref string) (*inboxapi.Detail, error)
	Reply(ctx context.Context, ref, text string, force bool) error
	Instructions(ctx context.Context) (*inboxapi.Instructions, error)
	SaveInstructions(ctx context.Context, req inboxapi.SaveInstructionsRequest) error
	Projects(ctx context.Context) (*inboxapi.Projects, error)
	AddProject(ctx context.Context, req inboxapi.AddProjectRequest) (*inboxapi.AddProjectResponse, error)
	IssueTargets(ctx context.Context) (*inboxapi.IssueTargets, error)
	CreateIssue(ctx context.Context, req inboxapi.CreateIssueRequest) (*inboxapi.CreateIssueResponse, error)
	Merge(ctx context.Context, ref, sha string) (*inboxapi.MergeResult, error)
	CancelAutoMerge(ctx context.Context, ref string) error
	ProjectIcon(ctx context.Context, ref string) (*inboxapi.Icon, error)
	ProjectIcons(ctx context.Context) (inboxapi.ProjectIcons, error)
	RegisterDevice(ctx context.Context, token string) error
	Approve(ctx context.Context, ref string, since time.Time) error
	MergeStatus(ctx context.Context, ref, sha string) (*inboxapi.MergeStatus, error)
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

	filesMu sync.Mutex
	files   map[string]*staticFile
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	// Every request, reads included, must name this address as its Host: a page that rebinds its own name to this
	// address (DNS rebinding) still sends its own name, and must not read the inbox.
	if r.Host != s.Listen {
		send(w, http.StatusMisdirectedRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the web inbox answers only at http://" + s.Listen})
		return
	}
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
	case r.URL.Path == "/api/projects" && r.Method == http.MethodGet:
		out, err := s.Inbox.Projects(r.Context())
		s.answer(w, out, err)
	case r.URL.Path == "/api/projects" && r.Method == http.MethodPost:
		s.addProject(w, r)
	case r.URL.Path == "/api/issue-targets" && r.Method == http.MethodGet:
		out, err := s.Inbox.IssueTargets(r.Context())
		s.answer(w, out, err)
	case r.URL.Path == "/api/issues" && r.Method == http.MethodPost:
		s.createIssue(w, r)
	case r.URL.Path == "/api/merge" && r.Method == http.MethodPost:
		s.merge(w, r)
	case r.URL.Path == "/api/project-icon" && r.Method == http.MethodGet:
		s.projectIcon(w, r)
	case r.URL.Path == "/api/project-icons" && r.Method == http.MethodGet:
		out, err := s.Inbox.ProjectIcons(r.Context())
		s.answer(w, out, err)
	case r.URL.Path == "/api/merge-status" && r.Method == http.MethodGet:
		// It only reads, as /api/issue does; the merge itself is a POST with the checks of a reply.
		q := r.URL.Query()
		out, err := s.Inbox.MergeStatus(r.Context(), q.Get("ref"), q.Get("sha"))
		s.answer(w, out, err)
	case r.URL.Path == "/api/automerge/cancel" && r.Method == http.MethodPost:
		s.cancelAutoMerge(w, r)
	case r.URL.Path == "/api/devices" && r.Method == http.MethodPost:
		s.registerDevice(w, r)
	case r.URL.Path == "/api/approve" && r.Method == http.MethodPost:
		s.approve(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/"):
		send(w, http.StatusNotFound, inboxapi.Error{Code: inboxapi.CodeNotFound, Message: "no such endpoint"})
	case r.Method == http.MethodGet || r.Method == http.MethodHead:
		s.static(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// sameOrigin tells whether a reply, a save, an addition, a new issue, a merge, an app's registration or an approval
// comes from the web inbox's own page or the app, which sends this Origin itself: a JSON body (a page elsewhere cannot send
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

// saveInstructions passes a save of instructions (of the plan runs or the runs that write files) on, from the web
// inbox's own page only.
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
	if err := s.Inbox.SaveInstructions(r.Context(), req); err != nil {
		s.answer(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addProject passes the addition of a project on, from the web inbox's own page only.
func (s *Server) addProject(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	var req inboxapi.AddProjectRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		send(w, http.StatusBadRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the body is not a project to add"})
		return
	}
	out, err := s.Inbox.AddProject(r.Context(), req)
	if err != nil {
		s.answer(w, nil, err)
		return
	}
	send(w, http.StatusCreated, out)
}

// createIssue passes the opening of an issue on, from the web inbox's own page only. The issue is the person's own
// and starts a run (ADR-040), so it gets the same checks as a reply.
func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	var req inboxapi.CreateIssueRequest
	// JSON may escape every byte of the body six times over; the inbox service measures the body itself.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 6*inboxapi.MaxIssueBody+1<<16)).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			send(w, http.StatusRequestEntityTooLarge, inboxapi.Error{Code: inboxapi.CodeTooLarge, Message: "the issue is too large"})
			return
		}
		send(w, http.StatusBadRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the body is not an issue to open"})
		return
	}
	out, err := s.Inbox.CreateIssue(r.Context(), req)
	if err != nil {
		s.answer(w, nil, err)
		return
	}
	send(w, http.StatusCreated, out)
}

// merge passes the merge of a done issue's change request on, from the web inbox's own page only. The merge cannot be
// undone and is made with the inbox token (ADR-043), so it gets the same checks as a reply.
func (s *Server) merge(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	var req inboxapi.MergeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		send(w, http.StatusBadRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the body is not a merge"})
		return
	}
	res, err := s.Inbox.Merge(r.Context(), req.Ref, req.SHA)
	if err != nil {
		s.answer(w, nil, err)
		return
	}
	if res.AutoMerge {
		// The forge refused it for a conflict: the inbox service merges it once the agent has fixed it (ADR-048).
		send(w, http.StatusAccepted, inboxapi.MergeAnswer{Code: inboxapi.CodeAutoMergePending,
			Message: "the change request conflicts with its base; it is merged once the agent has fixed the conflict"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// cancelAutoMerge passes the cancel of a merge waiting for the fix of a conflict on, from the web inbox's own page
// only: it gets the same checks as a merge.
func (s *Server) cancelAutoMerge(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	var req inboxapi.CancelAutoMergeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		send(w, http.StatusBadRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the body is not a cancel of an automatic merge"})
		return
	}
	if err := s.Inbox.CancelAutoMerge(r.Context(), req.Ref); err != nil {
		s.answer(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// projectIcon passes a project's icon image on from the inbox service, which read it with the inbox token (ADR-055).
// The page reads it from this origin, so the content security policy stays default-src 'self'; nosniff, set for
// every answer, keeps the browser to the image type the inbox service checked.
func (s *Server) projectIcon(w http.ResponseWriter, r *http.Request) {
	icon, err := s.Inbox.ProjectIcon(r.Context(), r.URL.Query().Get("ref"))
	if err != nil {
		s.answer(w, nil, err)
		return
	}
	w.Header().Set("Content-Type", icon.ContentType)
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Write(icon.Data)
}

// registerDevice passes an app's FCM token on to the inbox service.
func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	var req inboxapi.RegisterDeviceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		send(w, http.StatusBadRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the body is not a device registration"})
		return
	}
	if err := s.Inbox.RegisterDevice(r.Context(), req.Token); err != nil {
		s.answer(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// approve passes the approval of a plan, pressed on a notification, on to the inbox service.
func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	if code, why := s.sameOrigin(r); code != 0 {
		send(w, code, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: why})
		return
	}
	var req inboxapi.ApproveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		send(w, http.StatusBadRequest, inboxapi.Error{Code: inboxapi.CodeInvalid, Message: "the body is not an approval"})
		return
	}
	if err := s.Inbox.Approve(r.Context(), req.Ref, req.Since); err != nil {
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
	inboxapi.CodeStale:          http.StatusConflict,
	inboxapi.CodeNotMergeable:   http.StatusConflict,
	inboxapi.CodeForbidden:      http.StatusForbidden,
	inboxapi.CodeTooLarge:       http.StatusRequestEntityTooLarge,
	inboxapi.CodeSecret:         http.StatusUnprocessableEntity,
	inboxapi.CodeUnknownProject: http.StatusUnprocessableEntity,
	inboxapi.CodeForge:          http.StatusBadGateway,
	inboxapi.CodeAddingDisabled: http.StatusForbidden,
	inboxapi.CodeProjectExists:  http.StatusConflict,
	inboxapi.CodeForgeNotUsed:   http.StatusUnprocessableEntity,
	inboxapi.CodeRepoUnreadable: http.StatusUnprocessableEntity,
	inboxapi.CodePathTaken:      http.StatusConflict,
	inboxapi.CodeConfigInvalid:  http.StatusUnprocessableEntity,
	inboxapi.CodeConfigChanged:  http.StatusConflict,
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
// staticFile is what is served for one file of the front end, made on its first request and kept.
type staticFile struct {
	data, gz    []byte // gz is nil when the file is not worth compressing
	contentType string
	etag        string // of data; the gzip version's is etag with "-gz" before the closing quote
}

// compressible tells whether a type is text that gzip shrinks; images and fonts are compressed already.
func compressible(contentType string) bool {
	mt, _, _ := mime.ParseMediaType(contentType)
	switch mt {
	case "application/javascript", "text/javascript", "application/json", "application/manifest+json", "image/svg+xml":
		return true
	}
	return strings.HasPrefix(mt, "text/")
}

func newStaticFile(name string, data []byte) *staticFile {
	f := &staticFile{data: data, contentType: mime.TypeByExtension(path.Ext(name))}
	if f.contentType == "" {
		f.contentType = http.DetectContentType(data)
	}
	sum := sha256.Sum256(data)
	f.etag = `"` + hex.EncodeToString(sum[:16]) + `"`
	if compressible(f.contentType) {
		var b bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&b, gzip.BestCompression)
		zw.Write(data)
		zw.Close()
		if b.Len() < len(data) {
			f.gz = b.Bytes()
		}
	}
	return f
}

// file gives the kept staticFile of name, reading it on the first request. Assets are never replaced while the
// server runs, so what is kept is never dropped.
func (s *Server) file(name string) (*staticFile, error) {
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	if f, ok := s.files[name]; ok {
		return f, nil
	}
	data, err := fs.ReadFile(s.Assets, name)
	if err != nil {
		return nil, err
	}
	f := newStaticFile(name, data)
	if s.files == nil {
		s.files = map[string]*staticFile{}
	}
	s.files[name] = f
	return f, nil
}

// acceptsGzip tells whether the request's Accept-Encoding takes gzip with a q above 0.
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(coding), "gzip") {
			continue
		}
		for _, p := range strings.Split(params, ";") {
			k, v, _ := strings.Cut(strings.TrimSpace(p), "=")
			if strings.EqualFold(strings.TrimSpace(k), "q") {
				if q, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && q <= 0 {
					return false
				}
			}
		}
		return true
	}
	return false
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	var f *staticFile
	if st, err := fs.Stat(s.Assets, name); err == nil && !st.IsDir() {
		f, err = s.file(name)
		if err != nil {
			f = nil
		}
	}
	if f == nil {
		// Any other path is a page of the front end, which index.html routes.
		name = "index.html"
		var err error
		if f, err = s.file(name); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("The front end is not built: run pnpm build in web/ui, then build switchboard-web again.\n"))
			return
		}
	}
	h := w.Header()
	h.Set("Content-Type", f.contentType)
	if strings.HasPrefix(name, "assets/") {
		// Vite puts a hash of the content in the names under assets/.
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "no-cache")
	}
	data, etag := f.data, f.etag
	if f.gz != nil {
		h.Add("Vary", "Accept-Encoding")
		if acceptsGzip(r) {
			data, etag = f.gz, strings.TrimSuffix(f.etag, `"`)+`-gz"`
			h.Set("Content-Encoding", "gzip")
			// A range of the gzip bytes means nothing to the browser: send the whole of it.
			r.Header.Del("Range")
		}
	}
	h.Set("ETag", etag)
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

func send(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
