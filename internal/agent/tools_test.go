package agent

import (
	"encoding/json"
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
	if !slices.Contains(settings("make verify").Allow, "WebSearch") || !slices.Contains(planSettings(false, false, "", "").Allow, "WebSearch") {
		t.Fatal("WebSearch is not allowed in every run")
	}
	if !slices.Contains(runTools, "WebSearch") || !slices.Contains(planTools, "WebSearch") {
		t.Fatal("WebSearch is not passed to every run")
	}
}

// A plan run with WebFetch on allows it on any domain and gets the tool, with prototypes and pictures too; off, the
// settings and the tools are as they were before WebFetch could be allowed (ADR-074).
func TestPlanRunWebFetch(t *testing.T) {
	cases := []struct {
		name      string
		prototype bool
		shot, img string
		images    bool
	}{
		{"read only", false, "", "", false},
		{"prototype", true, "/bin/switchboard shot", "", false},
		{"pictures", false, "/bin/switchboard shot", "/w/.switchboard/plan-images", true},
	}
	for _, c := range cases {
		on := planSettings(c.prototype, true, c.shot, c.img)
		if !slices.Contains(on.Allow, "WebFetch") || slices.Contains(on.Deny, "WebFetch") {
			t.Errorf("%s, on: allow %v, deny %v; want WebFetch allowed", c.name, on.Allow, on.Deny)
		}
		if !slices.Contains(planToolsFor(c.prototype, c.images, true), "WebFetch") {
			t.Errorf("%s, on: WebFetch is not among the tools", c.name)
		}
		off := planSettings(c.prototype, false, c.shot, c.img)
		if slices.Contains(off.Allow, "WebFetch") || !slices.Contains(off.Deny, "WebFetch") {
			t.Errorf("%s, off: allow %v, deny %v; want WebFetch denied", c.name, off.Allow, off.Deny)
		}
		if slices.Contains(planToolsFor(c.prototype, c.images, false), "WebFetch") {
			t.Errorf("%s, off: WebFetch is among the tools", c.name)
		}
		on.Allow = slices.DeleteFunc(on.Allow, func(r string) bool { return r == "WebFetch" })
		on.Deny = append(slices.Clone(on.Deny), "WebFetch")
		slices.Sort(on.Deny)
		slices.Sort(off.Deny)
		if !slices.Equal(on.Allow, off.Allow) || !slices.Equal(on.Deny, off.Deny) {
			t.Errorf("%s: on and off differ beyond WebFetch: %+v, %+v", c.name, on, off)
		}
	}
}

// The runs that write files deny WebFetch whatever the project's plan_web_fetch.
func TestRunsThatWriteDenyWebFetch(t *testing.T) {
	if s := settings("make verify"); !slices.Contains(s.Deny, "WebFetch") || slices.Contains(s.Allow, "WebFetch") {
		t.Errorf("settings = %+v, want WebFetch denied", s)
	}
	if slices.Contains(runTools, "WebFetch") {
		t.Error("WebFetch is among the tools of the runs that write files")
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
	if got, want := flag(argv, "--tools"), "Bash,Read,Glob,Grep,WebSearch,WebFetch,Skill"; got != want {
		t.Errorf("the plan run's --tools = %q, want %q", got, want)
	}
	if flag(argv, "--model") != "sonnet" || flag(argv, "--effort") != "high" {
		t.Errorf("the plan run's model = %q, effort = %q; want sonnet, high", flag(argv, "--model"), flag(argv, "--effort"))
	}
	x.replyAndRun("/ok")
	if argv := x.argvOf(2); flag(argv, "--model") != "opus" || flag(argv, "--effort") != "high" {
		t.Errorf("the approved run's model = %q, effort = %q; want opus, high", flag(argv, "--model"), flag(argv, "--effort"))
	}
	if strings.Contains(flag(x.argvOf(2), "--tools"), "WebFetch") || !strings.Contains(x.file("settings-2.json"), `"WebFetch"`) {
		t.Errorf("the approved run may use WebFetch: --tools %q", flag(x.argvOf(2), "--tools"))
	}
	if !strings.Contains(x.file("system-1.md"), "## Web pages") || strings.Contains(x.file("system-2.md"), "## Web pages") {
		t.Error("only the plan run should be told about web pages")
	}
}

// A project with plan_web_fetch: false keeps WebFetch away from its plan runs.
func TestThePlanRunOfAProjectWithoutWebFetch(t *testing.T) {
	off := false
	x := newH(t, "", func(c *config.Config) { c.Projects[0].PlanWebFetch = &off })
	x.step(1, planDecl("The plan."))
	x.startPlan()
	if got := flag(x.argvOf(1), "--tools"); strings.Contains(got, "WebFetch") {
		t.Errorf("the plan run's --tools = %q, want no WebFetch", got)
	}
	var s struct {
		Permissions struct{ Allow, Deny []string } `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(x.file("settings-1.json")), &s); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(s.Permissions.Allow, "WebFetch") || !slices.Contains(s.Permissions.Deny, "WebFetch") {
		t.Errorf("permissions = %+v, want WebFetch denied", s.Permissions)
	}
	if strings.Contains(x.file("system-1.md"), "## Web pages") {
		t.Error("a plan run without WebFetch is told about web pages")
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
