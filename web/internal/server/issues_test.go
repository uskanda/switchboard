package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

func (f *fakeInbox) IssueTargets(context.Context) (*inboxapi.IssueTargets, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &inboxapi.IssueTargets{
		Items:  []inboxapi.IssueTarget{{Ref: "gh:o/r", Host: "nas"}},
		Starts: []inboxapi.IssueStart{{Kind: inboxapi.StartReady, Label: "agent:ready"}},
	}, nil
}

func (f *fakeInbox) CreateIssue(_ context.Context, req inboxapi.CreateIssueRequest) (*inboxapi.CreateIssueResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.created = append(f.created, req)
	return &inboxapi.CreateIssueResponse{Ref: req.Ref + "#41", URL: "https://github.test/o/r/issues/41"}, nil
}

func TestIssueTargetsAreRead(t *testing.T) {
	w := do(newServer(&fakeInbox{}), "GET", "/api/issue-targets", "", nil)
	var got inboxapi.IssueTargets
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil || len(got.Items) != 1 || len(got.Starts) != 1 {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
}

func TestIssueIsCreated(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/issues", `{"ref":"gh:o/r","title":"Add a flag","body":"Use --x.","start":"plan"}`, good)
	var got inboxapi.CreateIssueResponse
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Ref != "gh:o/r#41" || got.URL == "" {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	want := inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: "Add a flag", Body: "Use --x.", Start: "plan"}
	if len(f.created) != 1 || f.created[0] != want {
		t.Fatalf("created = %+v", f.created)
	}
}

func TestCreatingAnIssueFromElsewhereIsRefused(t *testing.T) {
	for name, tc := range map[string]struct {
		h    map[string]string
		want int
	}{
		"other origin":  {map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, http.StatusForbidden},
		"no origin":     {map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
		"rebound host":  {map[string]string{"Content-Type": "application/json", "Origin": "http://" + listen, "Host": "evil.example:8783"}, http.StatusMisdirectedRequest},
		"form encoding": {map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Origin": "http://" + listen}, http.StatusUnsupportedMediaType},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInbox{}
			w := do(newServer(f), "POST", "/api/issues", `{"ref":"gh:o/r","title":"t","start":"go"}`, tc.h)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
			if len(f.created) != 0 {
				t.Fatal("the issue was passed on")
			}
		})
	}
}

func TestCreatingAnIssueTooLargeIsRefused(t *testing.T) {
	f := &fakeInbox{}
	body := `{"ref":"gh:o/r","title":"t","start":"go","body":"` + strings.Repeat("x", 7*inboxapi.MaxIssueBody) + `"}`
	w := do(newServer(f), "POST", "/api/issues", body, good)
	if w.Code != http.StatusRequestEntityTooLarge || len(f.created) != 0 {
		t.Fatalf("status = %d, created = %d", w.Code, len(f.created))
	}
}

func TestRefusedIssuesKeepTheirCode(t *testing.T) {
	for code, want := range map[string]int{
		inboxapi.CodeUnknownProject: http.StatusUnprocessableEntity,
		inboxapi.CodeInvalid:        http.StatusBadRequest,
		inboxapi.CodeTooLarge:       http.StatusRequestEntityTooLarge,
		inboxapi.CodeForge:          http.StatusBadGateway,
	} {
		f := &fakeInbox{err: &inboxapi.Error{Code: code, Message: code}}
		w := do(newServer(f), "POST", "/api/issues", `{"ref":"gh:o/r","title":"t","start":"ready"}`, good)
		var e inboxapi.Error
		if w.Code != want || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != code {
			t.Errorf("%s: status = %d, body = %s", code, w.Code, w.Body)
		}
	}
}
