// Package doctor checks that this host can run switchboard: the tokens, the user IDs, the projects of this host and
// the claude CLI, on every forge the configuration uses. The configuration itself is checked when it is loaded.
package doctor

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	ghforge "github.com/uskanda/switchboard/internal/forge/github"
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
	// GHVersion reads the version of the GitHub CLI; nil runs gh --version.
	GHVersion func(ctx context.Context) (string, error)

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
	for _, w := range cfg.Warnings() {
		d.warn("%s", w)
	}
	for _, kind := range []string{forge.GitHub, forge.GitLab} {
		if uses(cfg, kind) {
			d.forge(ctx, kind)
		}
	}
	d.claude(ctx)
	d.gh(ctx)
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
	agentKnown := false
	if token, err := d.Secrets.Token(tokenRef); err != nil {
		d.fail("agent token (%s %s): %v", label, tokenRef, err)
	} else {
		d.ok("agent token (%s %s) found", label, tokenRef)
		f = d.Forge(kind, token)
		agentKnown = d.scopes(ctx, f, kind, "agent token")
	}
	inboxLabel := "inbox_token_ref"
	if kind == forge.GitLab {
		inboxLabel = "forges.gitlab.inbox_token_ref"
	}
	if ref := cfg.InboxTokenRef(kind); ref == "" {
		d.warn("forges.%s.inbox_token_ref is not set: inbox, show, reply and merging from the web inbox need it", n)
	} else if token, err := d.Secrets.Token(ref); err != nil {
		d.fail("inbox token (%s %s): %v", inboxLabel, ref, err)
	} else {
		// The token's permissions are not read: a fine-grained PAT does not tell them. Merging is refused with 403
		// until they are given (ADR-043). Only the CI is tried, as the merge dialog reads it (ADR-050).
		needs := "Issues, Pull requests and Contents write, and Actions and Commit statuses read"
		if kind == forge.GitLab {
			needs = "the api scope"
		}
		d.ok("inbox token (%s %s) found; merging from the web inbox needs %s", inboxLabel, ref, needs)
		inf := d.Forge(kind, token)
		d.scopes(ctx, inf, kind, "inbox token")
		d.inboxCI(ctx, inf, kind)
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
		pf, known := f, agentKnown
		if p.TokenRef != "" {
			if token, err := d.Secrets.Token(p.TokenRef); err != nil {
				d.fail("%s: agent token (token_ref %s): %v", p.Ref, p.TokenRef, err)
				pf = nil
			} else {
				d.ok("%s: agent token (token_ref %s) found", p.Ref, p.TokenRef)
				pf = d.Forge(kind, token)
				known = d.scopes(ctx, pf, kind, p.Ref+": agent token")
			}
			if j != nil {
				d.ok("%s: agent identity %s is user ID %d (recorded)", p.Ref, p.AgentIdentity, j.AgentIDs[p.Ref])
			}
		}
		d.project(ctx, pf, p)
		if pf != nil && !known {
			d.probe(ctx, pf, p)
		}
	}
}

// broadGitHubScopes are classic token scopes beyond what the agent and the inbox need; a scope that starts with
// "admin:" is broad too.
var broadGitHubScopes = map[string]bool{"repo": true, "delete_repo": true, "workflow": true, "user": true,
	"write:packages": true, "delete:packages": true, "codespace": true, "site_admin": true}

// broadGitLabScopes are GitLab scopes beyond the api scope switchboard needs.
var broadGitLabScopes = map[string]bool{"sudo": true, "admin_mode": true}

// scopes checks the scopes of the token behind f, named who, and reports whether they are known (AGENTS.md: least
// privilege). A token without the scope switchboard needs fails; one with more than it needs is a warning. A GitHub
// fine-grained token tells nothing of its permissions: its reads are tried on each project instead (probe), and its
// writes, which cannot be tried without a side effect, are only named.
func (d *Doctor) scopes(ctx context.Context, f forge.Forge, kind, who string) bool {
	tp, ok := f.(forge.TokenProber)
	if !ok {
		return true
	}
	ts, err := tp.TokenScopes(ctx)
	if err != nil {
		d.warn("%s: its scopes could not be read: %v", who, err)
		return false
	}
	if !ts.Known {
		d.ok("%s is a fine-grained token: its permissions cannot be read, so its reads are tried on each project; its writes (Issues, Pull requests and Contents) cannot be checked without changing something", who)
		return false
	}
	var broad []string
	has := map[string]bool{}
	for _, s := range ts.Scopes {
		has[s] = true
		if (kind == forge.GitLab && broadGitLabScopes[s]) || (kind != forge.GitLab && (broadGitHubScopes[s] || strings.HasPrefix(s, "admin:"))) {
			broad = append(broad, s)
		}
	}
	list := strings.Join(ts.Scopes, ", ")
	switch {
	case kind == forge.GitLab && !has["api"]:
		d.fail("%s has the scopes [%s]; switchboard needs the api scope", who, list)
		return true
	case kind != forge.GitLab && !has["repo"] && !has["public_repo"]:
		d.fail("%s has the scopes [%s]; switchboard needs repo (private repositories) or public_repo", who, list)
		return true
	}
	if len(broad) > 0 {
		d.warn("%s has the scopes [%s]; %s reach beyond the projects (a GitHub fine-grained token limited to them is narrower)", who, list, strings.Join(broad, ", "))
		return true
	}
	d.ok("%s has the scopes [%s]", who, list)
	return true
}

