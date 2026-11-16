package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
)

func (f *fakeInbox) Permissions(_ context.Context, issue string) (*inboxapi.Permissions, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.permissionsOf = issue
	return &inboxapi.Permissions{Host: "ubuntu-a", Items: []inboxapi.PermissionScope{{Scope: "global", Rules: []string{"Skill"}}}}, nil
}

func (f *fakeInbox) SavePermissions(_ context.Context, req inboxapi.SavePermissionsRequest) error {
	if f.err != nil {
		return f.err
	}
	f.granted = append(f.granted, req)
	return nil
}

func TestPermissionsAreRead(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "GET", "/api/permissions?issue=gh%3Ao%2Fr%2312", "", nil)
	var got inboxapi.Permissions
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Host != "ubuntu-a" || f.permissionsOf != "gh:o/r#12" {
		t.Fatalf("status = %d, body = %s, issue = %q", w.Code, w.Body, f.permissionsOf)
	}
}

func TestPermissionsAreSaved(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/permissions", `{"scope":"issue","ref":"gh:o/r#12","add":["Bash(git mv *)"]}`, good)
	if w.Code != http.StatusNoContent || len(f.granted) != 1 || f.granted[0].Ref != "gh:o/r#12" || f.granted[0].Add[0] != "Bash(git mv *)" {
		t.Fatalf("status = %d, saved = %+v", w.Code, f.granted)
	}
}

func TestARefusedRuleIsPassedBack(t *testing.T) {
	f := &fakeInbox{err: &inboxapi.Error{Code: inboxapi.CodeRuleRefused, Message: "refused", Rules: []inboxapi.RefusedRule{{Rule: "Bash(git *)", Reason: "allows every git command"}}}}
	w := do(newServer(f), "POST", "/api/permissions", `{"scope":"global","add":["Bash(git *)"]}`, good)
	var e inboxapi.Error
	if w.Code != http.StatusUnprocessableEntity || json.Unmarshal(w.Body.Bytes(), &e) != nil || len(e.Rules) != 1 || e.Rules[0].Rule != "Bash(git *)" {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
}

func TestSaveOfPermissionsFromElsewhereIsRefused(t *testing.T) {
	for name, h := range map[string]map[string]string{
		"other origin":  {"Content-Type": "application/json", "Origin": "https://evil.example"},
		"no origin":     {"Content-Type": "application/json"},
		"rebound host":  {"Content-Type": "application/json", "Origin": "http://" + listen, "Host": "evil.example:8783"},
		"form encoding": {"Content-Type": "application/x-www-form-urlencoded", "Origin": "http://" + listen},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInbox{}
			w := do(newServer(f), "POST", "/api/permissions", `{"scope":"global","add":["Skill"]}`, h)
			if w.Code != http.StatusForbidden && w.Code != http.StatusUnsupportedMediaType && w.Code != http.StatusMisdirectedRequest {
				t.Fatalf("status = %d", w.Code)
			}
			if len(f.granted) != 0 {
				t.Fatal("the save was passed on")
			}
		})
	}
}
