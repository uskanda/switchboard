package trust

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
)

var ctx = context.Background()

// The same login is a different account on each forge; each forge's issues are judged by its own ID, and both
// records are kept.
func TestResolvePerForge(t *testing.T) {
	gh, gl := fake.New(nil), fake.NewGitLab(nil)
	gh.AddUser("uskanda", 13825)
	gl.AddUser("uskanda", 1)
	d := state.Dir{Path: t.TempDir()}
	jh, err := ResolveFor(ctx, gh, d, forge.GitHub, []string{"uskanda"}, "uskanda", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	jl, err := ResolveFor(ctx, gl, d, forge.GitLab, []string{"uskanda"}, "uskanda", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !jh.Trusted[13825] || jh.Trusted[1] || !jl.Trusted[1] || jl.Trusted[13825] {
		t.Fatalf("github %v, gitlab %v", jh.Trusted, jl.Trusted)
	}
	ids, err := d.Identities()
	if err != nil || ids["gh:uskanda"] != 13825 || ids["gl:uskanda"] != 1 {
		t.Fatalf("identities = %v, %v", ids, err)
	}
	// Resolving again with both records in place is fine.
	if _, err := ResolveFor(ctx, gl, d, forge.GitLab, []string{"uskanda"}, "uskanda", nil, nil); err != nil {
		t.Fatal(err)
	}
}

// Phase 1 recorded GitHub logins without a forge. Those records still guard the GitHub logins, and are rewritten
// with the gh: key.
func TestResolveReadsPhase1RecordsAsGitHub(t *testing.T) {
	d := state.Dir{Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(d.Path, "identities.json"), []byte(`{"alice": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	gh := fake.New(nil)
	gh.AddUser("alice", 2) // the login now belongs to someone else
	if _, err := ResolveFor(ctx, gh, d, forge.GitHub, []string{"alice"}, "alice", nil, nil); err == nil || !strings.Contains(err.Error(), "alice") {
		t.Fatalf("a changed ID behind a Phase 1 record was accepted: %v", err)
	}
	gh2 := fake.New(nil)
	gh2.AddUser("alice", 1)
	if _, err := ResolveFor(ctx, gh2, d, forge.GitHub, []string{"alice"}, "alice", nil, nil); err != nil {
		t.Fatal(err)
	}
	ids, _ := d.Identities()
	if ids["gh:alice"] != 1 {
		t.Fatalf("identities = %v", ids)
	}
	if _, legacy := ids["alice"]; legacy {
		t.Fatalf("the Phase 1 key was kept: %v", ids)
	}
	// A GitLab login of the same name is not judged by the GitHub record.
	gl := fake.NewGitLab(nil)
	gl.AddUser("alice", 77)
	if _, err := ResolveFor(ctx, gl, d, forge.GitLab, []string{"alice"}, "alice", nil, nil); err != nil {
		t.Fatalf("the GitHub record blocked a GitLab login: %v", err)
	}
}

// A project with its own agent identity recognizes only that identity's marked comments as the agent's posts.
func TestAgentIdentityPerProject(t *testing.T) {
	gl := fake.NewGitLab(nil)
	me := gl.AddUser("uskanda", 1)
	bot := gl.AddUser("project_7_bot", 50)
	gl.AddProject("nas", "a", "private")
	gl.AddProject("nas", "b", "private")
	d := state.Dir{Path: t.TempDir()}
	j, err := ResolveFor(ctx, gl, d, forge.GitLab, []string{"uskanda"}, "uskanda", map[string]string{"gl:nas/a": "project_7_bot"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := gl.AddIssue("nas", "a", 1, "t", "b", me)
	b := gl.AddIssue("nas", "b", 1, "t", "b", me)
	post := "done\n\n" + marker.Marker{Host: "h", Run: "r1", State: "waiting"}.Format()
	byBot := forge.Comment{ID: 1, Author: bot, Body: post}
	byMe := forge.Comment{ID: 2, Author: me, Body: post}
	if j.Comment(a, byBot) != AgentPost || j.Comment(a, byMe) == AgentPost {
		t.Errorf("project a: bot %v, me %v", j.Comment(a, byBot), j.Comment(a, byMe))
	}
	if j.Comment(b, byMe) != AgentPost || j.Comment(b, byBot) == AgentPost {
		t.Errorf("project b: me %v, bot %v", j.Comment(b, byMe), j.Comment(b, byBot))
	}
	if j.AgentIDOf(a) != 50 || j.AgentIDOf(b) != 1 {
		t.Errorf("AgentIDOf = %d, %d", j.AgentIDOf(a), j.AgentIDOf(b))
	}
	if ids, _ := d.Identities(); ids["gl:project_7_bot"] != 50 {
		t.Errorf("the bot's ID is not recorded: %v", ids)
	}
}

// An edit whose editor the forge does not name is not trusted, and the audit log says the editor is unknown.
func TestUnknownEditorIsRejectedWithItsReason(t *testing.T) {
	rec := &recorder{}
	j := judge(rec)
	is := &forge.Issue{IssueSummary: forge.IssueSummary{Ref: forge.IssueRef{Forge: forge.GitLab, Owner: "nas", Repo: "a", Number: 1}}, Author: alice,
		BodyEdit: forge.Edit{Edited: true, Unknown: true}}
	if j.Body(is) {
		t.Fatal("a body edited by an unknown user was taken")
	}
	c := forge.Comment{ID: 5, Author: alice, Body: "go", Edit: forge.Edit{Edited: true, Unknown: true}}
	if j.Comment(is.Ref, c) != Untrusted {
		t.Fatal("a comment edited by an unknown user was taken")
	}
	for _, e := range rec.events {
		if e["reason"] != "the last editor is unknown" {
			t.Errorf("reason = %v", e["reason"])
		}
	}
}

// GitLab's internal projects count as public, so include_untrusted is refused on them.
func TestInternalProjectRefusesIncludeUntrusted(t *testing.T) {
	gl := fake.NewGitLab(nil)
	gl.AddProject("nas", "shared", "internal")
	private, err := gl.RepoPrivate(ctx, "nas", "shared")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UntrustedPolicy(true, private); err != ErrUntrustedOnPublic {
		t.Fatalf("UntrustedPolicy on internal = %v", err)
	}
}