// probe tries the reads the agent needs on p with a token whose permissions are not known.
func (d *Doctor) probe(ctx context.Context, f forge.Forge, p config.Project) {
	tp, ok := f.(forge.TokenProber)
	if !ok {
		return
	}
	for _, r := range tp.ProbeReads(ctx, p.Owner, p.Repo, cmp.Or(p.BaseBranch, "main")) {
		if r.Err != nil {
			d.fail("%s: the agent token cannot read the %s: %v", p.Ref, r.What, r.Err)
		} else {
			d.ok("%s: the agent token can read the %s", p.Ref, r.What)
		}
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
	d.ci(ctx, f, p)
	if !private {
		d.warn("%s is not private (public, or internal on GitLab): anyone who can see it can comment on its issues; only the input of trusted_authors is taken, include_untrusted is refused, and claude runs in the sandbox (ADR-060, docs/SECURITY.md)", p.Ref)
		return
	}
	d.ok("%s is private", p.Ref)
}

// ci checks that the agent token can read the CI of p, which the agent watches after it opens a change request.
// The base branch stands in for a commit: GitHub and GitLab both take a branch name there.
func (d *Doctor) ci(ctx context.Context, f forge.Forge, p config.Project) {
	c, ok := f.(forge.CI)
	if !ok {
		return
	}
	_, err := c.CommitChecks(ctx, p.Owner, p.Repo, cmp.Or(p.BaseBranch, "main"))
	switch {
	case errors.Is(err, forge.ErrForbidden):
		d.warn("%s: the agent token cannot read the CI, so the agent will not watch it: give the token Actions: Read and Commit statuses: Read (GitHub) (%v)", p.Ref, err)
	case err != nil:
		d.warn("%s: the CI could not be read with the agent token: %v", p.Ref, err)
	default:
		d.ok("%s: the agent token can read the CI", p.Ref)
	}
}

// inboxCI checks that the inbox token can read the CI of the first project on the forge, which the merge dialog of
// the web inbox shows (ADR-050). Without it the dialog shows no CI; nothing else changes, so it is only a warning.
func (d *Doctor) inboxCI(ctx context.Context, f forge.Forge, kind string) {
	c, ok := f.(forge.CI)
	if !ok {
		return
	}
	for _, p := range d.Config.Projects {
		if kindOf(p) != kind {
			continue
		}
		_, err := c.CommitChecks(ctx, p.Owner, p.Repo, cmp.Or(p.BaseBranch, "main"))
		switch {
		case errors.Is(err, forge.ErrForbidden):
			d.warn("%s: the inbox token cannot read the CI, so the merge dialog of the web inbox shows none: give the token Actions: Read and Commit statuses: Read (GitHub) (%v)", p.Ref, err)
		case err != nil:
			d.warn("%s: the CI could not be read with the inbox token: %v", p.Ref, err)
		default:
			d.ok("%s: the inbox token can read the CI", p.Ref)
		}
		return
	}
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

// gh checks the GitHub CLI the agent attaches the images of a done run with (ADR-051), when this host has a GitHub
// project. Without it the runs and their write-backs work as before; only the images are not attached.
func (d *Doctor) gh(ctx context.Context) {
	github := false
	for _, p := range d.Config.ProjectsFor(d.Config.Host.Name) {
		github = github || kindOf(p) == forge.GitHub
	}
	if !github {
		return
	}
	version := d.GHVersion
	if version == nil {
		version = func(ctx context.Context) (string, error) { return ghforge.GHVersion(ctx, "") }
	}
	v, err := version(ctx)
	switch {
	case err != nil:
		d.warn("gh: %v: the screenshots of a done run cannot be attached to GitHub issues without gh %s or later", err, ghforge.GHMinVersion)
	case !ghforge.GHAttaches(v):
		d.warn("gh %s is older than %s: the screenshots of a done run cannot be attached to GitHub issues", v, ghforge.GHMinVersion)
	default:
		d.ok("gh %s (attaches the screenshots of a done run)", v)
	}
}
