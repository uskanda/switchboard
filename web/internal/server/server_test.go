package server

import (
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
	replies []inboxapi.ReplyRequest
	saved   []inboxapi.SaveInstructionsRequest
	err     error
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
func (f *fakeInbox) Events(context.Context, func(*inboxapi.Inbox)) error { return inboxapi.ErrUnavailable }

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

func TestWithoutABuiltFrontend(t *testing.T) {
	s := &Server{Inbox: &fakeInbox{}, Listen: listen, Assets: fstest.MapFS{".gitkeep": {}}}
	w := do(s, "GET", "/", "", nil)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "pnpm build") {
		t.Fatalf("status = %d %s", w.Code, w.Body)
	}
}
