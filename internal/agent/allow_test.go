package agent

import (
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/instructions"
	"github.com/uskanda/switchboard/internal/permissions"
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

// The rules a run gets are listed in full, so that a rule added or lost by accident shows here.
func TestSettingsAreExactly(t *testing.T) {
	s := settings("make  verify", []string{"Skill"})
	wantAllow := []string{
		"Read", "Edit", "Write", "Glob", "Grep", "WebSearch",
		"Bash(git status)", "Bash(git status *)", "Bash(git diff)", "Bash(git diff *)",
		"Bash(git log)", "Bash(git log *)", "Bash(git add)", "Bash(git add *)",
		"Bash(git commit)", "Bash(git commit *)", "Bash(git merge)", "Bash(git merge *)",
		"Bash(git ls-tree)", "Bash(git ls-tree *)", "Bash(git ls-files)", "Bash(git ls-files *)",
		"Bash(git show)", "Bash(git show *)", "Bash(git cat-file)", "Bash(git cat-file *)",
		"Bash(git merge-base)", "Bash(git merge-base *)", "Bash(git rev-parse)", "Bash(git rev-parse *)",
		"Bash(git rev-list)", "Bash(git rev-list *)", "Bash(git blame)", "Bash(git blame *)",
		"Bash(git grep)", "Bash(git grep *)", "Bash(git shortlog)", "Bash(git shortlog *)",
		"Bash(git describe)", "Bash(git describe *)",
		"Bash(git mv)", "Bash(git mv *)", "Bash(git rm)", "Bash(git rm *)",
		"Bash(git restore)", "Bash(git restore *)", "Bash(git stash)", "Bash(git stash *)",
		"Bash(make)", "Bash(make *)", "Skill",
	}
	wantDeny := []string{"Bash(git push)", "Bash(git push *)", "Bash(git rebase)", "Bash(git rebase *)", "Bash(gh)", "Bash(gh *)", "WebFetch"}
	if !slices.Equal(s.Allow, wantAllow) {
		t.Errorf("allow = %q\nwant    %q", s.Allow, wantAllow)
	}
	if !slices.Equal(s.Deny, wantDeny) {
		t.Errorf("deny = %q\nwant   %q", s.Deny, wantDeny)
	}
	if want := []string{"make verify", "git", "git *"}; !slices.Equal(s.Unsandboxed, want) {
		t.Errorf("unsandboxed = %q, want %q", s.Unsandboxed, want)
	}
	if s := settings(""); !slices.Equal(s.Allow, wantAllow[:len(wantAllow)-3]) || !slices.Equal(s.Unsandboxed, []string{"git", "git *"}) {
		t.Errorf("without a verify command: allow = %q, unsandboxed = %q", s.Allow, s.Unsandboxed)
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

// Since ADR-069 the plan run gets the extra allows too, so that it can start the app for its pictures.
func TestThePlanRunAndTheApprovedRunGetTheExtraAllows(t *testing.T) {
	x := newH(t, "", extraAllows)
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.startPlan()
	if allow := x.allowOf(1); !hasExtraAllows(allow) {
		t.Fatalf("the plan run's allow = %v, want Skill and Bash(openspec *)", allow)
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

// A run that writes files may look into the history and move files in the worktree with git, but not fetch, switch
// branches, reset or change git's configuration; a plan run may only look (add-permission-grants).
func TestTheGitARunMayUse(t *testing.T) {
	writing, plan := settings("").Allow, planSettings(false, false, "", "").Allow
	for _, sub := range []string{"ls-tree", "ls-files", "show", "cat-file", "merge-base", "rev-parse", "rev-list", "blame", "grep", "shortlog", "describe"} {
		for _, allow := range [][]string{writing, plan} {
			if !slices.Contains(allow, "Bash(git "+sub+")") || !slices.Contains(allow, "Bash(git "+sub+" *)") {
				t.Errorf("git %s is not allowed: %q", sub, allow)
			}
		}
	}
	for _, sub := range []string{"mv", "rm", "restore", "stash"} {
		if !slices.Contains(writing, "Bash(git "+sub+" *)") {
			t.Errorf("git %s is not allowed to a run that writes files", sub)
		}
		if slices.Contains(plan, "Bash(git "+sub+" *)") {
			t.Errorf("git %s is allowed to a plan run", sub)
		}
	}
	for _, sub := range []string{"fetch", "checkout", "switch", "reset", "branch", "config"} {
		for _, allow := range [][]string{writing, plan} {
			if slices.Contains(allow, "Bash(git "+sub+" *)") || slices.Contains(allow, "Bash(git "+sub+")") {
				t.Errorf("git %s is allowed", sub)
			}
		}
	}
}

// The rules added from the web inbox go to every run that writes files, after allow_tools_extra: every project's,
// then the project's, then the issue's. Another issue's do not, nor does a plan run, and a rule the web inbox would
// refuse is left out and logged once (add-permission-grants).
func TestWritingRunsGetTheGrantedRules(t *testing.T) {
	x := newH(t, "", extraAllows)
	log := &lockedLog{}
	x.agentWith(func(a *Agent) { a.Log = log })
	s := permissions.For(x.cfg.Path, nil)
	for sc, rules := range map[permissions.Scope][]string{
		{}:                             {"Bash(git notes *)"},
		{Project: "gh:o/r"}:            {"Bash(git worktree list)"},
		{Project: "gh:o/r", Issue: 12}: {"Bash(git clean *)"},
		{Project: "gh:o/r", Issue: 99}: {"Bash(git gc *)"},
	} {
		if err := s.Add(sc, rules); err != nil {
			t.Fatal(err)
		}
	}
	p, _ := s.Path(permissions.Scope{})
	if err := os.WriteFile(p, []byte("Bash(git notes *)\nBash(git config *)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.step(3, commit("b.txt", "b")+done("Done again."))
	x.startPlan()
	if allow := x.allowOf(1); slices.Contains(allow, "Bash(git notes *)") || slices.Contains(allow, "Bash(git clean *)") {
		t.Fatalf("the plan run got the granted rules: %q", allow)
	}
	x.replyAndRun("/ok")
	allow := x.allowOf(2)
	i := slices.Index(allow, "Bash(openspec *)")
	if i < 0 || !slices.Equal(allow[i+1:], []string{"Bash(git notes *)", "Bash(git worktree list)", "Bash(git clean *)"}) {
		t.Fatalf("allow = %q, want the global, project and issue rules after allow_tools_extra", allow)
	}
	x.replyAndRun("One more file, please.")
	if allow := x.allowOf(3); slices.Contains(allow, "Bash(git config *)") || slices.Contains(allow, "Bash(git gc *)") {
		t.Fatalf("allow = %q", allow)
	}
	if n := strings.Count(log.String(), "Bash(git config *)"); n != 1 {
		t.Fatalf("the refused rule is logged %d times:\n%s", n, log.String())
	}
}
