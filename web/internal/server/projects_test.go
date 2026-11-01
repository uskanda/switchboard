package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

func (f *fakeInbox) Projects(context.Context) (*inboxapi.Projects, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &inboxapi.Projects{Host: "ubuntu-a", Enabled: true, Items: []inboxapi.Project{{Ref: "gh:o/r", Cloned: true, MissingLabels: []string{}}}}, nil
}

func (f *fakeInbox) AddProject(_ context.Context, req inboxapi.AddProjectRequest) (*inboxapi.AddProjectResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.added = append(f.added, req)
	return &inboxapi.AddProjectResponse{Project: inboxapi.Project{Ref: req.Ref}, Webhook: "switchboard relay hook add " + req.Ref}, nil
}

func TestProjectsAreRead(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/api/projects", "", nil)
	var got inboxapi.Projects
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil || !got.Enabled || len(got.Items) != 1 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
}

func TestProjectIsAdded(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/projects", `{"ref":"gh:o/notes","plan_first":true}`, good)
	var got inboxapi.AddProjectResponse
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Webhook == "" {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	if len(f.added) != 1 || f.added[0].Ref != "gh:o/notes" || !f.added[0].PlanFirst {
		t.Fatalf("added = %+v", f.added)
	}
}

func TestAddingAProjectFromElsewhereIsRefused(t *testing.T) {
	for name, h := range map[string]map[string]string{
		"other origin":  {"Content-Type": "application/json", "Origin": "https://evil.example"},
		"no origin":     {"Content-Type": "application/json"},
		"rebound host":  {"Content-Type": "application/json", "Origin": "http://" + listen, "Host": "evil.example:8783"},
		"form encoding": {"Content-Type": "application/x-www-form-urlencoded", "Origin": "http://" + listen},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInbox{}
			w := do(newServer(f), "POST", "/api/projects", `{"ref":"gh:evil/repo"}`, h)
			if w.Code != http.StatusForbidden && w.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("status = %d", w.Code)
			}
			if len(f.added) != 0 {
				t.Fatal("the project was passed on")
			}
		})
	}
}

func TestRefusedAdditionsKeepTheirCode(t *testing.T) {
	for code, want := range map[string]int{
		inboxapi.CodeAddingDisabled: http.StatusForbidden,
		inboxapi.CodeProjectExists:  http.StatusConflict,
		inboxapi.CodeForgeNotUsed:   http.StatusUnprocessableEntity,
		inboxapi.CodeRepoUnreadable: http.StatusUnprocessableEntity,
		inboxapi.CodePathTaken:      http.StatusConflict,
		inboxapi.CodeConfigInvalid:  http.StatusUnprocessableEntity,
		inboxapi.CodeConfigChanged:  http.StatusConflict,
	} {
		f := &fakeInbox{err: &inboxapi.Error{Code: code, Message: code}}
		w := do(newServer(f), "POST", "/api/projects", `{"ref":"gh:o/notes"}`, good)
		var e inboxapi.Error
		if w.Code != want || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != code {
			t.Errorf("%s: status = %d, body = %s", code, w.Code, w.Body)
		}
	}
}
