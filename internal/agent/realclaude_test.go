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
