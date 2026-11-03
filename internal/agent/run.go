package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/instructions"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/prompt"
	"github.com/uskanda/switchboard/internal/trust"
	"github.com/uskanda/switchboard/internal/workspace"
)

// Schema is the JSON schema of the declaration that ends a run.
const Schema = `{"type":"object","additionalProperties":false,"required":["status","summary"],` +
	`"properties":{"status":{"type":"string","enum":["waiting","done"]},` +
	`"summary":{"type":"string"},"question":{"type":"string"}}}`

// Declaration is how a run ends, as claude declared it.
type Declaration struct {
	Status   string `json:"status"`
	Summary  string `json:"summary"`
	Question string `json:"question,omitempty"`
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// structured returns the structured output of res, or says why the run ended abnormally.
func structured(res *claudecode.Result) ([]byte, string) {
	switch {
	case !res.HasResult:
		return nil, fmt.Sprintf("no result message (exit code %d)", res.ExitCode)
	case res.IsError:
		return nil, res.Subtype
	case res.ExitCode != 0:
		return nil, fmt.Sprintf("exit code %d", res.ExitCode)
	case res.Structured == nil:
		return nil, "no structured output"
	}
	return res.Structured, ""
}

// declaration reads the declaration from res, or says why the run ended abnormally.
func declaration(res *claudecode.Result) (*Declaration, string) {
	raw, problem := structured(res)
	if problem != "" {
		return nil, problem
	}
	var d Declaration
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, "the structured output does not match the schema: " + err.Error()
	}
	if d.Status != "waiting" && d.Status != "done" {
		return nil, fmt.Sprintf("the structured output has the status %q", d.Status)
	}
	if strings.TrimSpace(d.Summary) == "" {
		return nil, "the structured output has no summary"
	}
	return &d, ""
}

// settings are what a run may do without asking. A run may merge, to resolve a conflict with the base (ADR-042) or
// take in origin's issue branch after a refused push; no run may rebase. extra are the user's added allow rules
// (agent.allow_tools_extra, then the project's), each taken once; the deny rules still win over them (V42). The deny
// rules are not a boundary (Phase 0, V5); keeping the forge token away from claude is.
func settings(verifyCommand string, extra ...[]string) claudecode.Settings {
	base := baseAllow(verifyCommand)
	var unsandboxed []string
	if v := strings.Join(strings.Fields(verifyCommand), " "); v != "" {
		unsandboxed = append(unsandboxed, v)
	}
	return claudecode.Settings{
		Allow:       append(base, extraAllow(base, extra...)...),
		Deny:        []string{"Bash(git push)", "Bash(git push *)", "Bash(git rebase)", "Bash(git rebase *)", "Bash(gh)", "Bash(gh *)", "WebFetch"},
		Unsandboxed: append(unsandboxed, gitUnsandboxed...),
	}
}

// gitUnsandboxed lets git run outside the sandbox: a worktree's .git points into the clone's .git/worktrees/,
// which lies outside the worktree, so that git commit cannot write from inside. The verify command of a run that
// writes files also runs outside, whole, so that another target of the same tool (make deploy) stays inside; the
// verify command may need what the sandbox withholds, such as unix sockets.
var gitUnsandboxed = []string{"git", "git *"}

// credentials are the paths under the home directory that the commands in the sandbox may not read.
var credentials = []string{
	".ssh", ".gnupg", ".git-credentials", ".netrc",
	".config/gh", ".config/glab-cli",
	".aws", ".kube", ".docker/config.json",
}

