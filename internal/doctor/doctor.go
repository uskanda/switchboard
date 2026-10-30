// Package doctor checks that this host can run switchboard: the tokens, the user IDs, the projects of this host and
// the claude CLI, on every forge the configuration uses. The configuration itself is checked when it is loaded.
package doctor

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/secret"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

// Doctor runs the checks and writes one line per result.
type Doctor struct {
	Config  *config.Config
	State   state.Dir
	Secrets secret.Resolver
	// Forge connects to a forge (forge.GitHub or forge.GitLab) with a token.
	Forge func(kind, token string) forge.Forge
	Out   io.Writer

	failures int
}

// versioned is a forge that reports its version, as GitLab does.
type versioned interface {
	Version(ctx context.Context) (string, error)
}

// numbered is a forge whose projects have numeric IDs behind their paths, as GitLab's do.
type numbered interface {
	ProjectID(ctx context.Context, owner, repo string) (int64, error)
}

func (d *Doctor) ok(format string, args ...any)   { fmt.Fprintf(d.Out, "ok    "+format+"\n", args...) }
func (d *Doctor) warn(format string, args ...any) { fmt.Fprintf(d.Out, "warn  "+format+"\n", args...) }
func (d *Doctor) fail(format string, args ...any) {
	d.failures++
	fmt.Fprintf(d.Out, "fail  "+format+"\n", args...)
}

func name(kind string) string {
	if kind == forge.GitLab {
		return "gitlab"
	}
	return "github"
}

// Run runs every check and returns the number of failures. Warnings do not count.
func (d *Doctor) Run(ctx context.Context) int {
	d.failures = 0
	cfg := d.Config
	d.ok("configuration: host %s, %d project(s), %d on this host", cfg.Host.Name, len(cfg.Projects), len(cfg.ProjectsFor(cfg.Host.Name)))
	for _, kind := range []string{forge.GitHub, forge.GitLab} {
		if uses(cfg, kind) {
			d.forge(ctx, kind)
		}
	}
	d.claude(ctx)
	return d.failures
}

// uses reports whether a project is on the forge; a project without a forge (as Phase 1 code built them) is GitHub.
func uses(cfg *config.Config, kind string) bool {
	for _, p := range cfg.Projects {
		if kindOf(p) == kind {
			return true
		}
	}
	return false
}

func kindOf(p config.Project) string {
	if p.Forge == "" {
		return forge.GitHub
	}
	return p.Forge
}

// forge checks one forge: its agent and inbox tokens, the user IDs, and the projects of this host on it.
func (d *Doctor) forge(ctx context.Context, kind string) {
	cfg := d.Config
	n := name(kind)
	tokenRef, agentLogin := cfg.Forges.GitHub.TokenRef, cfg.Forges.GitHub.AgentIdentity
	label := "token_ref"
	if kind == forge.GitLab {
		tokenRef, agentLogin = cfg.Forges.GitLab.TokenRef, cfg.Forges.GitLab.AgentIdentity
		label = "forges.gitlab.token_ref"
	}
	var f forge.Forge
	if token, err := d.Secrets.Token(tokenRef); err != nil {
		d.fail("agent token (%s %s): %v", label, tokenRef, err)
	} else {
		d.ok("agent token (%s %s) found", label, tokenRef)
		f = d.Forge(kind, token)
	}
	inboxLabel := "inbox_token_ref"
	if kind == forge.GitLab {
		inboxLabel = "forges.gitlab.inbox_token_ref"
	}
	if ref := cfg.InboxTokenRef(kind); ref == "" {
		d.warn("forges.%s.inbox_token_ref is not set: inbox, show and reply need it", n)
	} else if _, err := d.Secrets.Token(ref); err != nil {
		d.fail("inbox token (%s %s): %v", inboxLabel, ref, err)
	} else {
		d.ok("inbox token (%s %s) found", inboxLabel, ref)
	}
	if v, ok := f.(versioned); ok && kind == forge.GitLab {
		if ver, err := v.Version(ctx); err != nil {
			d.fail("%s %s: %v", n, cfg.Forges.GitLab.BaseURL, err)
		} else {
			d.ok("%s %s is version %s", n, cfg.Forges.GitLab.BaseURL, ver)
		}
	}

	var projects []config.Project
	overrides := map[string]string{}
	for _, p := range cfg.ProjectsFor(cfg.Host.Name) {
		if kindOf(p) == kind {
			projects = append(projects, p)
			if p.AgentIdentity != "" {
				overrides[p.Ref] = p.AgentIdentity
			}
		}
	}
	var j *trust.Judge
	if f != nil {
		j = d.identities(ctx, f, kind, agentLogin, overrides)
	}
	for _, p := range projects {
		pf := f
		if p.TokenRef != "" {
			if token, err := d.Secrets.Token(p.TokenRef); err != nil {
				d.fail("%s: agent token (token_ref %s): %v", p.Ref, p.TokenRef, err)
				pf = nil
			} else {
				d.ok("%s: agent token (token_ref %s) found", p.Ref, p.TokenRef)
				pf = d.Forge(kind, token)
			}
			if j != nil {
				d.ok("%s: agent identity %s is user ID %d (recorded)", p.Ref, p.AgentIdentity, j.AgentIDs[p.Ref])
			}
		}
		d.project(ctx, pf, p)
	}
}

