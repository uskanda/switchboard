package inboxserve

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/permissions"
)

type permissionEvents struct{ names []string }

func (e *permissionEvents) Log(event string, _ map[string]any) { e.names = append(e.names, event) }

// withPermissions serves /v1/permissions over the projects gh:o/r and gl:nas/sb, with gh:o/r#50 done by the agent.
func withPermissions(t *testing.T) (*issuesWorld, permissions.Store, *permissionEvents) {
	t.Helper()
	w := withMerges(t)
	ev := &permissionEvents{}
	s := permissions.Store{Dir: filepath.Join(t.TempDir(), "permissions"), Audit: ev}
	w.srv.Permissions = &Permissions{Store: s, Projects: []string{"gh:o/r", "gl:nas/sb"}, Host: "here"}
	return w, s, ev
}

func (w *issuesWorld) permissions(issue string) (int, inboxapi.Permissions, inboxapi.Error) {
	rec := w.do("GET", "/v1/permissions?issue="+issue, nil)
	var out inboxapi.Permissions
	var e inboxapi.Error
	decode(rec, &out, &e)
	return rec.Code, out, e
}

func (w *issuesWorld) savePermissions(req inboxapi.SavePermissionsRequest) (int, inboxapi.Error) {
	rec := w.do("POST", "/v1/permissions", req)
	var e inboxapi.Error
	decode(rec, nil, &e)
	return rec.Code, e
}

func TestPermissionsListEveryScope(t *testing.T) {
	w, s, _ := withPermissions(t)
	s.Add(permissions.Scope{Project: "gh:o/r"}, []string{"Bash(git mv *)"})
	s.Add(permissions.Scope{Project: "gh:o/r", Issue: 50}, []string{"Bash(git ls-tree *)"})
	code, got, e := w.permissions("gh%3Ao%2Fr%2350")
	if code != 200 {
		t.Fatalf("%d %+v", code, e)
	}
	want := []inboxapi.PermissionScope{
		{Scope: "global", Rules: []string{}},
		{Scope: "project", Ref: "gh:o/r", Rules: []string{"Bash(git mv *)"}},
		{Scope: "project", Ref: "gl:nas/sb", Rules: []string{}},
		{Scope: "issue", Ref: "gh:o/r#50", Rules: []string{"Bash(git ls-tree *)"}},
	}
	if got.Host != "here" || len(got.Items) != len(want) {
		t.Fatalf("permissions = %+v", got)
	}
	for i := range want {
		g := got.Items[i]
		if g.Scope != want[i].Scope || g.Ref != want[i].Ref || !slices.Equal(g.Rules, want[i].Rules) {
			t.Errorf("item %d = %+v, want %+v", i, g, want[i])
		}
	}
	if code, got, _ := w.permissions(""); code != 200 || len(got.Items) != 3 {
		t.Fatalf("without an issue: %d %+v", code, got)
	}
}

func TestSavePermissions(t *testing.T) {
	w, s, ev := withPermissions(t)
	if code, e := w.savePermissions(inboxapi.SavePermissionsRequest{Scope: "issue", Ref: "gh:o/r#50", Add: []string{"Bash(git ls-tree *)", "Bash(git mv *)"}}); code != 204 {
		t.Fatalf("add: %d %+v", code, e)
	}
	if code, e := w.savePermissions(inboxapi.SavePermissionsRequest{Scope: "issue", Ref: "gh:o/r#50", Remove: []string{"Bash(git mv *)"}}); code != 204 {
		t.Fatalf("remove: %d %+v", code, e)
	}
	if got, _ := s.Read(permissions.Scope{Project: "gh:o/r", Issue: 50}); !slices.Equal(got, []string{"Bash(git ls-tree *)"}) {
		t.Fatalf("issue rules = %q", got)
	}
	if code, e := w.savePermissions(inboxapi.SavePermissionsRequest{Scope: "global", Add: []string{"Skill"}}); code != 204 {
		t.Fatalf("global: %d %+v", code, e)
	}
	if !slices.Equal(ev.names, []string{"permission.saved", "permission.removed", "permission.saved"}) {
		t.Fatalf("audit = %q", ev.names)
	}
}

func TestSavePermissionsRefusals(t *testing.T) {
	w, s, _ := withPermissions(t)
	code, e := w.savePermissions(inboxapi.SavePermissionsRequest{Scope: "project", Ref: "gh:o/r", Add: []string{"Bash(git mv *)", "Bash(git *)"}})
	if code != 422 || e.Code != inboxapi.CodeRuleRefused || len(e.Rules) != 1 || e.Rules[0].Rule != "Bash(git *)" || e.Rules[0].Reason == "" {
		t.Fatalf("a refused rule: %d %+v", code, e)
	}
	if got, _ := s.Read(permissions.Scope{Project: "gh:o/r"}); len(got) != 0 {
		t.Fatalf("saved %q", got)
	}
	for name, req := range map[string]inboxapi.SavePermissionsRequest{
		"an unknown project": {Scope: "project", Ref: "gh:someone/other", Add: []string{"Skill"}},
		"an unknown issue":   {Scope: "issue", Ref: "gh:o/r#999", Add: []string{"Skill"}},
		"an unknown scope":   {Scope: "host", Add: []string{"Skill"}},
	} {
		if code, _ := w.savePermissions(req); code < 400 {
			t.Errorf("%s: %d", name, code)
		}
	}
}

func TestOnlyAnAdminGrantsPermissions(t *testing.T) {
	w, _, a := multi(t)
	w.srv.Permissions = &Permissions{Store: permissions.Store{Dir: filepath.Join(t.TempDir(), "permissions")}, Projects: []string{"gl:nas/sb"}}
	if _, err := w.c.Permissions(as(t, w, "alice"), ""); !errors.Is(err, inboxapi.ErrForbidden) {
		t.Fatalf("alice reads: %v", err)
	}
	if err := w.c.SavePermissions(as(t, w, "alice"), inboxapi.SavePermissionsRequest{Scope: "global", Add: []string{"Skill"}}); !errors.Is(err, inboxapi.ErrForbidden) {
		t.Fatalf("alice saves: %v", err)
	}
	if err := w.c.SavePermissions(as(t, w, "olivia"), inboxapi.SavePermissionsRequest{Scope: "global", Add: []string{"Skill"}}); err != nil {
		t.Fatalf("olivia saves: %v", err)
	}
	if len(a.lines) == 0 || a.lines[len(a.lines)-1]["action"] != "permissions.save" {
		t.Fatalf("web.action = %+v", a.lines)
	}
}

// decode reads a recorded answer into out when it succeeded, and into e otherwise.
func decode(rec interface {
	Result() *http.Response
}, out any, e *inboxapi.Error) {
	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode < 300 {
		if out != nil {
			json.NewDecoder(resp.Body).Decode(out)
		}
		return
	}
	json.NewDecoder(resp.Body).Decode(e)
}
