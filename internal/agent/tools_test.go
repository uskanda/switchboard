package agent

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
)

// argvOf reads the arguments the nth start of claude got.
func (x *h) argvOf(n int) []string {
	x.t.Helper()
	return strings.Split(strings.TrimSpace(x.file("argv-"+strconv.Itoa(n))), "\n")
}

// flag is the value after name in argv, or "" when argv lacks it.
func flag(argv []string, name string) string {
	if i := slices.Index(argv, name); i >= 0 && i+1 < len(argv) {
		return argv[i+1]
	}
	return ""
}

func TestToolsAddTheToolNamesOfTheExtraAllowsOnce(t *testing.T) {
	got := tools(runTools, []string{"Skill", "Bash(openspec *)"}, []string{" Skill ", "mcp__docs__read", "Bash(make lint)"})
	want := append(slices.Clone(runTools), "Skill", "mcp__docs__read")
	if !slices.Equal(got, want) {
		t.Fatalf("tools = %v, want %v", got, want)
	}
}

func TestEveryRunMayUseWebSearch(t *testing.T) {
	if !slices.Contains(settings("make verify").Allow, "WebSearch") || !slices.Contains(planSettings(false, "", "").Allow, "WebSearch") {
		t.Fatal("WebSearch is not allowed in every run")
	}
	if !slices.Contains(runTools, "WebSearch") || !slices.Contains(planTools, "WebSearch") {
		t.Fatal("WebSearch is not passed to every run")
	}
}

func TestAnImplementingRunGetsItsToolsAndTheExtraOnes(t *testing.T) {
	x := newH(t, "", extraAllows)
	x.step(1, commit("a.txt", "a")+done("Done."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if got, want := flag(x.argvOf(1), "--tools"), "Bash,Read,Edit,Write,Glob,Grep,WebSearch,Skill"; got != want {
		t.Fatalf("--tools = %q, want %q", got, want)
	}
}

func TestThePlanRunGetsReadOnlyToolsAndItsModel(t *testing.T) {
	x := newH(t, "", func(c *config.Config) {
		extraAllows(c)
		c.Agent.Model, c.Agent.Effort = "opus", "high"
		c.Agent.Plan.Model = "sonnet"
	})
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.startPlan()
	argv := x.argvOf(1)
	if got, want := flag(argv, "--tools"), "Bash,Read,Glob,Grep,WebSearch,Skill"; got != want {
		t.Errorf("the plan run's --tools = %q, want %q", got, want)
	}
	if flag(argv, "--model") != "sonnet" || flag(argv, "--effort") != "high" {
		t.Errorf("the plan run's model = %q, effort = %q; want sonnet, high", flag(argv, "--model"), flag(argv, "--effort"))
	}
	x.replyAndRun("/ok")
	if argv := x.argvOf(2); flag(argv, "--model") != "opus" || flag(argv, "--effort") != "high" {
		t.Errorf("the approved run's model = %q, effort = %q; want opus, high", flag(argv, "--model"), flag(argv, "--effort"))
	}
}

func TestAFixRunGetsTheFixModel(t *testing.T) {
	w := newCI(t, "", func(c *config.Config) {
		c.Agent.Model, c.Agent.Effort = "opus", "high"
		c.Agent.Fix.Effort = "medium"
	})
	sha := w.syncHead()
	w.f.SetChecks("o", "r", sha, failedCheck("test", 7))
	w.f.SetLog(7, "--- FAIL: TestFlag\n")
	w.step(2, commit("fix.txt", "fixed")+done("Fixed TestFlag."))
	w.poll()
	w.settle()
	if argv := w.argvOf(2); flag(argv, "--model") != "opus" || flag(argv, "--effort") != "medium" {
		t.Fatalf("the fix run's model = %q, effort = %q; want opus, medium", flag(argv, "--model"), flag(argv, "--effort"))
	}
}

func TestARunWithoutAModelLeavesItToTheCLI(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, commit("a.txt", "a")+done("Done."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if argv := x.argvOf(1); slices.Contains(argv, "--model") || slices.Contains(argv, "--effort") {
		t.Fatalf("argv = %v, want neither --model nor --effort", argv)
	}
}
