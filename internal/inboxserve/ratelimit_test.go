package inboxserve

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
)

// getInbox asks the server for the listing, as GET /v1/inbox does, without a socket.
func getInbox(t *testing.T, w *world) inboxapi.Inbox {
	t.Helper()
	rec := httptest.NewRecorder()
	w.srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/inbox", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var in inboxapi.Inbox
	if err := json.Unmarshal(rec.Body.Bytes(), &in); err != nil {
		t.Fatal(err)
	}
	return in
}

// A forge that answered with a rate limit has retry_at in its error, and the next request within the minute does
// not ask it again; the listing keeps its last rows.
func TestRateLimitedForgeHasRetryAtAndIsNotAskedAgain(t *testing.T) {
	w := newServer(t)
	first := getInbox(t, w)
	if len(first.Items) != 1 {
		t.Fatalf("items = %+v", first.Items)
	}
	reset := w.now.Add(20 * time.Minute)
	w.now = w.now.Add(2 * time.Minute) // past the cache
	w.gh.err = &forge.RateLimitError{Reset: reset, Err: forge.ErrForbidden}
	got := getInbox(t, w)
	if len(got.Errors) != 1 || got.Errors[0].Forge != "github" || got.Errors[0].RetryAt == nil || !got.Errors[0].RetryAt.Equal(reset) {
		t.Fatalf("errors = %+v, want github with retry_at %v", got.Errors, reset)
	}
	if len(got.Items) != 1 || got.Items[0].Ref != "gh:o/r#12" {
		t.Fatalf("items = %+v, want the last row", got.Items)
	}

	searches := w.gh.searches.Load()
	w.now = w.now.Add(30 * time.Second)
	again := getInbox(t, w)
	if n := w.gh.searches.Load(); n != searches {
		t.Fatalf("searches %d -> %d while limited", searches, n)
	}
	if len(again.Errors) != 1 || again.Errors[0].RetryAt == nil || len(again.Items) != 1 {
		t.Fatalf("listing while limited = %+v", again)
	}
}

// An error that is no rate limit has no retry_at.
func TestPlainErrorHasNoRetryAt(t *testing.T) {
	w := newServer(t)
	w.gl.err = errors.New("connection refused")
	rec := httptest.NewRecorder()
	w.srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/inbox", nil))
	var raw struct {
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Errors) != 1 {
		t.Fatalf("errors = %+v", raw.Errors)
	}
	if _, ok := raw.Errors[0]["retry_at"]; ok {
		t.Fatalf("error = %+v, want no retry_at", raw.Errors[0])
	}
}
