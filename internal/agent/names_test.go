package agent

import (
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

// GitHub keeps the Phase 1 run records, so a machine that already ran Phase 1 finds them. GitLab names start with
// "@", which no GitHub owner can, so the two never collide. Lock names are taken only while a run starts, so they
// need no history; they join the parts with "~", which neither forge allows in a name.
func TestStateNamesKeepPhase1ForGitHubAndSeparateGitLab(t *testing.T) {
	gh := forge.IssueRef{Forge: forge.GitHub, Owner: "uskanda", Repo: "sandbox", Number: 1}
	gl := forge.IssueRef{Forge: forge.GitLab, Owner: "nas/tools", Repo: "sandbox", Number: 1}
	if got := lockName(gh); got != "gh~uskanda~sandbox~1" {
		t.Errorf("lockName(gh) = %q", got)
	}
	if got := recordName(gh); got != "issues/uskanda/sandbox/1.json" {
		t.Errorf("recordName(gh) = %q", got)
	}
	if got := lockName(gl); got != "gl~nas~tools~sandbox~1" {
		t.Errorf("lockName(gl) = %q", got)
	}
	if got := recordName(gl); got != "issues/@gitlab/nas/tools/sandbox/1.json" {
		t.Errorf("recordName(gl) = %q", got)
	}
}

// Issues of different repositories never share a lock: a hyphen in a name is not a separator.
func TestLockNamesDoNotCollide(t *testing.T) {
	refs := []forge.IssueRef{
		{Forge: forge.GitHub, Owner: "a-b", Repo: "c", Number: 1},
		{Forge: forge.GitHub, Owner: "a", Repo: "b-c", Number: 1},
		{Forge: forge.GitLab, Owner: "a-b", Repo: "c", Number: 1},
		{Forge: forge.GitLab, Owner: "a/b", Repo: "c", Number: 1},
		{Forge: forge.GitLab, Owner: "a", Repo: "b-c", Number: 1},
		{Forge: forge.GitHub, Owner: "a", Repo: "b", Number: 11},
		{Forge: forge.GitHub, Owner: "a", Repo: "b-1", Number: 1},
	}
	seen := map[string]forge.IssueRef{}
	for _, r := range refs {
		n := lockName(r)
		if strings.Contains(n, "/") {
			t.Errorf("lockName(%s) = %q is not one file name", r, n)
		}
		if prev, ok := seen[n]; ok {
			t.Errorf("lockName(%s) = lockName(%s) = %q", r, prev, n)
		}
		seen[n] = r
	}
}
