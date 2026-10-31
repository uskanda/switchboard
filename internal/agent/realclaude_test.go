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
		SystemPrompt: "Answer with the structured output.", Settings: planSettings(), MaxTurns: 8,
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
