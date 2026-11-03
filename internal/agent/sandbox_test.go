package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
)

func TestTheVerifyCommandAndGitRunOutsideTheSandbox(t *testing.T) {
	cases := map[string]struct {
		got  claudecode.Settings
		want []string
	}{
		"implement":         {settings("make verify"), []string{"make verify", "git", "git *"}},
		"spaces":            {settings("  make   verify "), []string{"make verify", "git", "git *"}},
		"no verify command": {settings(""), []string{"git", "git *"}},
		"plan":              {planSettings(), []string{"git", "git *"}},
	}
	for name, c := range cases {
		if !slices.Equal(c.got.Unsandboxed, c.want) {
			t.Errorf("%s: unsandboxed = %v, want %v", name, c.got.Unsandboxed, c.want)
		}
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDenyReadListsTheCredentialsTheConfigurationAndTheStateThatExist(t *testing.T) {
	root := t.TempDir()
	home, cfgDir, stateDir := filepath.Join(root, "home"), filepath.Join(root, "cfg"), filepath.Join(root, "state")
	for _, p := range []string{".ssh/id_ed25519", ".config/gh/hosts.yml", ".docker/config.json", ".netrc"} {
		touch(t, filepath.Join(home, p))
	}
	touch(t, filepath.Join(cfgDir, "config.yaml"))
	for _, p := range []string{"audit.jsonl", "runs/r1/prompt.md", "issues/x.json"} {
		touch(t, filepath.Join(stateDir, p))
	}
	worktree := filepath.Join(stateDir, "worktrees", "o", "r", "12")
	touch(t, filepath.Join(worktree, "README.md"))

	got := denyRead(home, filepath.Join(cfgDir, "config.yaml"), stateDir, worktree)
	want := []string{
		filepath.Join(home, ".ssh"), filepath.Join(home, ".netrc"), filepath.Join(home, ".config/gh"),
		filepath.Join(home, ".docker/config.json"),
		cfgDir,
		filepath.Join(stateDir, "audit.jsonl"), filepath.Join(stateDir, "issues"), filepath.Join(stateDir, "runs"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("denyRead =\n%v\nwant\n%v", got, want)
	}
}

func TestDenyReadNeverHidesTheWorktree(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	worktree := filepath.Join(stateDir, "worktrees", "o", "r", "12")
	touch(t, filepath.Join(worktree, "README.md"))
	touch(t, filepath.Join(stateDir, "runs", "r1", "prompt.md"))
	// The configuration sits above the state, as in a test or an unusual layout.
	got := denyRead(filepath.Join(root, "home"), filepath.Join(root, "config.yaml"), stateDir, worktree)
	if want := []string{filepath.Join(stateDir, "runs")}; !slices.Equal(got, want) {
		t.Errorf("denyRead = %v, want %v", got, want)
	}
}

// sandboxOf reads the sandbox settings the nth start of claude got.
func (x *h) sandboxOf(n int) (excluded, denyRead []string) {
	x.t.Helper()
	var s struct {
		Sandbox struct {
			ExcludedCommands []string `json:"excludedCommands"`
			Filesystem       struct {
				DenyRead []string `json:"denyRead"`
			} `json:"filesystem"`
		} `json:"sandbox"`
	}
	if err := json.Unmarshal([]byte(x.file("settings-"+strconv.Itoa(n)+".json")), &s); err != nil {
		x.t.Fatal(err)
	}
	return s.Sandbox.ExcludedCommands, s.Sandbox.Filesystem.DenyRead
}

func TestARunHidesTheStateButNotItsWorktree(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, commit("a.txt", "a")+done("Done."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	excluded, deny := x.sandboxOf(1)
	if !slices.Equal(excluded, []string{"git", "git *"}) {
		t.Errorf("excluded = %v", excluded)
	}
	if !slices.Contains(deny, filepath.Join(x.dir.Path, "runs")) {
		t.Errorf("denyRead = %v, want the runs", deny)
	}
	for _, p := range deny {
		if strings.HasPrefix(p, filepath.Join(x.dir.Path, "worktrees")) {
			t.Errorf("denyRead hides %s", p)
		}
	}
}

func TestARunFailsWithoutStartingClaudeWhenTheSandboxIsUnavailable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the check runs bwrap only on Linux")
	}
	bwrap := filepath.Join(t.TempDir(), "bwrap")
	if err := os.WriteFile(bwrap, []byte("#!/bin/sh\necho 'bwrap: Creating new namespace failed: No space left on device' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	x := newH(t, "", nil)
	x.agentWith(func(a *Agent) {
		a.CheckSandbox = func(ctx context.Context) error { return claudecode.CheckSandbox(ctx, bwrap) }
	})
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if n := x.starts(); n != 0 {
		t.Fatalf("claude started %d time(s)", n)
	}
	p := x.lastPost(x.ref)
	if p.Marker.State != "waiting" || p.Marker.Reason != "error" || !strings.Contains(p.Body, "Creating new namespace failed") {
		t.Fatalf("post = %+v", p)
	}
}

func TestAWritingRunIsToldAboutTheSandboxAndAPlanRunIsNot(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Projects[0].VerifyCommand = "true" })
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.startPlan()
	if sys := x.file("system-1.md"); strings.Contains(sys, "## The sandbox") || !strings.Contains(sys, "sandbox") {
		t.Errorf("plan run system prompt:\n%s", sys)
	}
	x.replyAndRun("/ok")
	if sys := x.file("system-2.md"); !strings.Contains(sys, "## The sandbox") || !strings.Contains(sys, "Only git and `true` run outside") {
		t.Errorf("approved run system prompt:\n%s", sys)
	}
	if excluded, _ := x.sandboxOf(2); !slices.Equal(excluded, []string{"true", "git", "git *"}) {
		t.Errorf("excluded = %v", excluded)
	}
}