// denyRead returns the paths the commands of a run in worktree may not read: the credentials under home, the
// directory of the configuration file, and every entry of the state directory but the worktrees. Only paths that
// exist are listed, since the sandbox may leave an empty file at a path it hides (V46, V52). A path that holds the
// worktree is left out.
func denyRead(home, configPath, stateDir, worktree string) []string {
	var paths []string
	if home != "" {
		for _, c := range credentials {
			paths = append(paths, filepath.Join(home, c))
		}
	}
	if configPath != "" {
		if abs, err := filepath.Abs(configPath); err == nil {
			paths = append(paths, filepath.Dir(abs))
		}
	}
	if entries, err := os.ReadDir(stateDir); err == nil {
		for _, e := range entries {
			if e.Name() != "worktrees" {
				paths = append(paths, filepath.Join(stateDir, e.Name()))
			}
		}
	}
	out := []string{}
	for _, p := range paths {
		if _, err := os.Lstat(p); err != nil || within(worktree, p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// within reports whether path is dir or lies under it.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// homeOf returns HOME from env, the environment claude is started from, or this process's home directory.
func homeOf(env []string) string {
	if env == nil {
		env = os.Environ()
	}
	for _, kv := range slices.Backward(env) {
		if v, ok := strings.CutPrefix(kv, "HOME="); ok {
			return v
		}
	}
	h, _ := os.UserHomeDir()
	return h
}

// runTools are the tools every run that writes files gets; planTools those of a plan run. A tool the session lacks
// cannot be called, whatever the allow rules say, and every tool left out shortens the system prompt.
var (
	runTools  = []string{"Bash", "Read", "Edit", "Write", "Glob", "Grep", "WebSearch"}
	planTools = []string{"Bash", "Read", "Glob", "Grep", "WebSearch"}
)

// tools returns base followed by the tool names of the extra allow rules (Bash for "Bash(openspec *)"), each once.
func tools(base []string, extra ...[]string) []string {
	out := slices.Clone(base)
	for _, rules := range extra {
		for _, r := range rules {
			name, _, _ := strings.Cut(strings.TrimSpace(r), "(")
			if name = strings.TrimSpace(name); name != "" && !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	}
	return out
}

// modelKind is the kind of run whose model and effort level a run of kind k uses.
func modelKind(k runKind) config.RunKind {
	switch k {
	case planRun:
		return config.PlanRun
	case fixRun:
		return config.FixRun
	}
	return config.ImplementRun
}

// baseAllow are the allow rules every run that writes files gets.
func baseAllow(verifyCommand string) []string {
	allow := []string{"Read", "Edit", "Write", "Glob", "Grep", "WebSearch"}
	for _, c := range []string{"git status", "git diff", "git log", "git add", "git commit", "git merge"} {
		allow = append(allow, "Bash("+c+")", "Bash("+c+" *)")
	}
	if f := strings.Fields(verifyCommand); len(f) > 0 {
		allow = append(allow, "Bash("+f[0]+")", "Bash("+f[0]+" *)")
	}
	return allow
}

// extraAllow returns the rules of extra that settings adds after base, in order: trimmed, each taken once, and none
// that base already has. It is never nil, so that the audit log records an empty list.
func extraAllow(base []string, extra ...[]string) []string {
	added := []string{}
	for _, rules := range extra {
		for _, r := range rules {
			if r = strings.TrimSpace(r); !slices.Contains(base, r) && !slices.Contains(added, r) {
				added = append(added, r)
			}
		}
	}
	return added
}

func (a *Agent) space(pl *plan) *workspace.Space {
	p, ref := pl.project, pl.issue.Ref
	return a.spaceOf(p, ref)
}

// spaceOf is the worktree of ref. GitHub keeps the Phase 1 place; GitLab's sit under "@gitlab", which no GitHub
// owner can be, and push with the user name GitLab expects for a token.
func (a *Agent) spaceOf(p config.Project, ref forge.IssueRef) *workspace.Space {
	root := filepath.Join(a.State.Path, "worktrees", ref.Owner, ref.Repo)
	user := ""
	if ref.Kind() == forge.GitLab {
		root = filepath.Join(a.State.Path, "worktrees", "@gitlab", ref.Owner, ref.Repo)
		user = "oauth2"
	}
	return &workspace.Space{
		Repo: p.Path, Root: root, Base: p.BaseBranch, Number: ref.Number, Env: a.Env,
		Token: a.route(ref).GitToken, TokenUser: user,
	}
}

// execute runs pl and writes the outcome back. It returns the state written ("waiting", "done"), or "" when the run
// was canceled or the write-back failed.
func (a *Agent) execute(ctx context.Context, pl *plan, act *active) string {
	ref := pl.issue.Ref
	rec := record{Issue: ref.String(), Run: act.run, Session: newUUID(), State: "running", Started: act.started, Ack: pl.ack()}
	a.saveRecord(ref, rec)
	o := a.work(ctx, pl, rec)
	for o != nil && o.next != nil {
		o = a.fixAgain(ctx, pl, act, &rec, o)
	}
	if o == nil {
		rec.State, rec.Ended = "canceled", time.Now().UTC()
		a.saveRecord(ref, rec)
		a.Audit.Log("run.canceled", map[string]any{"issue": ref.String(), "run": rec.Run})
		if a.ctx.Err() == nil {
			if _, comments, err := a.forgeOf(ref).Thread(a.ctx, ref); err == nil {
				a.dropProgress(a.ctx, ref, rec.Run, comments)
			}
		}
		a.logf("%s: run %s stopped without writing back", ref, rec.Run)
		return ""
	}
	if err := a.writeBack(a.ctx, ref, o); err != nil {
		if a.ctx.Err() != nil {
			a.logf("%s: write back run %s: %v; it is finished when the agent starts again", ref, rec.Run, err)
			return ""
		}
		a.keepWriteBack(ref, &pendingWriteBack{project: pl.project, o: o}, rec, err)
		a.retune() // tried again within poll_interval, also while the loop waits for reconcile_interval
		return ""
	}
	a.ended(pl.project, ref, rec, o)
	return o.State
}

// ended records a run whose outcome was written back.
func (a *Agent) ended(p config.Project, ref forge.IssueRef, rec record, o *outcome) {
	if ref.Kind() == forge.GitLab {
		a.keepNotes(a.ctx, p, ref)
	}
	rec.State, rec.Ended, rec.Result = "finished", time.Now().UTC(), o.result()
	a.saveRecord(ref, rec)
	_ = a.State.WriteJSON("last.json", rec)
	a.logf("%s: run %s ended: %s", ref, rec.Run, rec.Result)
	if o.State == "done" && o.SHA != "" && o.CR != 0 {
		a.startWatch(p, ref, o.SHA, o.CR, a.now())
	}
}

// implements reports whether pl implements the issue: a direct run or the run of an approved plan. Only these get the
// implement instructions and the screenshot rules (ADR-051).
func (pl *plan) implements() bool { return pl.kind == directRun || pl.kind == approvedRun }

// work prepares the worktree, runs claude and judges the result. It returns nil when the run was canceled.
func (a *Agent) work(ctx context.Context, pl *plan, rec record) (o *outcome) {
	p, is := pl.project, pl.issue
	o = &outcome{Run: rec.Run, Session: rec.Session, Ack: rec.Ack, msg: a.msg}
	if pl.kind == conflictRun {
		var starts []string
		for _, l := range is.Labels {
			if labels.IsStart(l, a.Config.Labels) {
				starts = append(starts, "`"+l+"`")
			}
		}
		return o.fail(fmt.Sprintf(a.msg.multipleStarts, strings.Join(starts, ", ")))
	}
	sp := a.space(pl)
	dir, err := sp.Prepare(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return o.fail(fmt.Sprintf(a.msg.prepare, err))
	}
	if err := sp.CheckClaudeConfig(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return o.verify(fmt.Sprintf(a.msg.configChanged, err))
	}
	if pl.implements() {
		if err := sp.PrepareScreenshots(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return o.fail(fmt.Sprintf(a.msg.prepare, err))
		}
		// The images of a run that does not end done are not attached; those of a run handed to a fix run of a
		// refused push are, when the fix run ends done.
		defer func() {
			if o != nil && o.State != "done" && o.next == nil {
				_ = sp.ClearScreenshots()
			}
		}()
	}
	include := p.IncludeUntrusted && a.isPrivate(p.Ref)
	in := prompt.Input{Ref: is.Ref, Title: is.Title, Body: is.Body, IncludeUntrusted: include, Nonce: newID(),
		Resumed: pl.resumeNote != 0 && pl.kind != planRun}
	for i, c := range pl.comments {
		var k prompt.Kind
		switch pl.kinds[i] {
		case trust.Instruction:
			k = prompt.Trusted
		case trust.AgentPost:
			k = prompt.Previous
		default:
			k = prompt.Untrusted
		}
		in.Comments = append(in.Comments, prompt.Comment{Kind: k, ID: c.ID, Author: c.Author.Login, Body: c.Body})
	}
	lang, word := a.Config.Comments.Language, a.Config.Plan.ApproveWord
	base := baseAllow(p.VerifyCommand)
	added := extraAllow(base, a.Config.Agent.AllowToolsExtra, p.AllowToolsExtra)
	req := claudecode.Request{
		Binary: a.Config.Claude.Binary, Dir: dir, RunDir: a.runDir(rec.Run), SessionID: rec.Session,
		SystemPrompt: prompt.System(lang, p.BaseBranch), Settings: settings(p.VerifyCommand, a.Config.Agent.AllowToolsExtra, p.AllowToolsExtra), MaxTurns: a.Config.Agent.MaxTurns,
		Schema: Schema, Auth: a.Config.Claude.Auth, Timeout: a.Config.Agent.Timeout,
		IntGrace: a.IntGrace, TermGrace: a.TermGrace, Env: a.Env,
		Tools: tools(runTools, a.Config.Agent.AllowToolsExtra, p.AllowToolsExtra),
	}
	m := a.Config.Agent.ModelFor(modelKind(pl.kind))
	req.Model, req.Effort = m.Model, m.Effort
	head := ""
	var used, impl instructions.Loaded
	switch pl.kind {
	case planRun:
		in.Task = prompt.PlanTask
		used, added = a.loadInstructions(instructions.Plan, p), []string{}
		req.SystemPrompt, req.Settings, req.Schema = prompt.PlanSystem(lang, word, used.Text), planSettings(), PlanSchema
		req.Tools = planTools
		if head, err = sp.Head(ctx); err != nil {
			return o.fail(fmt.Sprintf(a.msg.prepare, err))
		}
	case approvedRun:
		req.SystemPrompt = prompt.ApprovedSystem(lang, p.BaseBranch, pl.planID, pl.okID)
	case fixRun:
		in.Task, in.Failure = prompt.FixTask, pl.fix.report
		req.SystemPrompt = prompt.FixSystem(lang, p.BaseBranch, pl.fix.cause)
	}
	if pl.kind != planRun {
		used = a.loadInstructions(instructions.Run, p)
		extra := used.Text
		if pl.implements() {
			impl = a.loadInstructions(instructions.Implement, p)
			extra = strings.TrimSpace(extra + "\n\n" + impl.Text)
			req.SystemPrompt = prompt.WithScreenshots(req.SystemPrompt)
		}
		req.SystemPrompt = prompt.WithSandbox(req.SystemPrompt, p.VerifyCommand)
		req.SystemPrompt = prompt.WithUserInstructions(req.SystemPrompt, extra)
	}
	req.Prompt = prompt.Build(in)
	// The run's directory is made first, so that the runs are hidden from the first run on.
	if err := os.MkdirAll(req.RunDir, 0o700); err != nil {
		return o.fail(fmt.Sprintf(a.msg.prepare, err))
	}
	req.Settings.DenyRead = denyRead(homeOf(a.Env), a.Config.Path, a.State.Path, dir)
	if err := a.checkSandbox(ctx); err != nil {
		return o.fail(fmt.Sprintf(a.msg.sandbox, err))
	}
	a.logStarted(is.Ref, rec.Run, pl, added, used, impl)
	stopProgress := a.followProgress(ctx, is.Ref, rec, filepath.Join(a.runDir(rec.Run), "stream.jsonl"))
	res, err := claudecode.Run(ctx, req)
	stopProgress()
	a.addCost(res)
	discarded := false
	if head != "" {
		// A plan run changes nothing; whatever it changed anyway is thrown away, also when it failed.
		var rerr error
		if discarded, rerr = sp.Restore(a.ctx, head); rerr != nil {
			return o.fail(fmt.Sprintf(a.msg.prepare, rerr))
		}
	}
	switch {
	case err != nil:
		if ctx.Err() != nil {
			return nil
		}
		return o.fail(fmt.Sprintf(a.msg.start, err))
	case res.Canceled:
		return nil
	case res.TimedOut:
		return o.fail(fmt.Sprintf(a.msg.timeout, a.Config.Agent.Timeout))
	case res.Limit != nil:
		// The service refused the run on a limit: pause the host, and put the issue back in the queue to resume once
		// the pause ends (ADR-053). A fix run cannot wait in the queue, so it waits for a reply.
		until := a.pauseForLimit(res.Limit)
		a.logf("%s: run %s hit a usage limit (%s): %s", is.Ref, rec.Run, res.Limit.Kind, res.Limit.Message)
		a.Audit.Log("run.limited", map[string]any{"issue": is.Ref.String(), "run": rec.Run, "kind": res.Limit.Kind, "until": until.UTC().Format(time.RFC3339)})
		if pl.kind == fixRun {
			return o.limitedFix(a.Config.Host.Name, until)
		}
		return o.limited(a.Config.Host.Name, until)
	}
	if pl.kind == planRun {
		return a.planOutcome(res, pl, o, discarded)
	}
	d, problem := declaration(res)
	switch {
	case res.Subtype == "error_max_turns":
		return o.fail(fmt.Sprintf(a.msg.maxTurns, a.Config.Agent.MaxTurns))
	case problem != "" && res.IsError:
		return o.fail(fmt.Sprintf(a.msg.failed, problem))
	case problem != "":
		return o.fail(fmt.Sprintf(a.msg.noDeclaration, problem))
	case d.Status == "waiting":
		return o.wait(d, res.Denials)
	}
	o = a.finish(ctx, pl, sp, d, o)
	if o != nil && o.State == "done" {
		a.collectShots(is.Ref, sp, o)
	}
	return o
}

// collectShots moves the images a done run left for the issue to its run directory, where the write-back reads them
// (ADR-051), and says on the done post that they follow. A failure is recorded; it never undoes the done.
func (a *Agent) collectShots(ref forge.IssueRef, sp *workspace.Space, o *outcome) {
	names, skipped, err := sp.CollectScreenshots(filepath.Join(a.runDir(o.Run), "screenshots"))
	if err != nil {
		a.logf("%s: collect the screenshots of run %s: %v", ref, o.Run, err)
		a.Audit.Log("attachment.failed", map[string]any{"issue": ref.String(), "run": o.Run, "error": err.Error()})
		o.ShotsError = err.Error()
	}
	o.Shots, o.Skipped = names, skipped
	if len(names) > 0 || len(skipped) > 0 || o.ShotsError != "" {
		o.Body += "\n\n" + a.msg.shotsNext
	}
}

// finish handles a run that declared done: verify, check the output, push, open the pull request.
func (a *Agent) finish(ctx context.Context, pl *plan, sp *workspace.Space, d *Declaration, o *outcome) *outcome {
	p, is := pl.project, pl.issue
	if pl.fix != nil {
		// The done post names what the fix run fixed; the causes are written as the marker's values (fix=).
		o.Fix = pl.fix.cause
	}
	verified := a.msg.notVerified
	if cmd := p.VerifyCommand; cmd != "" {
		// The command runs code claude wrote, so it gets claude's environment: no forge token.
		out, err := a.shell(ctx, sp.Dir(), claudecode.Env(a.Env, "subscription"), cmd)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return o.verify(fmt.Sprintf(a.msg.verifyFailed, cmd) + "\n\n```\n" + tail(out) + "\n```")
		}
		verified = fmt.Sprintf(a.msg.verified, cmd)
	}
	body := fmt.Sprintf("Closes #%d\n\n## %s\n\n%s\n\n## %s\n\n%s\n", is.Ref.Number, a.msg.prSummary, d.Summary, a.msg.prVerification, verified)
	problems, err := sp.CheckOutput(ctx, p.ProtectedPathsExtra, []string{d.Summary, body})
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return o.fail(fmt.Sprintf(a.msg.pushFailed, err))
	}
	if len(problems) > 0 {
		return o.verify(a.msg.notPushed + "\n\n- " + strings.Join(problems, "\n- "))
	}
	if err := a.push(ctx, sp); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		var pe *workspace.PushError
		if !errors.As(err, &pe) {
			return o.fail(fmt.Sprintf(a.msg.pushFailed, err))
		}
		out := workspace.Redact(pe.Output)
		if pe.Kind == workspace.PushTransient {
			return o.fail(fmt.Sprintf(a.msg.pushFailed, "\n\n```\n"+tail(out)+"\n```"))
		}
		made, limit := pl.fixesDone(), a.Config.Agent.FixAttempts()
		if pe.Kind == workspace.PushNonFastForward && made < limit {
			if err := sp.FetchIssueBranch(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return o.fail(fmt.Sprintf(a.msg.pushFailed, workspace.Redact(err.Error())))
			}
			return o.again(&fixInfo{cause: prompt.FixPush, report: out})
		}
		return o.pushRefused(pe.Kind, out, made, limit)
	}
	sha, err := sp.Head(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return o.fail(fmt.Sprintf(a.msg.pushFailed, err))
	}
	// A rerun pushes to the pull request an earlier run opened, such as a run resumed after done (ADR-042); opening
	// a second one would fail.
	gitlab := is.Ref.Kind() == forge.GitLab
	f := a.forgeOf(is.Ref)
	url, err := f.OpenChangeRequest(ctx, is.Ref.Owner, is.Ref.Repo, sp.IssueBranch(), p.BaseBranch)
	switch {
	case err == nil:
		line := a.msg.prUpdated
		if gitlab {
			line = a.msg.mrUpdated
		}
		return o.done(d, url, sha, fmt.Sprintf(line, url))
	case errors.Is(err, forge.ErrNotFound):
		url, err = f.CreateChangeRequest(ctx, forge.ChangeRequest{
			Owner: is.Ref.Owner, Repo: is.Ref.Repo, Title: is.Title, Body: body,
			Head: sp.IssueBranch(), Base: p.BaseBranch, Draft: true,
		})
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		if gitlab {
			return o.fail(fmt.Sprintf(a.msg.mrFailed, err))
		}
		return o.fail(fmt.Sprintf(a.msg.prFailed, err))
	}
	if gitlab {
		return o.done(d, url, sha, fmt.Sprintf(a.msg.mr, url))
	}
	return o.done(d, url, sha, fmt.Sprintf(a.msg.pr, url))
}

// shell runs cmd with sh in dir and env, within the run timeout, and returns its combined output.
func (a *Agent) shell(ctx context.Context, dir string, env []string, cmd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.Config.Agent.Timeout)
	defer cancel()
	c := exec.CommandContext(ctx, "sh", append([]string{"-c", cmd, "sh"}, args...)...)
	c.Dir = dir
	c.Env = env
	out, err := c.CombinedOutput()
	return string(out), err
}

