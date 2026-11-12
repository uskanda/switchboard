package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/workspace"
)

// setup runs the project's setup command cmd in the worktree before claude starts: outside the run's sandbox, with
// the verify command's environment (no forge token) and under agent.timeout (ADR-070). why is empty when claude may
// start; otherwise it says, for the issue, that the command failed, ran out of time, or left changes git sees in
// the worktree. Those changes are kept for a person to look at. canceled reports a run that was canceled meanwhile.
// The output goes to setup.log in the run's directory, and setup.ran to the audit log without it.
func (a *Agent) setup(ctx context.Context, sp *workspace.Space, ref forge.IssueRef, run, cmd string) (why string, canceled bool) {
	fail := func(err error) (string, bool) {
		if ctx.Err() != nil {
			return "", true
		}
		return fmt.Sprintf(a.msg.prepare, err), false
	}
	if err := os.MkdirAll(a.runDir(run), 0o700); err != nil {
		return fail(err)
	}
	before, err := sp.State(ctx)
	if err != nil {
		return fail(err)
	}
	limit := a.Config.Agent.Timeout
	sctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	start := time.Now()
	out, runErr := a.shell(sctx, sp.Dir(), claudecode.Env(a.Env, "subscription"), cmd)
	if ctx.Err() != nil {
		return "", true
	}
	ms := time.Since(start).Milliseconds()
	_ = os.WriteFile(filepath.Join(a.runDir(run), "setup.log"), []byte(out), 0o600)
	ran := func(reason string) {
		fields := map[string]any{"issue": ref.String(), "run": run, "command": cmd, "ms": ms, "ok": reason == ""}
		if reason != "" {
			fields["reason"] = reason
		}
		a.Audit.Log("setup.ran", fields)
	}
	output := "\n\n```\n" + tail(out) + "\n```"
	switch {
	case sctx.Err() != nil:
		ran("timeout")
		return fmt.Sprintf(a.msg.setupTimeout, cmd, limit) + output, false
	case runErr != nil:
		ran("exit")
		return fmt.Sprintf(a.msg.setupFailed, cmd) + output, false
	}
	after, err := sp.State(ctx)
	if err != nil {
		return fail(err)
	}
	paths, changed := workspace.Changed(before, after)
	if !changed {
		ran("")
		return "", false
	}
	ran("changed")
	if len(paths) == 0 {
		return fmt.Sprintf(a.msg.setupChangedContent, cmd), false
	}
	return fmt.Sprintf(a.msg.setupChanged, cmd) + "\n\n- `" + strings.Join(paths, "`\n- `") + "`", false
}
