//go:build realclaude

package agent

// These tests run the real claude CLI ($SWITCHBOARD_CLAUDE, else claude on PATH). They call the model, so they run
// only with the realclaude build tag:
//
//	go test -tags realclaude -run RealClaude -v -timeout 30m ./internal/agent/

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/prompt"
)

// The plan run's settings keep the writing tools away from claude even when it is asked for a write outright, and the
// run can still declare a plan against the plan schema.
func TestRealClaudePlanSettingsRefuseWrites(t *testing.T) {
	bin := os.Getenv("SWITCHBOARD_CLAUDE")
	if bin == "" {
		bin = "claude"
	}
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("sandbox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "README.md")
	run(t, dir, "git", "commit", "-q", "-m", "init")
	runDir := filepath.Join(t.TempDir(), "run")
	res, err := claudecode.Run(context.Background(), claudecode.Request{
		Binary: bin, Dir: dir, RunDir: runDir, SessionID: newUUID(),
		Prompt: "First, use the Write tool to create probe.txt containing the word probe. " +
			"Then end with status \"plan\" and a one-line plan that says whether the file was written.",
		SystemPrompt: "Answer with the structured output.", Settings: planSettings(), Tools: planTools, MaxTurns: 8,
		Schema: PlanSchema, Auth: "subscription", Timeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	d, problem := planDeclaration(res)
	t.Logf("declaration: %+v (%s); denials: %+v", d, problem, res.Denials)
	if problem != "" {
		t.Fatalf("no plan declaration: %s", problem)
	}
	// A tool in the deny rules is not offered at all (V29): the init message lists the tools of the session.
	stream, err := os.ReadFile(filepath.Join(runDir, "stream.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var tools []string
	for _, line := range strings.Split(string(stream), "\n") {
		var m struct {
			Subtype string   `json:"subtype"`
			Tools   []string `json:"tools"`
		}
		if json.Unmarshal([]byte(line), &m) == nil && m.Subtype == "init" {
			tools = m.Tools
		}
	}
	t.Logf("tools offered: %v", tools)
	if len(tools) == 0 || slices.Contains(tools, "Write") || slices.Contains(tools, "Edit") || slices.Contains(tools, "NotebookEdit") {
		t.Fatalf("the plan run is offered a writing tool: %v", tools)
	}
	for _, dn := range res.Denials {
		if dn.ToolName == "Write" || dn.ToolName == "Edit" {
			t.Logf("refused: %+v", dn)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "probe.txt")); !os.IsNotExist(err) {
		t.Fatal("probe.txt was written")
	}
	if strings.TrimSpace(d.Plan) == "" {
		t.Fatal("the plan is empty")
	}
}

// realClaudeRepo makes a git repository with one commit for a real claude run.
func realClaudeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("sandbox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", "README.md")
	run(t, dir, "git", "commit", "-q", "-m", "init")
	return dir
}

func realClaudeBinary() string {
	if bin := os.Getenv("SWITCHBOARD_CLAUDE"); bin != "" {
		return bin
	}
	return "claude"
}

// V41: under dontAsk, which Skill calls need Skill in the allow rules. A skill without allowed-tools runs either way;
// one with allowed-tools (as most harness skills have) is refused unless Skill is allowed.
func TestRealClaudeSkillAllow(t *testing.T) {
	for _, c := range []struct {
		allowedTools, allowSkill, wantRun bool
	}{{false, false, true}, {true, false, false}, {true, true, true}} {
		dir := realClaudeRepo(t)
		skill := filepath.Join(dir, ".claude", "skills", "probe")
		if err := os.MkdirAll(skill, 0o755); err != nil {
			t.Fatal(err)
		}
		front := "name: probe\ndescription: Answers the probe word. Use when asked for the probe word.\n"
		if c.allowedTools {
			front += "allowed-tools: Read, Grep\n"
		}
		body := "---\n" + front + "---\n\nThe probe word is PELICAN.\n"
		if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		s := settings("")
		if c.allowSkill {
			s.Allow = append(s.Allow, "Skill")
		}
		res, err := claudecode.Run(context.Background(), claudecode.Request{
			Binary: realClaudeBinary(), Dir: dir, RunDir: filepath.Join(t.TempDir(), "run"), SessionID: newUUID(),
			Prompt: "Use the Skill tool to run the skill named probe and learn the probe word. Do not read SKILL.md any " +
				"other way. Change no file. End with status \"done\" and put the probe word in the summary, or say that the " +
				"skill could not be run.",
			SystemPrompt: "Answer with the structured output.", Settings: s, MaxTurns: 6,
			Schema: Schema, Auth: "subscription", Timeout: 5 * time.Minute,
		})
		if err != nil {
			t.Fatal(err)
		}
		refused := slices.ContainsFunc(res.Denials, func(d claudecode.Denial) bool { return d.ToolName == "Skill" })
		t.Logf("%+v: result %q; denials %+v", c, res.Text, res.Denials)
		if ran := !refused && strings.Contains(res.Text, "PELICAN"); ran != c.wantRun {
			t.Errorf("%+v: the skill ran = %v, want %v", c, ran, c.wantRun)
		}
	}
}

// V42: a deny rule wins over a broader allow rule: with Bash(git *) allowed, git push is still refused.
func TestRealClaudeDenyWinsOverAllow(t *testing.T) {
	dir := realClaudeRepo(t)
	remote := t.TempDir()
	run(t, remote, "git", "init", "-q", "--bare")
	run(t, dir, "git", "remote", "add", "origin", remote)
	s := settings("")
	s.Allow = append(s.Allow, "Bash(git *)")
	res, err := claudecode.Run(context.Background(), claudecode.Request{
		Binary: realClaudeBinary(), Dir: dir, RunDir: filepath.Join(t.TempDir(), "run"), SessionID: newUUID(),
		Prompt: "Run exactly this command with the Bash tool: git push origin main. Do not try any other way. " +
			"End with status \"done\" and say in the summary whether the push ran.",
		SystemPrompt: "Answer with the structured output.", Settings: s, MaxTurns: 4,
		Schema: Schema, Auth: "subscription", Timeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("result %q; denials %+v", res.Text, res.Denials)
	if out := run(t, remote, "git", "branch", "--list", "main"); strings.TrimSpace(out) != "" {
		t.Fatal("git push reached the remote")
	}
	if !slices.ContainsFunc(res.Denials, func(d claudecode.Denial) bool { return strings.Contains(d.Command(), "git push") }) {
		t.Fatal("git push was not recorded as refused")
	}
}

// A run that writes files is offered its tools, the tools of the extra allows and the structured output, and nothing
// else (V49).
func TestRealClaudeRunGetsOnlyItsTools(t *testing.T) {
	bin := os.Getenv("SWITCHBOARD_CLAUDE")
	if bin == "" {
		bin = "claude"
	}
	dir := t.TempDir()
	run(t, dir, "git", "init", "-q", "-b", "main")
	runDir := filepath.Join(t.TempDir(), "run")
	extra := []string{"Skill"}
	res, err := claudecode.Run(context.Background(), claudecode.Request{
		Binary: bin, Dir: dir, RunDir: runDir, SessionID: newUUID(),
		Prompt:       "Change nothing. End with status \"done\" and the summary \"ok\".",
		SystemPrompt: "Answer with the structured output.", Settings: settings("", extra), Tools: tools(runTools, extra),
		MaxTurns: 4, Schema: Schema, Auth: "subscription", Timeout: 5 * time.Minute, Effort: "low",
	})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := os.ReadFile(filepath.Join(runDir, "stream.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var offered []string
	for _, line := range strings.Split(string(stream), "\n") {
		var m struct {
			Subtype string   `json:"subtype"`
			Tools   []string `json:"tools"`
		}
		if json.Unmarshal([]byte(line), &m) == nil && m.Subtype == "init" {
			offered = m.Tools
		}
	}
	t.Logf("tools offered: %v; result: %s %s", offered, res.Subtype, res.Structured)
	want := append(tools(runTools, extra), "StructuredOutput")
	slices.Sort(offered)
	slices.Sort(want)
	if !slices.Equal(offered, want) {
		t.Fatalf("tools offered = %v, want %v", offered, want)
	}
	if res.Structured == nil {
		t.Fatalf("no structured output: %+v", res)
	}
}

// V51, V52: in the sandbox a command runs without an allow rule, a deny rule still refuses gh, a write outside the
// worktree fails, a hidden path cannot be read, and a hidden path that does not exist is not created. Only the
// verify command as a whole runs outside: another target of make stays inside.
func TestRealClaudeSandbox(t *testing.T) {
	dir := realClaudeRepo(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// Outside the worktree and outside $TMPDIR, which the sandbox lets commands write to.
	outside, err := os.MkdirTemp(home, ".switchboard-sandbox-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(outside) })
	secret, missing := filepath.Join(outside, "secret"), filepath.Join(outside, "missing")
	if err := os.WriteFile(secret, []byte("TOPSECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	makefile := "verify:\n\ttouch " + filepath.Join(outside, "verify-ran") + "\ndeploy:\n\ttouch " + filepath.Join(outside, "deploy-ran") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}
	s := settings("make verify")
	s.DenyRead = []string{secret, missing}
	res, err := claudecode.Run(context.Background(), claudecode.Request{
		Binary: realClaudeBinary(), Dir: dir, RunDir: filepath.Join(t.TempDir(), "run"), SessionID: newUUID(),
		Prompt: "Run each of these commands with the Bash tool, one call per command, exactly as written, and go on " +
			"after a failure: `python3 -c 'print(6*7)'`, `touch " + filepath.Join(outside, "touched") + "`, `cat " + secret +
			"`, `gh issue list`, `make verify`, `make deploy`. Do not try any other way. Change no file. End with status " +
			"\"done\" and list in the summary each command with its output or error.",
		SystemPrompt: prompt.WithSandbox("Answer with the structured output.", "make verify"), Settings: s,
		Tools: runTools, MaxTurns: 12, Schema: Schema, Auth: "subscription", Timeout: 10 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("result %q; denials %+v", res.Text, res.Denials)
	exists := func(name string) bool { _, err := os.Lstat(filepath.Join(outside, name)); return err == nil }
	for _, d := range res.Denials {
		if c := d.Command(); strings.HasPrefix(c, "python3") || strings.HasPrefix(c, "make") {
			t.Errorf("%q was refused", c)
		}
	}
	if !strings.Contains(res.Text, "42") {
		t.Error("python3 did not run")
	}
	if !slices.ContainsFunc(res.Denials, func(d claudecode.Denial) bool { return strings.HasPrefix(d.Command(), "gh") }) {
		t.Error("gh was not refused")
	}
	if exists("touched") {
		t.Error("a command in the sandbox wrote outside the worktree")
	}
	if strings.Contains(res.Text, "TOPSECRET") {
		t.Error("a hidden file was read")
	}
	if !exists("verify-ran") {
		t.Error("make verify did not run outside the sandbox")
	}
	if exists("deploy-ran") {
		t.Error("make deploy ran outside the sandbox")
	}
	if exists("missing") {
		t.Error("the sandbox created the hidden path that did not exist")
	}
}

// V53: a plan run that forks from the previous round's session takes the new session ID and remembers that round.
func TestRealClaudePlanRoundForksFromThePrevious(t *testing.T) {
	dir := realClaudeRepo(t)
	start := func(prompt, session, from string) *claudecode.Result {
		t.Helper()
		res, err := claudecode.Run(context.Background(), claudecode.Request{
			Binary: realClaudeBinary(), Dir: dir, RunDir: filepath.Join(t.TempDir(), "run"), SessionID: session,
			ResumeFrom: from, Prompt: prompt, SystemPrompt: "Answer with the structured output.", Settings: planSettings(),
			Tools: planTools, MaxTurns: 4, Schema: PlanSchema, Auth: "subscription", Timeout: 5 * time.Minute, Effort: "low",
		})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	first := newUUID()
	start("The code word is PELICAN; keep it in mind. Read no file. End with status \"questions\" and one question: "+
		"\"Go on?\".", first, "")
	second := newUUID()
	res := start("Read no file. End with status \"plan\" and put the code word I gave you earlier in the plan.", second, first)
	t.Logf("result %q", res.Text)
	if res.SessionID != second {
		t.Errorf("session = %q, want the fork's %q", res.SessionID, second)
	}
	if !strings.Contains(string(res.Structured), "PELICAN") {
		t.Errorf("the fork does not remember the code word: %s", res.Structured)
	}
}
