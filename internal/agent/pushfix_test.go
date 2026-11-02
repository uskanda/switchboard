package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/marker"
)

// pushToIssueBranch adds a commit to origin's switchboard/issue-12 from another clone, as a person does.
func (x *h) pushToIssueBranch(name string) {
	x.t.Helper()
	other := filepath.Join(x.t.TempDir(), "other")
	run(x.t, filepath.Dir(other), "git", "clone", "-q", x.origin, other)
	if x.originHas("switchboard/issue-12") {
		run(x.t, other, "git", "switch", "-q", "switchboard/issue-12")
	} else {
		run(x.t, other, "git", "switch", "-q", "-c", "switchboard/issue-12")
	}
	if err := os.WriteFile(filepath.Join(other, name), []byte("by a person\n"), 0o644); err != nil {
		x.t.Fatal(err)
	}
	run(x.t, other, "git", "add", name)
	run(x.t, other, "git", "commit", "-q", "-m", "a person's change")
	run(x.t, other, "git", "push", "-q", "origin", "switchboard/issue-12")
}

func TestNonFastForwardStartsAFixRunThatMerges(t *testing.T) {
	x := newH(t, "", nil)
	x.pushToIssueBranch("person.txt")
	x.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	x.step(2, "git merge -q --no-edit origin/switchboard/issue-12 >/dev/null\n"+done("Merged the person's change."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.starts() != 2 {
		t.Fatalf("claude started %d times, want 2", x.starts())
	}
	claims := x.claims(x.ref)
	if len(claims) != 2 {
		t.Fatalf("claims = %d, want 2", len(claims))
	}
	m, _ := marker.Parse(claims[1].Body)
	if m.Reason != marker.Fix || !strings.Contains(claims[1].Body, "(1/2)") {
		t.Fatalf("the second claim is not a fix claim:\n%s", claims[1].Body)
	}
	prompt := x.file("prompt-2.md")
	if !strings.Contains(prompt, "<failure-") || !strings.Contains(prompt, "fetch first") {
		t.Fatalf("the fix run's prompt lacks the push failure:\n%s", prompt)
	}
	p := x.lastPost(x.ref)
	if x.labels(x.ref) != "agent:done" || p.Marker.State != "done" || len(x.f.PRs) != 1 {
		t.Fatalf("labels=%q post=%+v PRs=%d\n%s", x.labels(x.ref), p.Marker, len(x.f.PRs), p.Body)
	}
	log := run(t, x.origin, "git", "log", "--format=%s", "switchboard/issue-12")
	if !strings.Contains(log, "a person's change") || !strings.Contains(log, "change flag.txt") {
		t.Fatalf("origin's branch lost a commit (force push?):\n%s", log)
	}
}

func TestNonFastForwardWithoutFixesWaits(t *testing.T) {
	x := newH(t, "agent:\n  max_fix_attempts: 0\n", nil)
	x.pushToIssueBranch("person.txt")
	x.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.starts() != 1 || x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != marker.Push || !strings.Contains(p.Body, "fetch first") {
		t.Fatalf("starts=%d labels=%q post=%+v\n%s", x.starts(), x.labels(x.ref), p.Marker, p.Body)
	}
}

func TestRejectedPushWaitsWithTheRemoteMessage(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Comments.Language = "ja" })
	hook := filepath.Join(x.origin, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho 'GL-HOOK-ERR: commit message must reference an issue' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	x.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.starts() != 1 || p.Marker.Reason != marker.Push || !strings.Contains(p.Body, "commit message must reference an issue") || !strings.Contains(p.Body, "リモートの規則") {
		t.Fatalf("starts=%d post=%+v\n%s", x.starts(), p.Marker, p.Body)
	}
	if len(x.f.PRs) != 0 {
		t.Fatal("a pull request was opened for a refused push")
	}
}

func TestDoneMarkerNamesTheCommitAndThePullRequest(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	head := run(t, x.origin, "git", "rev-parse", "switchboard/issue-12")
	if p.Marker.SHA != head || p.Marker.CR != 1 {
		t.Fatalf("marker = %+v, want sha %s and cr 1", p.Marker, head)
	}
}
