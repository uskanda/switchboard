package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/forge/github"
	"github.com/uskanda/switchboard/internal/forge/gitlab"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
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

// With GitHub and GitLab in one configuration, each project trusts only the user IDs of its own forge: the same
// login is another account, with another ID, on the other forge.
func TestJudgeGivesEachProjectTheIDsOfItsOwnForge(t *testing.T) {
	cfg := load(t, `version: 1
host: {name: here}
forges:
  github: {token_ref: gh-agent, inbox_token_ref: gh-inbox, agent_identity: bot}
  gitlab: {base_url: "https://gl.test", token_ref: gl-agent, inbox_token_ref: gl-inbox, agent_identity: bot}
trusted_authors: {github: [me], gitlab: [me]}
projects:
  - {ref: "gh:o/r", path: /src/r, host: here}
  - {ref: "gl:nas/a", path: /src/a, host: here}
  - {ref: "gl:nas/b", path: /src/b, host: here, token_ref: gl-bot, agent_identity: project_7_bot}
`)
	gh := fake.New(nil)
	gh.AddUser("me", 1)
	gh.AddUser("bot", 2)
	gl := fake.NewGitLab(nil)
	gl.AddUser("me", 100)
	gl.AddUser("bot", 200)
	gl.AddUser("project_7_bot", 700)
	routes := map[string]agent.Route{"gh:o/r": {Forge: gh}, "gl:nas/a": {Forge: gl}, "gl:nas/b": {Forge: gl}}
	c := newConn(cfg, func(string) (string, error) { return "t", nil })
	d := state.Dir{Path: t.TempDir()}
	for _, kind := range []string{forge.GitHub, forge.GitLab} {
		if err := c.judge(context.Background(), kind, routes, d, nil); err != nil {
			t.Fatal(err)
		}
	}
	ghj, glj := routes["gh:o/r"].Judge, routes["gl:nas/a"].Judge
	if ghj == nil || glj == nil || routes["gl:nas/b"].Judge != glj {
		t.Fatalf("judges: gh %p, gl:nas/a %p, gl:nas/b %p", ghj, glj, routes["gl:nas/b"].Judge)
	}
	if !ghj.Trusted[1] || ghj.Trusted[100] || ghj.AgentID != 2 {
		t.Errorf("GitHub judge = %+v", ghj)
	}
	if !glj.Trusted[100] || glj.Trusted[1] || glj.AgentID != 200 {
		t.Errorf("GitLab judge = %+v", glj)
	}
	nasB := forge.IssueRef{Forge: forge.GitLab, Owner: "nas", Repo: "b", Number: 1}
	if glj.AgentIDOf(nasB) != 700 || ghj.AgentIDOf(forge.IssueRef{Owner: "o", Repo: "r"}) != 2 {
		t.Errorf("agent IDs: nas/b %d", glj.AgentIDOf(nasB))
	}
}

// A forge no project of this host is on gets no judge, and its client is never asked.
func TestJudgeOfAForgeWithoutProjectsDoesNothing(t *testing.T) {
	cfg := load(t, `version: 1
host: {name: here}
forges:
  github: {token_ref: gh-agent, inbox_token_ref: gh-inbox, agent_identity: bot}
trusted_authors: {github: [me]}
projects:
  - {ref: "gh:o/r", path: /src/r, host: here}
`)
	routes := map[string]agent.Route{"gh:o/r": {}}
	c := newConn(cfg, func(string) (string, error) { return "t", nil })
	if err := c.judge(context.Background(), forge.GitLab, routes, state.Dir{Path: t.TempDir()}, nil); err != nil || routes["gh:o/r"].Judge != nil {
		t.Fatalf("err = %v, judge = %v", err, routes["gh:o/r"].Judge)
	}
}

func TestReloadRouteUsesTheForgesTokenAndJudge(t *testing.T) {
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
	routes, err := c.agentRoutes()
	if err != nil {
		t.Fatal(err)
	}
	j := &trust.Judge{}
	r := routes["gh:o/r"]
	r.Judge = j
	routes["gh:o/r"] = r
	newRoute := c.reloadRoute(routes)

	got, err := newRoute(config.Project{Ref: "gh:o/notes", Forge: "gh", Owner: "o", Repo: "notes", Host: "here"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Judge != j || got.GitToken != "token-gh-agent" {
		t.Errorf("route = %+v, want the judge of GitHub and the agent token", got)
	}
	if _, err := newRoute(config.Project{Ref: "gl:g/p", Forge: "gl", Owner: "g", Repo: "p", Host: "here"}); err == nil || !strings.Contains(err.Error(), "service install") {
		t.Errorf("err = %v, want a forge new to this host refused with a hint to run service install", err)
	}
	if _, err := newRoute(config.Project{Ref: "gh:o/bot", Forge: "gh", Owner: "o", Repo: "bot", Host: "here", TokenRef: "x", AgentIdentity: "y"}); err == nil || !strings.Contains(err.Error(), "restart") {
		t.Errorf("err = %v, want an identity override refused until a restart", err)
	}
}
