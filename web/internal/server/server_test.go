package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/uskanda/switchboard/inboxapi"
)

const listen = "192.168.0.82:8783"

// fakeInbox stands in for the inbox service.
type fakeInbox struct {
	replies  []inboxapi.ReplyRequest
	saved    []inboxapi.SaveInstructionsRequest
	added    []inboxapi.AddProjectRequest
	created  []inboxapi.CreateIssueRequest
	merged   []inboxapi.MergeRequest
	devices  []string
	approved []inboxapi.ApproveRequest
	// later makes a merge wait for the fix of a conflict; cancelled lists the cancels of such merges.
	later     bool
	cancelled []string
	// statusOf are the issues and commits whose merge status was asked for.
	statusOf []inboxapi.MergeRequest
	err      error
}

func (f *fakeInbox) List(context.Context) (*inboxapi.Inbox, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &inboxapi.Inbox{Items: []inboxapi.Item{{N: 1, Ref: "gh:o/r#12", Title: "<script>x</script>"}}}, nil
}

func (f *fakeInbox) Issue(_ context.Context, ref string) (*inboxapi.Detail, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &inboxapi.Detail{Ref: ref, Waiting: true}, nil
}

func (f *fakeInbox) Reply(_ context.Context, ref, text string, force bool) error {
	if f.err != nil {
		return f.err
	}
	f.replies = append(f.replies, inboxapi.ReplyRequest{Ref: ref, Text: text, Force: force})
	return nil
}

// Events has no stream to give: the tests of the event stream use streamInbox.
func (f *fakeInbox) Events(context.Context, func(*inboxapi.Inbox)) error {
	return inboxapi.ErrUnavailable
}

var assets = fstest.MapFS{
	"index.html":    {Data: []byte("<!doctype html><title>inbox</title>")},
	"assets/app.js": {Data: []byte("console.log(1)")},
}

func newServer(f *fakeInbox) *Server { return &Server{Inbox: f, Listen: listen, Assets: assets} }

func do(s http.Handler, method, target, body string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = listen
	for k, v := range header {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	return w
}

var good = map[string]string{"Content-Type": "application/json", "Origin": "http://" + listen}

func TestReplyIsPassedOn(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/reply", `{"ref":"gh:o/r#12","text":"Go with B.","force":true}`, good)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d %s", w.Code, w.Body)
	}
	// force is never taken from the browser.
	if len(f.replies) != 1 || f.replies[0] != (inboxapi.ReplyRequest{Ref: "gh:o/r#12", Text: "Go with B."}) {
		t.Fatalf("replies = %+v", f.replies)
	}
}

func TestReplyFromElsewhereIsRefused(t *testing.T) {
	for name, h := range map[string]map[string]string{
		"other origin":  {"Content-Type": "application/json", "Origin": "https://evil.example"},
		"no origin":     {"Content-Type": "application/json"},
		"other host":    {"Content-Type": "application/json", "Origin": "http://evil.example:8783", "Host": "evil.example:8783"},
		"rebound host":  {"Content-Type": "application/json", "Origin": "http://" + listen, "Host": "evil.example:8783"},
		"null origin":   {"Content-Type": "application/json", "Origin": "null"},
		"form encoding": {"Content-Type": "application/x-www-form-urlencoded", "Origin": "http://" + listen},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInbox{}
			w := do(newServer(f), "POST", "/api/reply", `{"ref":"gh:o/r#12","text":"x"}`, h)
			if w.Code != http.StatusForbidden && w.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("status = %d", w.Code)
			}
			if len(f.replies) != 0 {
				t.Fatal("the reply was passed on")
			}
		})
	}
}

func TestReadsAreServed(t *testing.T) {
	s := newServer(&fakeInbox{})
	w := do(s, "GET", "/api/inbox", "", nil)
	var got inboxapi.Inbox
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Items[0].Ref != "gh:o/r#12" {
		t.Fatalf("inbox: %d %s", w.Code, w.Body)
	}
	w = do(s, "GET", "/api/issue?ref="+"gl%3Anas%2Fsb%236", "", nil)
	var d inboxapi.Detail
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &d) != nil || d.Ref != "gl:nas/sb#6" {
		t.Fatalf("issue: %d %s", w.Code, w.Body)
	}
}

