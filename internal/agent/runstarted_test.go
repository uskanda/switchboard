package agent

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/instructions"
)

// started returns the run.started events in the order they were logged.
func (x *h) started() []map[string]any {
	x.audit.mu.Lock()
	defer x.audit.mu.Unlock()
	var out []map[string]any
	for _, e := range x.audit.events {
		if e["event"] == "run.started" {
			out = append(out, e)
		}
	}
	return out
}

// saved returns the sha256 of the last instructions.saved event.
func (x *h) saved() string {
	x.audit.mu.Lock()
	defer x.audit.mu.Unlock()
	for i := len(x.audit.events) - 1; i >= 0; i-- {
		if e := x.audit.events[i]; e["event"] == "instructions.saved" {
			return e["sha256"].(string)
		}
	}
	x.t.Fatal("no instructions.saved event")
	return ""
}

// saveInstructions saves text as the instructions of kind for target, logged to the harness's audit log.
func (x *h) saveInstructions(kind instructions.Kind, target, text string) string {
	x.t.Helper()
	if err := instructions.For(x.cfg.Path, x.audit).Of(kind).Save(target, text); err != nil {
		x.t.Fatal(err)
	}
	return x.saved()
}

func hashes(e map[string]any) map[string]string {
	m, _ := e["instructions"].(map[string]string)
	return m
}

// noInstructionText fails when any audit event holds one of texts.
func (x *h) noInstructionText(texts ...string) {
	x.t.Helper()
	x.audit.mu.Lock()
	defer x.audit.mu.Unlock()
	for _, e := range x.audit.events {
		s := fmt.Sprint(e)
		for _, t := range texts {
			if strings.Contains(s, t) {
				x.t.Fatalf("the audit log holds the instructions: %v", e)
			}
		}
	}
}

func TestADirectRunRecordsItsAllowsAndInstructions(t *testing.T) {
	x := newH(t, "", func(c *config.Config) {
		c.Agent.AllowToolsExtra = []string{"Skill", " Bash(openspec *) ", "Read"}
		c.Projects[0].AllowToolsExtra = []string{"Bash(openspec *)", "Bash(make lint)"}
	})
	global := x.saveInstructions(instructions.Run, instructions.Global, "Record an OpenSpec change first.\n")
	project := x.saveInstructions(instructions.Run, "gh:o/r", "Tick the tasks.\n")
	x.saveInstructions(instructions.Plan, instructions.Global, "Ask in short sentences.\n")
	x.step(1, commit("a.txt", "a")+done("Done."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	s := x.started()
	if len(s) != 1 {
		t.Fatalf("run.started = %v, want one", s)
	}
	e := s[0]
	if e["kind"] != "direct" || e["issue"] != x.ref.String() || e["run"] == "" || e["run"] != x.lastPost(x.ref).Marker.Run {
		t.Fatalf("run.started = %v", e)
	}
	if _, ok := e["cause"]; ok {
		t.Errorf("a direct run has a cause: %v", e)
	}
	if allow, _ := e["allow"].([]string); !slices.Equal(allow, []string{"Skill", "Bash(openspec *)", "Bash(make lint)"}) {
		t.Errorf("allow = %#v, want Skill, Bash(openspec *), Bash(make lint)", e["allow"])
	}
	if h := hashes(e); h["global"] != global || h["project"] != project {
		t.Errorf("instructions = %v, want global %s and project %s", e["instructions"], global, project)
	}
	x.noInstructionText("OpenSpec change", "Tick the tasks", "short sentences")
}

func TestThePlanAndTheApprovedRunRecordTheirKinds(t *testing.T) {
	x := newH(t, "", extraAllows)
	plan := x.saveInstructions(instructions.Plan, instructions.Global, "Ask in short sentences.\n")
	run := x.saveInstructions(instructions.Run, "gh:o/r", "Tick the tasks.\n")
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.startPlan()
	s := x.started()
	if len(s) != 1 || s[0]["kind"] != "plan" {
		t.Fatalf("run.started = %v, want one plan run", s)
	}
	if allow, ok := s[0]["allow"].([]string); !ok || allow == nil || len(allow) != 0 {
		t.Errorf("the plan run's allow = %#v, want an empty list", s[0]["allow"])
	}
	if h := hashes(s[0]); h["global"] != plan || h["project"] != "" {
		t.Errorf("the plan run's instructions = %v, want the plan.md hash %s", s[0]["instructions"], plan)
	}
	x.replyAndRun("/ok")
	s = x.started()
	if len(s) != 2 || s[1]["kind"] != "approved" || s[1]["run"] == s[0]["run"] {
		t.Fatalf("run.started = %v, want an approved run after the plan run", s)
	}
	if allow, _ := s[1]["allow"].([]string); !slices.Equal(allow, []string{"Skill", "Bash(openspec *)"}) {
		t.Errorf("the approved run's allow = %#v", s[1]["allow"])
	}
	if h := hashes(s[1]); h["global"] != "" || h["project"] != run {
		t.Errorf("the approved run's instructions = %v, want the run.md hash %s", s[1]["instructions"], run)
	}
	x.noInstructionText("short sentences", "Tick the tasks")
}

func TestAFixRunAfterARefusedPushRecordsItsCause(t *testing.T) {
	x := newH(t, "", nil)
	x.pushToIssueBranch("person.txt")
	x.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	x.step(2, "git merge -q --no-edit origin/switchboard/issue-12 >/dev/null\n"+done("Merged the person's change."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	s := x.started()
	if len(s) != 2 || s[0]["kind"] != "direct" || s[1]["kind"] != "fix" || s[1]["cause"] != "push" || s[1]["run"] == s[0]["run"] {
		t.Fatalf("run.started = %v, want a direct run, then a push fix run", s)
	}
	x.audit.mu.Lock()
	defer x.audit.mu.Unlock()
	fix, start := -1, -1
	for i, e := range x.audit.events {
		switch {
		case e["event"] == "run.fix" && e["run"] == s[1]["run"]:
			fix = i
		case e["event"] == "run.started" && e["run"] == s[1]["run"]:
			start = i
		}
	}
	if fix < 0 || start < fix {
		t.Fatalf("run.fix at %d, run.started at %d; want run.started after run.fix of the same run", fix, start)
	}
}
