// Package doctor checks that this host can run switchboard: the tokens, the user IDs, the projects of this host and
// the claude CLI. The configuration itself is checked when it is loaded.
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
	// Forge connects to the forge with a token.
	Forge func(token string) forge.Forge
	Out   io.Writer

	failures int
}

func (d *Doctor) ok(format string, args ...any)   { fmt.Fprintf(d.Out, "ok    "+format+"\n", args...) }
func (d *Doctor) warn(format string, args ...any) { fmt.Fprintf(d.Out, "warn  "+format+"\n", args...) }
func (d *Doctor) fail(format string, args ...any) {
	d.failures++
	fmt.Fprintf(d.Out, "fail  "+format+"\n", args...)
}

// Run runs every check and returns the number of failures. Warnings do not count.
func (d *Doctor) Run(ctx context.Context) int {
	d.failures = 0
	cfg := d.Config
	gh := cfg.Forges.GitHub
	d.ok("configuration: host %s, %d project(s), %d on this host", cfg.Host.Name, len(cfg.Projects), len(cfg.ProjectsFor(cfg.Host.Name)))

	var f forge.Forge
	if token, err := d.Secrets.Token(gh.TokenRef); err != nil {
		d.fail("agent token (token_ref %s): %v", gh.TokenRef, err)
	} else {
		d.ok("agent token (token_ref %s) found", gh.TokenRef)
		f = d.Forge(token)
	}
	if gh.InboxTokenRef == "" {
		d.warn("forges.github.inbox_token_ref is not set: inbox, show and reply need it")
	} else if _, err := d.Secrets.Token(gh.InboxTokenRef); err != nil {
		d.fail("inbox token (inbox_token_ref %s): %v", gh.InboxTokenRef, err)
	} else {
		d.ok("inbox token (inbox_token_ref %s) found", gh.InboxTokenRef)
	}

	if f != nil {
		d.identities(ctx, f)
	}
	for _, p := range cfg.ProjectsFor(cfg.Host.Name) {
		d.project(ctx, f, p)
	}
	d.claude(ctx)
	return d.failures
}

func (d *Doctor) identities(ctx context.Context, f forge.Forge) {
	if err := d.State.Ensure(); err != nil {
		d.fail("state directory %s: %v", d.State.Path, err)
		return
	}
	cfg := d.Config
	j, err := trust.Resolve(ctx, f, d.State, cfg.TrustedAuthors.GitHub, cfg.Forges.GitHub.AgentIdentity, nil)
	if err != nil {
		d.fail("user IDs: %v", err)
		return
	}
	ids, _ := d.State.Identities()
	for _, login := range cfg.TrustedAuthors.GitHub {
		d.ok("trusted author %s is user ID %d (recorded)", login, ids[login])
	}
	d.ok("agent identity %s is user ID %d (recorded)", cfg.Forges.GitHub.AgentIdentity, j.AgentID)
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
		d.warn("%s is public: anyone can comment on its issues, and claude runs with your account (docs/SECURITY.md)", p.Ref)
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
