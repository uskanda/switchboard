package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

func (f *fakeInbox) Merge(_ context.Context, ref, sha string) error {
	if f.err != nil {
		return f.err
	}
	f.merged = append(f.merged, inboxapi.MergeRequest{Ref: ref, SHA: sha})
	return nil
}

func TestMergeIsPassedOn(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/merge", `{"ref":"gh:o/r#50","sha":"abc"}`, good)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	if len(f.merged) != 1 || f.merged[0] != (inboxapi.MergeRequest{Ref: "gh:o/r#50", SHA: "abc"}) {
		t.Fatalf("merged = %+v", f.merged)
	}
}

func TestMergingFromElsewhereIsRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		h    map[string]string
		want int
	}{
		"other origin":  {map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, http.StatusForbidden},
		"no origin":     {map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
		"rebound host":  {map[string]string{"Content-Type": "application/json", "Origin": "http://" + listen, "Host": "evil.example:8783"}, http.StatusForbidden},
		"form encoding": {map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Origin": "http://" + listen}, http.StatusUnsupportedMediaType},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInbox{}
			w := do(newServer(f), "POST", "/api/merge", `{"ref":"gh:o/r#50","sha":"abc"}`, tc.h)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if len(f.merged) != 0 {
				t.Fatal("the merge was passed on")
			}
		})
	}
}

func TestMergeIsOnlyPosted(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/api/merge", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestRefusedMergesKeepTheirCode(t *testing.T) {
	for code, want := range map[string]int{
		inboxapi.CodeStale:        http.StatusConflict,
		inboxapi.CodeNotMergeable: http.StatusConflict,
		inboxapi.CodeForbidden:    http.StatusForbidden,
		inboxapi.CodeInvalid:      http.StatusBadRequest,
	} {
		f := &fakeInbox{err: &inboxapi.Error{Code: code, Message: code}}
		w := do(newServer(f), "POST", "/api/merge", `{"ref":"gh:o/r#50","sha":"abc"}`, good)
		var e inboxapi.Error
		if w.Code != want || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != code {
			t.Errorf("%s: status = %d, body = %s", code, w.Code, w.Body)
		}
	}
}
