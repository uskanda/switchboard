package agent

import (
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

// GitHub keeps the Phase 1 names, so a machine that already ran Phase 1 finds its locks and run records. GitLab
// names start with "@", which no GitHub owner can, so the two never collide.
func TestStateNamesKeepPhase1ForGitHubAndSeparateGitLab(t *testing.T) {
	gh := forge.IssueRef{Forge: forge.GitHub, Owner: "uskanda", Repo: "sandbox", Number: 1}
	gl := forge.IssueRef{Forge: forge.GitLab, Owner: "nas/tools", Repo: "sandbox", Number: 1}
	if got := lockName(gh); got != "uskanda-sandbox-1" {
		t.Errorf("lockName(gh) = %q", got)
	}
	if got := recordName(gh); got != "issues/uskanda/sandbox/1.json" {
		t.Errorf("recordName(gh) = %q", got)
	}
	if got := lockName(gl); got != "@gitlab-nas~tools-sandbox-1" {
		t.Errorf("lockName(gl) = %q", got)
	}
	if got := recordName(gl); got != "issues/@gitlab/nas/tools/sandbox/1.json" {
		t.Errorf("recordName(gl) = %q", got)
	}
}
