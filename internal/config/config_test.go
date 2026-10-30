package config

import (
	"os"
	"path/filepath"
	"regexp"
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

const gitlab = `
  gitlab:
    base_url: https://gl.uskan.dev
    token_ref: switchboard/gitlab
    inbox_token_ref: switchboard/gitlab-inbox
    agent_identity: alice
`

// withGitLab adds the GitLab settings, a GitLab trusted author, and a GitLab project on ubuntu-a.
func withGitLab(body, ref string) string {
	body = strings.Replace(body, "trusted_authors:\n  github: [alice]\n", "trusted_authors:\n  github: [alice]\n  gitlab: [alice]\n", 1)
	body = strings.Replace(body, "trusted_authors:", strings.TrimPrefix(gitlab, "\n")+"trusted_authors:", 1)
	return body + "  - ref: " + ref + "\n    path: /srv/gl\n    host: ubuntu-a\n"
}

func TestGitLabProjectsAreAccepted(t *testing.T) {
	for _, c := range []struct{ ref, owner, repo string }{
		{"gl:nas/switchboard-sandbox", "nas", "switchboard-sandbox"},
		{"gl:nas/tools/switchboard-sandbox", "nas/tools", "switchboard-sandbox"},
	} {
		cfg, err := Load(write(t, withGitLab(valid, c.ref)))
		if err != nil {
			t.Fatalf("%s: %v", c.ref, err)
		}
		p := cfg.Projects[2]
		if p.Forge != "gl" || p.Owner != c.owner || p.Repo != c.repo || p.FullName() != c.owner+"/"+c.repo {
			t.Errorf("%s parsed as %+v", c.ref, p)
		}
		if cfg.Projects[0].Forge != "gh" {
			t.Errorf("a GitHub project parsed as forge %q", cfg.Projects[0].Forge)
		}
	}
}

func TestGitLabProjectWithoutGitLabSettings(t *testing.T) {
	body := valid + "  - ref: gl:nas/switchboard-sandbox\n    path: /srv/gl\n    host: ubuntu-a\n"
	_, err := Load(write(t, body))
	if err == nil || !strings.Contains(err.Error(), "forges.gitlab") || !strings.Contains(err.Error(), "trusted_authors.gitlab") {
		t.Fatalf("err = %v, want the missing GitLab settings named", err)
	}
}

func TestBadGitLabRef(t *testing.T) {
	_, err := Load(write(t, withGitLab(valid, "gl:onlyone")))
	if err == nil || !strings.Contains(err.Error(), "gl:group/project") {
		t.Fatalf("err = %v, want the expected form named", err)
	}
}

func TestGitLabBaseURLDefaultsToGitLabCom(t *testing.T) {
	cfg, err := Load(write(t, strings.Replace(withGitLab(valid, "gl:nas/sb"), "    base_url: https://gl.uskan.dev\n", "", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Forges.GitLab.BaseURL != "https://gitlab.com" {
		t.Fatalf("base_url = %q", cfg.Forges.GitLab.BaseURL)
	}
}

func TestGitLabRefusesSearchScope(t *testing.T) {
	body := strings.Replace(withGitLab(valid, "gl:nas/sb"), "    agent_identity: alice\ntrusted_authors", "    agent_identity: alice\n    search_scope: \"group:nas\"\ntrusted_authors", 1)
	_, err := Load(write(t, body))
	if err == nil || !strings.Contains(err.Error(), "search_scope") {
		t.Fatalf("err = %v, want search_scope refused for GitLab", err)
	}
}

func TestGitHubOnlyNeedsNoGitLabSettings(t *testing.T) {
	cfg, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Uses("gl") || !cfg.Uses("gh") {
		t.Fatalf("Uses(gl)=%v Uses(gh)=%v", cfg.Uses("gl"), cfg.Uses("gh"))
	}
}

func TestGitLabOnlyNeedsNoGitHubSettings(t *testing.T) {
	body := `
version: 1
host: {name: ubuntu-a}
forges:
  gitlab: {token_ref: switchboard/gitlab, inbox_token_ref: switchboard/gitlab-inbox, agent_identity: alice}
trusted_authors: {gitlab: [alice]}
projects:
  - {ref: gl:nas/sb, path: /srv/gl, host: ubuntu-a}
`
	if _, err := Load(write(t, body)); err != nil {
		t.Fatal(err)
	}
}

func TestProjectOverridesTheAgentIdentity(t *testing.T) {
	body := withGitLab(valid, "gl:nas/sb") + "    token_ref: switchboard/gitlab-sb-bot\n    agent_identity: project_7_bot\n"
	cfg, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	gl, gh := cfg.Projects[2], cfg.Projects[0]
	if cfg.AgentTokenRef(gl) != "switchboard/gitlab-sb-bot" || cfg.AgentIdentity(gl) != "project_7_bot" {
		t.Errorf("override: %q %q", cfg.AgentTokenRef(gl), cfg.AgentIdentity(gl))
	}
	if cfg.AgentTokenRef(gh) != "switchboard/github" || cfg.AgentIdentity(gh) != "alice" {
		t.Errorf("forge default: %q %q", cfg.AgentTokenRef(gh), cfg.AgentIdentity(gh))
	}
	if cfg.InboxTokenRef("gl") != "switchboard/gitlab-inbox" || cfg.InboxTokenRef("gh") != "switchboard/github-inbox" {
		t.Errorf("inbox: %q %q", cfg.InboxTokenRef("gl"), cfg.InboxTokenRef("gh"))
	}
	if got := cfg.TrustedAuthorsOf("gl"); len(got) != 1 || got[0] != "alice" {
		t.Errorf("TrustedAuthorsOf(gl) = %v", got)
	}
}

func TestOverrideNeedsBothKeys(t *testing.T) {
	for _, extra := range []string{"    token_ref: switchboard/x\n", "    agent_identity: bot\n"} {
		_, err := Load(write(t, withGitLab(valid, "gl:nas/sb")+extra))
		if err == nil || !strings.Contains(err.Error(), "token_ref and agent_identity") {
			t.Errorf("%q: err = %v, want both keys required together", extra, err)
		}
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

// The GitLab lines of the example are the indented comments. Uncommented, with the placeholders kept, they load.
func TestExampleConfigGitLabLinesLoad(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "switchboard.example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	uncomment := regexp.MustCompile(`(?m)^(\s+)# `)
	cfg, err := Load(write(t, uncomment.ReplaceAllString(string(data), "$1")))
	if err != nil {
		t.Fatalf("the uncommented GitLab example does not load: %v", err)
	}
	gl := cfg.Projects[len(cfg.Projects)-1]
	if gl.Forge != "gl" || cfg.AgentIdentity(gl) != "project_N_bot_XXXX" || cfg.Forges.GitLab.BaseURL != "https://gitlab.com" {
		t.Fatalf("parsed %+v, base_url %q", gl, cfg.Forges.GitLab.BaseURL)
	}
}

func TestMissingFileIsReported(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil || !strings.Contains(err.Error(), "nope.yaml") {
		t.Fatalf("err = %v", err)
	}
}