func (d *Doctor) identities(ctx context.Context, f forge.Forge, kind, agentLogin string, overrides map[string]string) *trust.Judge {
	if err := d.State.Ensure(); err != nil {
		d.fail("state directory %s: %v", d.State.Path, err)
		return nil
	}
	cfg := d.Config
	j, err := trust.ResolveFor(ctx, f, d.State, kind, cfg.TrustedAuthorsOf(kind), agentLogin, overrides, nil)
	if err != nil {
		d.fail("user IDs: %v", err)
		return nil
	}
	ids, _ := d.State.Identities()
	prefix := ""
	if kind == forge.GitLab {
		prefix = "gitlab: "
	}
	for _, login := range cfg.TrustedAuthorsOf(kind) {
		d.ok("%strusted author %s is user ID %d (recorded)", prefix, login, ids[kind+":"+login])
	}
	d.ok("%sagent identity %s is user ID %d (recorded)", prefix, agentLogin, j.AgentID)
	return j
}

func (d *Doctor) project(ctx context.Context, f forge.Forge, p config.Project) {
	cmd := exec.CommandContext(ctx, "git", "-C", p.Path, "rev-parse", "--git-dir")
	if out, err := cmd.CombinedOutput(); err != nil {
		d.fail("%s: %s is not a git repository: %s", p.Ref, p.Path, out)
	} else {
		d.ok("%s: %s is a git repository", p.Ref, p.Path)
	}
	if f == nil {
		return
	}
	if nf, ok := f.(numbered); ok && kindOf(p) == forge.GitLab {
		id, err := nf.ProjectID(ctx, p.Owner, p.Repo)
		if err != nil {
			d.fail("%s: the project cannot be read with the agent token: %v", p.Ref, err)
			return
		}
		d.ok("%s is project ID %d", p.Ref, id)
	}
	private, err := f.RepoPrivate(ctx, p.Owner, p.Repo)
	if err != nil {
		d.fail("%s: the repository cannot be read with the agent token: %v", p.Ref, err)
		return
	}
	if _, err := trust.UntrustedPolicy(p.IncludeUntrusted, private); err != nil {
		d.fail("%s: %v", p.Ref, err)
		return
	}
	if !private {
		d.warn("%s is not private (public, or internal on GitLab): anyone who can see it can comment on its issues, and claude runs with your account (docs/SECURITY.md)", p.Ref)
		return
	}
	d.ok("%s is private", p.Ref)
}

func (d *Doctor) claude(ctx context.Context) {
	bin := d.Config.Claude.Binary
	v, err := claudecode.Version(ctx, bin)
	if err != nil {
		d.fail("claude: %v", err)
		return
	}
	if claudecode.Older(v, claudecode.CheckedVersion) {
		d.warn("claude %s is older than %s, the version switchboard was checked against", v, claudecode.CheckedVersion)
		return
	}
	d.ok("claude %s (checked against %s)", v, claudecode.CheckedVersion)
}