// tail is the end of a command's output: at most 40 lines and 4000 bytes.
func tail(s string) string {
	s = strings.TrimRight(s, "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	s = strings.Join(lines, "\n")
	if len(s) > 4000 {
		s = s[len(s)-4000:]
	}
	return strings.ReplaceAll(s, "```", "'''")
}

// planOutcome writes back the end of a plan run: its questions or its plan, never a push.
func (a *Agent) planOutcome(res *claudecode.Result, pl *plan, o *outcome, discarded bool) *outcome {
	d, problem := planDeclaration(res)
	word := a.Config.Plan.ApproveWord
	switch {
	case res.Subtype == "error_max_turns":
		return o.fail(fmt.Sprintf(a.msg.maxTurns, a.Config.Agent.MaxTurns))
	case problem != "" && res.IsError:
		return o.fail(fmt.Sprintf(a.msg.failed, problem))
	case problem != "":
		return o.fail(fmt.Sprintf(a.msg.noDeclaration, problem))
	case d.Status == "questions":
		return o.planQuestions(d, pl.qNext, res.Denials, discarded, word)
	}
	return o.planText(d, res.Denials, discarded, word)
}

// loadInstructions returns the user's additional instructions of kind for a run of p, read again for every run. A
// configuration that was not loaded from a file has none.
func (a *Agent) loadInstructions(kind instructions.Kind, p config.Project) instructions.Loaded {
	if a.Config.Path == "" {
		return instructions.Loaded{}
	}
	return instructions.For(a.Config.Path, nil).Load(kind, p.Ref)
}

// logStarted records, as claude starts, the allow rules the run added and the hashes of the instruction files it got,
// so that the audit log tells which permissions and which saved version of the instructions a run used: used are the
// plan or run instructions, impl the implement instructions. The text of the instructions is never logged, as for
// instructions.saved.
func (a *Agent) logStarted(ref forge.IssueRef, run string, pl *plan, added []string, used, impl instructions.Loaded) {
	fields := map[string]any{
		"issue": ref.String(), "run": run, "kind": pl.kind.String(), "allow": added,
		"instructions": map[string]string{"global": used.Global, "project": used.Project},
		"implement":    map[string]string{"global": impl.Global, "project": impl.Project},
	}
	if pl.kind == fixRun {
		fields["cause"] = pl.fix.cause
	}
	a.Audit.Log("run.started", fields)
}
