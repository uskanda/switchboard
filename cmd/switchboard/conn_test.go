package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge/github"
	"github.com/uskanda/switchboard/internal/forge/gitlab"
)

func load(t *testing.T, yml string) *config.Config {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// tokens hands out "token-<ref>" and records which refs were asked for.
func tokens(asked *[]string) func(string) (string, error) {
	return func(ref string) (string, error) {
		*asked = append(*asked, ref)
		return "token-" + ref, nil
	}
}

func TestGitHubOnlyConfigNeverAsksForAGitLabToken(t *testing.T) {
	cfg := load(t, `version: 1
host: {name: here}
forges:
  github: {token_ref: gh-agent, inbox_token_ref: gh-inbox, agent_identity: bot}
trusted_authors: {github: [me]}
projects:
  - {ref: "gh:o/r", path: /src/r, host: here}
`)
	var asked []string
	c := newConn(cfg, tokens(&asked))
	if _, err := c.agentRoutes(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.inboxForges(""); err != nil {
		t.Fatal(err)
	}
	for _, r := range asked {
		if r != "gh-agent" && r != "gh-inbox" {
			t.Fatalf("asked for %q", r)
		}
	}
}

func TestClientsFollowTheProjectsForgeAndOverride(t *testing.T) {
	cfg := load(t, `version: 1
host: {name: here}
forges:
  github: {token_ref: gh-agent, inbox_token_ref: gh-inbox, agent_identity: bot}
  gitlab: {base_url: "https://gl.test", token_ref: gl-agent, inbox_token_ref: gl-inbox, agent_identity: me}
trusted_authors: {github: [me], gitlab: [me]}
projects:
  - {ref: "gh:o/r", path: /src/r, host: here}
  - {ref: "gl:nas/a", path: /src/a, host: here}
  - {ref: "gl:nas/b", path: /src/b, host: here, token_ref: gl-bot, agent_identity: project_7_bot}
  - {ref: "gl:nas/elsewhere", path: /src/e, host: there, token_ref: gl-other, agent_identity: other_bot}
`)
	var asked []string
	c := newConn(cfg, tokens(&asked))
	gh, err := c.agentClient(cfg.Projects[0])
	if _, ok := gh.Forge.(*github.Client); err != nil || !ok || gh.GitToken != "token-gh-agent" {
		t.Fatalf("GitHub client = %T %q, %v", gh.Forge, gh.GitToken, err)
	}
	a, err := c.agentClient(cfg.Projects[1])
	if _, ok := a.Forge.(*gitlab.Client); err != nil || !ok || a.GitToken != "token-gl-agent" {
		t.Fatalf("GitLab client = %T %q, %v", a.Forge, a.GitToken, err)
	}
	b, err := c.agentClient(cfg.Projects[2])
	if err != nil || b.GitToken != "token-gl-bot" {
		t.Fatalf("override client token = %q, %v", b.GitToken, err)
	}
	inb, err := c.inboxForges("gitlab")
	if err != nil || len(inb) != 1 || inb["gl"] == nil {
		t.Fatalf("inbox forges = %v, %v", inb, err)
	}
	for _, r := range asked {
		if r == "gl-other" {
			t.Fatal("asked for the token of another host's project")
		}
	}
}
