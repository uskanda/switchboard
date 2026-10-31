package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
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

// settings are what a run may do without asking. The deny rules are not a boundary (Phase 0, V5); keeping the
// forge token away from claude is.
func settings(verifyCommand string) claudecode.Settings {
	allow := []string{"Read", "Edit", "Write", "Glob", "Grep"}
	for _, c := range []string{"git status", "git diff", "git log", "git add", "git commit"} {
		allow = append(allow, "Bash("+c+")", "Bash("+c+" *)")
	}
	if f := strings.Fields(verifyCommand); len(f) > 0 {
		allow = append(allow, "Bash("+f[0]+")", "Bash("+f[0]+" *)")
	}
	return claudecode.Settings{
		Allow: allow,
		Deny:  []string{"Bash(git push)", "Bash(git push *)", "Bash(gh)", "Bash(gh *)", "WebFetch"},
	}
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
		a.logf("%s: write back run %s: %v; it is finished when the agent starts again", ref, rec.Run, err)
		return ""
	}
	if o.State == "waiting" && ref.Kind() == forge.GitLab {
		a.keepNotes(a.ctx, pl.project, ref)
	}
	rec.State, rec.Ended, rec.Result = "finished", time.Now().UTC(), o.result()
	a.saveRecord(ref, rec)
	_ = a.State.WriteJSON("last.json", rec)
	a.logf("%s: run %s ended: %s", ref, rec.Run, rec.Result)
	return o.State
}

// work prepares the worktree, runs claude and judges the result. It returns nil when the run was canceled.
func (a *Agent) work(ctx context.Context, pl *plan, rec record) *outcome {
	p, is := pl.project, pl.issue
	o := &outcome{Run: rec.Run, Session: rec.Session, Ack: rec.Ack, msg: a.msg}
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
	include := p.IncludeUntrusted && a.private[p.Ref]
	in := prompt.Input{Ref: is.Ref, Title: is.Title, Body: is.Body, IncludeUntrusted: include, Nonce: newID()}
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
	req := claudecode.Request{
		Binary: a.Config.Claude.Binary, Dir: dir, RunDir: a.runDir(rec.Run), SessionID: rec.Session,
		SystemPrompt: prompt.System(lang), Settings: settings(p.VerifyCommand), MaxTurns: a.Config.Agent.MaxTurns,
		Schema: Schema, Auth: a.Config.Claude.Auth, Timeout: a.Config.Agent.Timeout,
		IntGrace: a.IntGrace, TermGrace: a.TermGrace, Env: a.Env,
	}
	head := ""
	switch pl.kind {
	case planRun:
		in.Task = prompt.PlanTask
		req.SystemPrompt, req.Settings, req.Schema = prompt.PlanSystem(lang, word, a.planInstructions(p)), planSettings(), PlanSchema
		if head, err = sp.Head(ctx); err != nil {
			return o.fail(fmt.Sprintf(a.msg.prepare, err))
		}
	case approvedRun:
		req.SystemPrompt = prompt.ApprovedSystem(lang, pl.planID, pl.okID)
	}
	req.Prompt = prompt.Build(in)
	stopProgress := a.followProgress(ctx, is.Ref, rec, filepath.Join(a.runDir(rec.Run), "stream.jsonl"))
	res, err := claudecode.Run(ctx, req)
	stopProgress()
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
	return a.finish(ctx, pl, sp, d, o)
}

// finish handles a run that declared done: verify, check the output, push, open the pull request.
func (a *Agent) finish(ctx context.Context, pl *plan, sp *workspace.Space, d *Declaration, o *outcome) *outcome {
	p, is := pl.project, pl.issue
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
	if err := sp.Push(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return o.fail(fmt.Sprintf(a.msg.pushFailed, err))
	}
	url, err := a.forgeOf(is.Ref).CreateChangeRequest(ctx, forge.ChangeRequest{
		Owner: is.Ref.Owner, Repo: is.Ref.Repo, Title: is.Title, Body: body,
		Head: sp.IssueBranch(), Base: p.BaseBranch, Draft: true,
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		if is.Ref.Kind() == forge.GitLab {
			return o.fail(fmt.Sprintf(a.msg.mrFailed, err))
		}
		return o.fail(fmt.Sprintf(a.msg.prFailed, err))
	}
	if is.Ref.Kind() == forge.GitLab {
		return o.done(d, url, fmt.Sprintf(a.msg.mr, url))
	}
	return o.done(d, url, fmt.Sprintf(a.msg.pr, url))
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

// planInstructions returns the user's additional instructions for the plan runs of p, read again for every run.
// A configuration that was not loaded from a file has none.
func (a *Agent) planInstructions(p config.Project) string {
	if a.Config.Path == "" {
		return ""
	}
	return instructions.For(a.Config.Path, nil).Plan(p.Ref)
}
