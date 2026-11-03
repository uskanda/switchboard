package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

func (f *fakeInbox) MergeStatus(_ context.Context, ref, sha string) (*inboxapi.MergeStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.statusOf = append(f.statusOf, inboxapi.MergeRequest{Ref: ref, SHA: sha})
	return &inboxapi.MergeStatus{State: "open", Mergeable: inboxapi.MergeConflict, CI: inboxapi.CIRead,
		Checks: []inboxapi.CheckStatus{{Name: "<b>lint</b>", Status: inboxapi.CheckFailure, URL: "https://ci/2"}}}, nil
}

func TestMergeStatusIsPassedOn(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "GET", "/api/merge-status?ref=gh%3Ao%2Fr%2350&sha=abc", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	if len(f.statusOf) != 1 || f.statusOf[0] != (inboxapi.MergeRequest{Ref: "gh:o/r#50", SHA: "abc"}) {
		t.Fatalf("asked = %+v", f.statusOf)
	}
	var st inboxapi.MergeStatus
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil || st.Mergeable != inboxapi.MergeConflict || len(st.Checks) != 1 ||
		st.Checks[0].Name != "<b>lint</b>" {
		t.Fatalf("status = %+v, %v", st, err)
	}
}

func TestMergeStatusIsOnlyRead(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/merge-status?ref=gh%3Ao%2Fr%2350&sha=abc", "{}", good)
	if w.Code != http.StatusNotFound || len(f.statusOf) != 0 {
		t.Fatalf("status = %d, asked = %+v", w.Code, f.statusOf)
	}
}

func TestMergeStatusKeepsTheCodeOfARefusal(t *testing.T) {
	f := &fakeInbox{err: &inboxapi.Error{Code: inboxapi.CodeStale, Message: "open it again"}}
	w := do(newServer(f), "GET", "/api/merge-status?ref=gh%3Ao%2Fr%2350&sha=old", "", nil)
	var e inboxapi.Error
	if w.Code != http.StatusConflict || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != inboxapi.CodeStale {
		t.Fatalf("status = %d %s", w.Code, w.Body)
	}
}

func TestMergeStatusWithoutTheInboxService(t *testing.T) {
	c := &inboxapi.Client{Socket: filepath.Join(t.TempDir(), "none.sock")}
	w := do(&Server{Inbox: c, Listen: listen, Assets: assets}, "GET", "/api/merge-status?ref=gh%3Ao%2Fr%2350&sha=abc", "", nil)
	var e inboxapi.Error
	if w.Code != http.StatusServiceUnavailable || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != CodeUnavailable {
		t.Fatalf("status = %d %s", w.Code, w.Body)
	}
}
