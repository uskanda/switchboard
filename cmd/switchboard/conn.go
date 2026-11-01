package main

import (
	"context"
	"fmt"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/github"
	"github.com/uskanda/switchboard/internal/forge/gitlab"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

// conn builds the forge clients a command needs. A token is read only when a client that uses it is built, so a
// GitHub-only configuration never asks for a GitLab token.
type conn struct {
	cfg    *config.Config
	token  func(ref string) (string, error)
	tokens map[string]string
}

func newConn(cfg *config.Config, token func(string) (string, error)) *conn {
	return &conn{cfg: cfg, token: token, tokens: map[string]string{}}
}

func (c *conn) tokenOf(ref string) (string, error) {
	if t, ok := c.tokens[ref]; ok {
		return t, nil
	}
	t, err := c.token(ref)
	if err != nil {
		return "", err
	}
	c.tokens[ref] = t
	return t, nil
}

// client connects to the forge kind with the token named by ref.
func (c *conn) client(kind, ref string) (forge.Forge, string, error) {
	t, err := c.tokenOf(ref)
	if err != nil {
		return nil, "", err
	}
	if kind == forge.GitLab {
		return gitlab.New(t, c.cfg.Forges.GitLab.BaseURL), t, nil
	}
	return github.New(t), t, nil
}

// agentClient is the project's route with the agent's token: the project's override, or its forge's token_ref.
func (c *conn) agentClient(p config.Project) (agent.Route, error) {
	f, t, err := c.client(p.Forge, c.cfg.AgentTokenRef(p))
	if err != nil {
		return agent.Route{}, err
	}
	return agent.Route{Forge: f, GitToken: t}, nil
}

// agentRoutes builds the route of every project of this host, keyed by its ref, without judges.
func (c *conn) agentRoutes() (map[string]agent.Route, error) {
	routes := map[string]agent.Route{}
	for _, p := range c.cfg.ProjectsFor(c.cfg.Host.Name) {
		r, err := c.agentClient(p)
		if err != nil {
			return nil, err
		}
		routes[p.Ref] = r
	}
	return routes, nil
}

// judge resolves the trusted authors and agent identities of every route on one forge, and gives each of those
// routes the judge.
func (c *conn) judge(ctx context.Context, kind string, routes map[string]agent.Route, d state.Dir, audit state.Auditor) error {
	var first forge.Forge
	overrides := map[string]string{}
	for _, p := range c.cfg.ProjectsFor(c.cfg.Host.Name) {
		if p.Forge != kind {
			continue
		}
		if first == nil {
			first = routes[p.Ref].Forge
		}
		if p.AgentIdentity != "" {
			overrides[p.Ref] = p.AgentIdentity
		}
	}
	if first == nil {
		return nil
	}
	agentLogin := c.cfg.Forges.GitHub.AgentIdentity
	if kind == forge.GitLab {
		agentLogin = c.cfg.Forges.GitLab.AgentIdentity
	}
	j, err := trust.ResolveFor(ctx, first, d, kind, c.cfg.TrustedAuthorsOf(kind), agentLogin, overrides, audit)
	if err != nil {
		return err
	}
	for ref, r := range routes {
		if p := c.project(ref); p != nil && p.Forge == kind {
			r.Judge = j
			routes[ref] = r
		}
	}
	return nil
}

// reloadRoute returns the agent's NewRoute: a project that appears on a reload gets its forge's agent token and the
// judge resolved at start for that forge. The agent's tokens and judges are settled when it starts, so a forge no
// project of this host used then, or an agent identity of the project's own, waits for a restart.
func (c *conn) reloadRoute(routes map[string]agent.Route) func(config.Project) (agent.Route, error) {
	judges := map[string]*trust.Judge{}
	for ref, r := range routes {
		if p := c.project(ref); p != nil && r.Judge != nil {
			judges[p.Forge] = r.Judge
		}
	}
	return func(p config.Project) (agent.Route, error) {
		if p.TokenRef != "" || p.AgentIdentity != "" {
			return agent.Route{}, fmt.Errorf("%s has an agent identity of its own; restart the agent to take it on", p.Ref)
		}
		j, ok := judges[p.Forge]
		if !ok {
			return agent.Route{}, fmt.Errorf("no project of this host was on the forge of %s when the agent started; run switchboard service install again to give the agent its token", p.Ref)
		}
		r, err := c.agentClient(p)
		if err != nil {
			return agent.Route{}, err
		}
		r.Judge = j
		return r, nil
	}
}

func (c *conn) project(ref string) *config.Project {
	for i, p := range c.cfg.Projects {
		if p.Ref == ref {
			return &c.cfg.Projects[i]
		}
	}
	return nil
}

// inboxForges connects to each forge the configuration uses (only "github" or "gitlab" when set) with its
// inbox_token_ref.
func (c *conn) inboxForges(only string) (map[string]forge.Forge, error) {
	out := map[string]forge.Forge{}
	for _, k := range []struct{ kind, name string }{{forge.GitHub, "github"}, {forge.GitLab, "gitlab"}} {
		if !c.cfg.Uses(k.kind) || (only != "" && only != k.name) {
			continue
		}
		ref := c.cfg.InboxTokenRef(k.kind)
		if ref == "" {
			return nil, fmt.Errorf("forges.%s.inbox_token_ref is not set; the inbox reads and replies with your own token", k.name)
		}
		f, _, err := c.client(k.kind, ref)
		if err != nil {
			return nil, err
		}
		out[k.kind] = f
	}
	return out, nil
}
