package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const valid = `
version: 1
host:
  name: ubuntu-a
forges:
  github:
    token_ref: switchboard/github
    inbox_token_ref: switchboard/github-inbox
    agent_identity: alice
    search_scope: "user:alice"
trusted_authors:
  github: [alice]
projects:
  - ref: gh:alice/sandbox
    path: ~/repos/sandbox
    host: ubuntu-a
  - ref: gh:alice/other
    path: /srv/other
    host: ubuntu-b
    base_branch: develop
    verify_command: make test
`

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := DefaultPath(); got != "/xdg/switchboard/config.yaml" {
		t.Errorf("DefaultPath() = %q", got)
	}
}

func TestDefaultPathFallsBackToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/alice")
	if got := DefaultPath(); got != "/home/alice/.config/switchboard/config.yaml" {
		t.Errorf("DefaultPath() = %q", got)
	}
}

func TestLoadValidConfigAppliesDefaults(t *testing.T) {
	t.Setenv("HOME", "/home/alice")
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Labels.Waiting != "agent:waiting" || c.Labels.Ready != "agent:ready" {
		t.Errorf("default labels not applied: %+v", c.Labels)
	}
	if c.Agent.PollInterval != 60*time.Second || c.Agent.Timeout != 45*time.Minute || c.Agent.MaxParallel != 1 {
		t.Errorf("agent defaults not applied: %+v", c.Agent)
	}
	if c.Comments.Language != "en" || c.Claude.Auth != "subscription" || c.Claude.Binary != "claude" {
		t.Errorf("defaults not applied: %+v %+v", c.Comments, c.Claude)
	}
	p := c.Projects[0]
	if p.Owner != "alice" || p.Repo != "sandbox" || p.BaseBranch != "main" || p.Path != "/home/alice/repos/sandbox" {
		t.Errorf("project not resolved: %+v", p)
	}
	if c.Projects[1].BaseBranch != "develop" {
		t.Errorf("explicit base branch lost: %+v", c.Projects[1])
	}
}

func TestMissingRequiredKeyIsNamed(t *testing.T) {
	body := strings.Replace(valid, "    path: ~/repos/sandbox\n", "", 1)
	_, err := Load(write(t, body))
	if err == nil || !strings.Contains(err.Error(), "projects[0].path") {
		t.Fatalf("err = %v, want it to name projects[0].path", err)
	}
}

func TestUnknownKeyIsNamed(t *testing.T) {
	_, err := Load(write(t, valid+"agnet:\n  timeout: 1m\n"))
	if err == nil || !strings.Contains(err.Error(), "agnet") {
		t.Fatalf("err = %v, want it to name agnet", err)
	}
}

func TestTypeMismatchFails(t *testing.T) {
	_, err := Load(write(t, valid+"agent:\n  max_parallel: many\n"))
	if err == nil || !strings.Contains(err.Error(), "max_parallel") {
		t.Fatalf("err = %v, want a type error on max_parallel", err)
	}
}

func TestGitLabProjectIsRefused(t *testing.T) {
	body := strings.Replace(valid, "gh:alice/other", "gl:group/project", 1)
	_, err := Load(write(t, body))
	if err == nil || !strings.Contains(err.Error(), "GitLab is not supported") {
		t.Fatalf("err = %v, want GitLab to be refused", err)
	}
}

func TestPlaintextTokenIsRefusedWithoutEchoingIt(t *testing.T) {
	secret := "ghp_" + strings.Repeat("A", 36)
	body := strings.Replace(valid, "token_ref: switchboard/github\n", "token_ref: "+secret+"\n", 1)
	_, err := Load(write(t, body))
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("err = %v, want a refusal of the plaintext token", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "AAAA") {
		t.Fatalf("the error echoes the token: %v", err)
	}
}

func TestProjectsForHost(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	got := c.ProjectsFor("ubuntu-a")
	if len(got) != 1 || got[0].Repo != "sandbox" {
		t.Fatalf("ProjectsFor(ubuntu-a) = %+v", got)
	}
}

func TestExampleConfigLoads(t *testing.T) {
	if _, err := Load(filepath.Join("..", "..", "config", "switchboard.example.yaml")); err != nil {
		t.Fatalf("the example configuration does not load: %v", err)
	}
}

func TestMissingFileIsReported(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil || !strings.Contains(err.Error(), "nope.yaml") {
		t.Fatalf("err = %v", err)
	}
}
