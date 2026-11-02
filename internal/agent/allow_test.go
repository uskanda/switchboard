package agent

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/instructions"
)

func TestSettingsAddTheExtraAllowsOnce(t *testing.T) {
	s := settings("make verify", []string{"Skill", "Bash(openspec *)"}, []string{"Bash(openspec *)", "Bash(make lint)"})
	n := len(settings("make verify").Allow)
	if got := s.Allow[n:]; !slices.Equal(got, []string{"Skill", "Bash(openspec *)", "Bash(make lint)"}) {
		t.Errorf("extra allows = %v, want Skill, Bash(openspec *), Bash(make lint) in that order", got)
	}
	if !slices.Equal(s.Deny, settings("").Deny) {
		t.Errorf("deny = %v, want it unchanged", s.Deny)
	}
}

// extraAllows configures Skill for every project and Bash(openspec *) for o/r.
func extraAllows(c *config.Config) {
	c.Agent.AllowToolsExtra = []string{"Skill"}
	c.Projects[0].AllowToolsExtra = []string{"Bash(openspec *)"}
}

// allowOf reads the allow rules the nth start of claude got.
func (x *h) allowOf(n int) []string {
	x.t.Helper()
	var s struct {
		Permissions struct{ Allow, Deny []string } `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(x.file("settings-"+strconv.Itoa(n)+".json")), &s); err != nil {
		x.t.Fatal(err)
	}
	return s.Permissions.Allow
}

func hasExtraAllows(allow []string) bool {
	return slices.Contains(allow, "Skill") && slices.Contains(allow, "Bash(openspec *)")
}

func TestAnImplementingRunGetsTheExtraAllows(t *testing.T) {
	x := newH(t, "", extraAllows)
	x.step(1, commit("a.txt", "a")+done("Done."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if allow := x.allowOf(1); !hasExtraAllows(allow) {
		t.Fatalf("allow = %v, want Skill and Bash(openspec *)", allow)
	}
}

func TestThePlanRunLacksTheExtraAllowsAndTheApprovedRunHasThem(t *testing.T) {
	x := newH(t, "", extraAllows)
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.startPlan()
	if allow := x.allowOf(1); slices.Contains(allow, "Skill") || slices.Contains(allow, "Bash(openspec *)") {
		t.Fatalf("the plan run's allow = %v, want no extra allows", allow)
	}
	x.replyAndRun("/ok")
	if allow := x.allowOf(2); !hasExtraAllows(allow) {
		t.Fatalf("the approved run's allow = %v, want Skill and Bash(openspec *)", allow)
	}
}

func TestAFixRunGetsTheExtraAllows(t *testing.T) {
	w := newCI(t, "", extraAllows)
	sha := w.syncHead()
	w.f.SetChecks("o", "r", sha, failedCheck("test", 7))
	w.f.SetLog(7, "--- FAIL: TestFlag\n")
	w.step(2, commit("fix.txt", "fixed")+done("Fixed TestFlag."))
	w.poll()
	w.settle()
	if allow := w.allowOf(2); !hasExtraAllows(allow) {
		t.Fatalf("the fix run's allow = %v, want Skill and Bash(openspec *)", allow)
	}
}

func TestWritingRunsGetTheRunInstructionsFreshEachTime(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.step(3, commit("b.txt", "b")+done("Done again."))
	s := instructions.For(x.cfg.Path, nil)
	if err := s.Of(instructions.Run).Save(instructions.Global, "Record an OpenSpec change first."); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(instructions.Global, "Ask in short sentences."); err != nil {
		t.Fatal(err)
	}
	p := x.startPlan()
	if sys := x.runFile(p, "system.md"); strings.Contains(sys, "OpenSpec") || !strings.Contains(sys, "Ask in short sentences.") {
		t.Fatalf("the plan run must get the plan instructions only:\n%s", sys)
	}
	p = x.replyAndRun("/ok")
	sys := x.runFile(p, "system.md")
	if !strings.Contains(sys, "## Additional instructions from the user") || !strings.Contains(sys, "Record an OpenSpec change first.") || strings.Contains(sys, "Ask in short sentences.") {
		t.Fatalf("the approved run must get the run instructions only:\n%s", sys)
	}
	if err := s.Of(instructions.Run).Save("gh:o/r", "Tick the tasks."); err != nil {
		t.Fatal(err)
	}
	p = x.replyAndRun("One more file, please.")
	if sys := x.runFile(p, "system.md"); !strings.Contains(sys, "Record an OpenSpec change first.\n\nTick the tasks.") {
		t.Fatalf("the next run did not read the run instructions again:\n%s", sys)
	}
}