func TestErrorsOfTheInboxServiceArePassedOn(t *testing.T) {
	w := do(newServer(&fakeInbox{err: &inboxapi.Error{Code: inboxapi.CodeNotWaiting, Message: "not waiting"}}), "POST", "/api/reply", `{"ref":"gh:o/r#12","text":"x"}`, good)
	var e inboxapi.Error
	if w.Code != http.StatusConflict || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != inboxapi.CodeNotWaiting {
		t.Fatalf("not waiting: %d %s", w.Code, w.Body)
	}
}

func TestUnreachableInboxService(t *testing.T) {
	c := &inboxapi.Client{Socket: filepath.Join(t.TempDir(), "none.sock")}
	w := do(&Server{Inbox: c, Listen: listen, Assets: assets}, "GET", "/api/inbox", "", nil)
	var e inboxapi.Error
	if w.Code != http.StatusServiceUnavailable || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != CodeUnavailable {
		t.Fatalf("status = %d %s", w.Code, w.Body)
	}
}

func TestPagesAndHeaders(t *testing.T) {
	s := newServer(&fakeInbox{})
	for _, path := range []string{"/", "/issue/gh:o%2Fr%2312"} {
		w := do(s, "GET", path, "", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "<title>inbox</title>") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s headers: %v", path, w.Header())
		}
	}
	w := do(s, "GET", "/assets/app.js", "", nil)
	if b, _ := io.ReadAll(w.Body); w.Code != 200 || string(b) != "console.log(1)" {
		t.Fatalf("asset: %d %s", w.Code, b)
	}
	if w := do(s, "GET", "/api/nothing", "", nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown api: %d", w.Code)
	}
}

// The icons and the manifest are served as files of their own type, so a browser takes them under nosniff, and
// are not replaced by index.html.
func TestIconsAndManifest(t *testing.T) {
	s := &Server{Inbox: &fakeInbox{}, Listen: listen, Assets: fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>inbox</title>")},
		"manifest.json": {Data: []byte(`{"name":"SwitchBoard Inbox"}`)},
		"favicon.png":   {Data: []byte("\x89PNG\r\n\x1a\n")},
	}}
	for path, want := range map[string]string{"/manifest.json": "application/json", "/favicon.png": "image/png"} {
		w := do(s, "GET", path, "", nil)
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), want) || strings.Contains(w.Body.String(), "<title>") {
			t.Fatalf("%s: %d %q %s", path, w.Code, w.Header().Get("Content-Type"), w.Body)
		}
	}
}

// bigJS is an asset big enough for gzip to shrink.
var bigJS = strings.Repeat("console.log('switchboard inbox');\n", 200)

var textAssets = fstest.MapFS{
	"index.html":      {Data: []byte("<!doctype html><title>inbox</title>")},
	"assets/app-1.js": {Data: []byte(bigJS)},
	"assets/logo.png": {Data: []byte(strings.Repeat("\x89PNG", 200))},
	"favicon.png":     {Data: []byte("\x89PNG\r\n\x1a\n")},
}

func gunzip(t *testing.T, b []byte) string {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	return string(out)
}

func TestAssetsAreGzipped(t *testing.T) {
	s := &Server{Inbox: &fakeInbox{}, Listen: listen, Assets: textAssets}
	w := do(s, "GET", "/assets/app-1.js", "", map[string]string{"Accept-Encoding": "gzip, deflate, br"})
	h := w.Header()
	if w.Code != 200 || h.Get("Content-Encoding") != "gzip" || h.Get("Vary") != "Accept-Encoding" {
		t.Fatalf("gzip: %d %v", w.Code, h)
	}
	if !strings.HasPrefix(h.Get("Content-Type"), "text/javascript") {
		t.Fatalf("content type: %q", h.Get("Content-Type"))
	}
	if w.Body.Len() >= len(bigJS) || gunzip(t, w.Body.Bytes()) != bigJS {
		t.Fatalf("gzip body: %d bytes", w.Body.Len())
	}

	for _, ae := range []string{"", "gzip;q=0", "identity"} {
		w = do(s, "GET", "/assets/app-1.js", "", map[string]string{"Accept-Encoding": ae})
		if w.Code != 200 || w.Header().Get("Content-Encoding") != "" || w.Body.String() != bigJS || w.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatalf("Accept-Encoding %q: %d %v", ae, w.Code, w.Header())
		}
	}

	w = do(s, "GET", "/assets/logo.png", "", map[string]string{"Accept-Encoding": "gzip"})
	if w.Code != 200 || w.Header().Get("Content-Encoding") != "" || w.Body.String() != strings.Repeat("\x89PNG", 200) {
		t.Fatalf("png: %d %v", w.Code, w.Header())
	}

	// A range of a gzipped file is not taken: the whole gzip comes back.
	w = do(s, "GET", "/assets/app-1.js", "", map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-9"})
	if w.Code != 200 || gunzip(t, w.Body.Bytes()) != bigJS {
		t.Fatalf("range on gzip: %d", w.Code)
	}
}

