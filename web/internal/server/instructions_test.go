package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

func (f *fakeInbox) Instructions(context.Context) (*inboxapi.Instructions, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &inboxapi.Instructions{Host: "ubuntu-a", Items: []inboxapi.Instruction{{Target: ""}, {Target: "gh:o/r", Content: "Mind the API."}}}, nil
}

func (f *fakeInbox) SaveInstructions(_ context.Context, target, content string) error {
	if f.err != nil {
		return f.err
	}
	f.saved = append(f.saved, inboxapi.SaveInstructionsRequest{Target: target, Content: content})
	return nil
}

func TestInstructionsAreRead(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/api/instructions", "", nil)
	var got inboxapi.Instructions
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Host != "ubuntu-a" || len(got.Items) != 2 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
}

func TestInstructionsAreSaved(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/instructions", `{"target":"gh:o/r","content":"Ask in Japanese."}`, good)
	if w.Code != http.StatusNoContent || len(f.saved) != 1 || f.saved[0].Target != "gh:o/r" || f.saved[0].Content != "Ask in Japanese." {
		t.Fatalf("status = %d, saved = %+v", w.Code, f.saved)
	}
}

func TestSaveOfInstructionsFromElsewhereIsRefused(t *testing.T) {
	for name, h := range map[string]map[string]string{
		"other origin":  {"Content-Type": "application/json", "Origin": "https://evil.example"},
		"no origin":     {"Content-Type": "application/json"},
		"rebound host":  {"Content-Type": "application/json", "Origin": "http://" + listen, "Host": "evil.example:8783"},
		"form encoding": {"Content-Type": "application/x-www-form-urlencoded", "Origin": "http://" + listen},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInbox{}
			w := do(newServer(f), "POST", "/api/instructions", `{"target":"","content":"x"}`, h)
			if w.Code != http.StatusForbidden && w.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("status = %d", w.Code)
			}
			if len(f.saved) != 0 {
				t.Fatal("the save was passed on")
			}
		})
	}
}

func TestRefusedSavesKeepTheirCode(t *testing.T) {
	for code, want := range map[string]int{
		inboxapi.CodeTooLarge:       http.StatusRequestEntityTooLarge,
		inboxapi.CodeSecret:         http.StatusUnprocessableEntity,
		inboxapi.CodeUnknownProject: http.StatusUnprocessableEntity,
	} {
		f := &fakeInbox{err: &inboxapi.Error{Code: code, Message: code}}
		w := do(newServer(f), "POST", "/api/instructions", `{"target":"gh:o/r","content":"x"}`, good)
		var e inboxapi.Error
		if w.Code != want || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != code {
			t.Errorf("%s: status = %d, body = %s", code, w.Code, w.Body)
		}
	}
}
