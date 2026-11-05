package inboxserve

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

// readIssue asks the server for the detail of gh:o/r#12 and tells whether the forges were then asked to be read again.
func readIssue(t *testing.T, w *world) bool {
	t.Helper()
	rec := httptest.NewRecorder()
	w.srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/issue?ref=gh:o/r%2312", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	select {
	case <-w.srv.refresh:
		return true
	default:
		return false
	}
}

func TestIssueReadsTheListAgainWhenItsStateDiffers(t *testing.T) {
	w := newServer(t)
	w.srv.refresh = make(chan struct{}, 1)
	w.srv.last = &inboxapi.Inbox{Items: []inboxapi.Item{{Ref: "gh:o/r#12", State: "queued"}}}
	if !readIssue(t, w) {
		t.Error("a queued row of a waiting issue: the forges were not asked to be read again")
	}
}

func TestIssueLeavesTheListWhenItsStateIsTheSame(t *testing.T) {
	w := newServer(t)
	w.srv.refresh = make(chan struct{}, 1)
	w.srv.last = &inboxapi.Inbox{Items: []inboxapi.Item{{Ref: "gh:o/r#12", State: "waiting"}}}
	if readIssue(t, w) {
		t.Error("a waiting row of a waiting issue: the forges were asked to be read again")
	}
}

func TestIssueReadsTheListAgainWhenItHasNoRow(t *testing.T) {
	w := newServer(t)
	w.srv.refresh = make(chan struct{}, 1)
	w.srv.last = &inboxapi.Inbox{Items: []inboxapi.Item{{Ref: "gh:o/r#99", State: "waiting"}}}
	if !readIssue(t, w) {
		t.Error("no row of a waiting issue: the forges were not asked to be read again")
	}
}

func TestIssueReadsTheListAgainWhenTheIssueLeftIt(t *testing.T) {
	w := newServer(t)
	w.f.RemoveLabel(w.ref, "agent:waiting")
	w.srv.refresh = make(chan struct{}, 1)
	w.srv.last = &inboxapi.Inbox{Items: []inboxapi.Item{{Ref: "gh:o/r#12", State: "waiting"}}}
	if !readIssue(t, w) {
		t.Error("a row of an issue no longer listed: the forges were not asked to be read again")
	}
}

func TestIssueLeavesTheListWhenNoneWasSent(t *testing.T) {
	w := newServer(t)
	w.srv.refresh = make(chan struct{}, 1)
	if readIssue(t, w) {
		t.Error("no listing sent: the forges were asked to be read again")
	}
}