func TestCacheHeaders(t *testing.T) {
	s := &Server{Inbox: &fakeInbox{}, Listen: listen, Assets: textAssets}
	for _, p := range []string{"/assets/app-1.js", "/assets/logo.png"} {
		if cc := do(s, "GET", p, "", nil).Header().Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
			t.Fatalf("%s: Cache-Control %q", p, cc)
		}
	}
	for _, p := range []string{"/", "/index.html", "/issue/x", "/favicon.png"} {
		w := do(s, "GET", p, "", nil)
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-cache" || w.Header().Get("ETag") == "" {
			t.Fatalf("%s: %d %v", p, w.Code, w.Header())
		}
	}
	if w := do(s, "GET", "/issue/x", "", nil); !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") || !strings.Contains(w.Body.String(), "<title>inbox</title>") {
		t.Fatalf("fallback: %v %s", w.Header(), w.Body)
	}
}

func TestETagAndNotModified(t *testing.T) {
	s := &Server{Inbox: &fakeInbox{}, Listen: listen, Assets: textAssets}
	for _, c := range []struct{ path, ae string }{{"/", ""}, {"/issue/x", "gzip"}, {"/assets/app-1.js", ""}, {"/assets/app-1.js", "gzip"}} {
		h := map[string]string{"Accept-Encoding": c.ae}
		first := do(s, "GET", c.path, "", h)
		tag := first.Header().Get("ETag")
		if tag == "" {
			t.Fatalf("%s %q: no ETag", c.path, c.ae)
		}
		again := do(s, "GET", c.path, "", h)
		if again.Header().Get("ETag") != tag || !bytes.Equal(again.Body.Bytes(), first.Body.Bytes()) {
			t.Fatalf("%s %q: second answer differs", c.path, c.ae)
		}
		h["If-None-Match"] = tag
		w := do(s, "GET", c.path, "", h)
		if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
			t.Fatalf("%s %q: If-None-Match: %d %d bytes", c.path, c.ae, w.Code, w.Body.Len())
		}
	}
	plain := do(s, "GET", "/assets/app-1.js", "", nil).Header().Get("ETag")
	gz := do(s, "GET", "/assets/app-1.js", "", map[string]string{"Accept-Encoding": "gzip"}).Header().Get("ETag")
	if plain == gz {
		t.Fatalf("the gzip version has the same ETag %s", plain)
	}
}

func TestHeadOfStaticFiles(t *testing.T) {
	s := &Server{Inbox: &fakeInbox{}, Listen: listen, Assets: textAssets}
	for _, p := range []string{"/", "/assets/app-1.js"} {
		h := map[string]string{"Accept-Encoding": "gzip"}
		get := do(s, "GET", p, "", h)
		head := do(s, "HEAD", p, "", h)
		if head.Code != 200 || head.Body.Len() != 0 {
			t.Fatalf("HEAD %s: %d %d bytes", p, head.Code, head.Body.Len())
		}
		for _, k := range []string{"Content-Type", "Content-Encoding", "Cache-Control", "ETag", "Vary", "Content-Length"} {
			if head.Header().Get(k) != get.Header().Get(k) {
				t.Fatalf("HEAD %s: %s %q, GET has %q", p, k, head.Header().Get(k), get.Header().Get(k))
			}
		}
	}
}

func TestWithoutABuiltFrontend(t *testing.T) {
	s := &Server{Inbox: &fakeInbox{}, Listen: listen, Assets: fstest.MapFS{".gitkeep": {}}}
	w := do(s, "GET", "/", "", nil)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "pnpm build") {
		t.Fatalf("status = %d %s", w.Code, w.Body)
	}
}
