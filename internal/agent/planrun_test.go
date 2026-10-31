package agent

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/instructions"
)

func questions(qs ...[3]string) string {
	var items []map[string]any
	for _, q := range qs {
		items = append(items, map[string]any{"title": q[0], "body": q[1], "recommendation": q[2]})
	}
	return result(map[string]any{"status": "questions", "summary": "Read the config package.", "questions": items})
}

func planDecl(text string) string {
	return result(map[string]any{"status": "plan", "plan": text})
}

// startPlan puts agent:plan on the issue, runs the first step and returns the agent's post.
func (x *h) startPlan() post {
	x.t.Helper()
	x.f.AddLabel(x.ref, "agent:plan", x.master)
	x.poll()
	x.settle()
	return x.lastPost(x.ref)
}

func (x *h) replyAndRun(body string) post {
	x.t.Helper()
	x.f.AddComment(x.ref, body, x.master)
	x.poll()
	x.settle()
	return x.lastPost(x.ref)
}

// runFile reads a file of the run that wrote p.
func (x *h) runFile(p post, name string) string {
	x.t.Helper()
	b, err := os.ReadFile(filepath.Join(x.dir.Path, "runs", p.Marker.Run, name))
	if err != nil {
		x.t.Fatal(err)
	}
	return string(b)
}

func TestPlanRunPostsNumberedQuestions(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, questions([3]string{"Default", "Which default?", "A, because it is safe."}, [3]string{"Name", "Which name?", "B."}))
	p := x.startPlan()
	if x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != "plan-questions" || p.Marker.Q != 2 {
		t.Fatalf("labels=%q marker=%+v", x.labels(x.ref), p.Marker)
	}
	for _, want := range []string{"**Q1** - **Default**", "Which default?", "A, because it is safe.", "**Q2** - **Name**", "`/ok`", "Read the config package."} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("questions comment lacks %q:\n%s", want, p.Body)
		}
	}
	if last := lastLine(p.Body); !strings.Contains(last, "`/ok`") {
		t.Errorf("the last line is not the hint: %q", last)
	}
	sys := x.runFile(p, "system.md")
	if !strings.Contains(sys, "plan stage") || strings.Contains(sys, `status "done"`) {
		t.Fatalf("the run did not get the plan-stage rules:\n%s", sys)
	}
	if !strings.Contains(x.runFile(p, "prompt.md"), "Plan the work") {
		t.Fatal("the prompt does not ask for a plan")
	}
}

func TestQuestionNumbersContinueAfterAnOK(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, questions([3]string{"A", "a?", "a"}, [3]string{"B", "b?", "b"}, [3]string{"C", "c?", "c"}))
	x.step(2, questions([3]string{"D", "d?", "d"}, [3]string{"E", "e?", "e"}))
	x.startPlan()
	p := x.replyAndRun("/ok")
	if !strings.Contains(p.Body, "**Q4** - **D**") || !strings.Contains(p.Body, "**Q5** - **E**") || p.Marker.Q != 5 {
		t.Fatalf("second round = %+v\n%s", p.Marker, p.Body)
	}
	if !strings.Contains(x.file("prompt-2.md"), "/ok") {
		t.Fatal("the /ok reply is not in the second prompt")
	}
}

func TestPlanRunPostsThePlanAndApprovalImplementsIt(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("1. Add greeting.txt.\n2. Test it."))
	x.step(2, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	p := x.startPlan()
	if p.Marker.Reason != "plan" || !strings.Contains(p.Body, "Add greeting.txt.") || x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("plan post = %+v labels=%q\n%s", p.Marker, x.labels(x.ref), p.Body)
	}
	if last := lastLine(p.Body); !strings.Contains(last, "`/ok`") {
		t.Errorf("the last line is not the hint: %q", last)
	}
	planID := x.lastComment(t, "plan").ID
	ok := x.f.AddComment(x.ref, "/ok q2 is skipped", x.master)
	x.poll()
	x.settle()
	d := x.lastPost(x.ref)
	if d.Marker.State != "done" || x.labels(x.ref) != "agent:done" {
		t.Fatalf("after the approval: %+v labels=%q\n%s", d.Marker, x.labels(x.ref), d.Body)
	}
	sys := x.runFile(d, "system.md")
	for _, want := range []string{"comment " + strconv.FormatInt(planID, 10), "comment " + strconv.FormatInt(ok.ID, 10), `status "done"`} {
		if !strings.Contains(sys, want) {
			t.Errorf("the approved run's system prompt lacks %q", want)
		}
	}
}

func TestAReplyToThePlanPlansAgain(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("Plan one."))
	x.step(2, planDecl("Plan two."))
	x.startPlan()
	p := x.replyAndRun("Rename the API.")
	if p.Marker.Reason != "plan" || !strings.Contains(p.Body, "Plan two.") {
		t.Fatalf("second post = %+v\n%s", p.Marker, p.Body)
	}
	if strings.Contains(x.runFile(p, "system.md"), `status "done"`) {
		t.Fatal("a reply to the plan started an implementing run")
	}
}

func TestAnApprovalEditedByAnUntrustedUserIsNoApproval(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("Plan one."))
	x.step(2, planDecl("Plan two."))
	x.startPlan()
	ok := x.f.AddComment(x.ref, "Looks wrong.", x.master)
	x.f.EditCommentAs(x.ref, ok.ID, "/ok", x.mallory)
	x.poll()
	x.settle()
	// The edited comment is no instruction at all, so it neither approves nor counts as a reply.
	if x.starts() != 1 || x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q; an approval edited by mallory started a run", x.starts(), x.labels(x.ref))
	}
}

func TestPlanRunThatDeclaresDoneIsAnError(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, done("Implemented it anyway."))
	p := x.startPlan()
	if p.Marker.Reason != "error" {
		t.Fatalf("marker = %+v\n%s", p.Marker, p.Body)
	}
}

func TestPlanRunWithNoQuestionsIsAnError(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, result(map[string]any{"status": "questions", "questions": []any{}}))
	if p := x.startPlan(); p.Marker.Reason != "error" {
		t.Fatalf("marker = %+v\n%s", p.Marker, p.Body)
	}
}

func TestPlanRunChangesAreThrownAway(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, "echo stray > stray.txt\n"+commit("sneaky.txt", "x")+questions([3]string{"A", "a?", "a"}))
	p := x.startPlan()
	if p.Marker.Reason != "plan-questions" || !strings.Contains(p.Body, "discarded") {
		t.Fatalf("post = %+v\n%s", p.Marker, p.Body)
	}
	wt := filepath.Join(x.dir.Path, "worktrees", "o", "r", "issue-12")
	for _, f := range []string{"stray.txt", "sneaky.txt"} {
		if _, err := os.Stat(filepath.Join(wt, f)); !os.IsNotExist(err) {
			t.Errorf("%s is still in the worktree", f)
		}
	}
	if got := run(t, wt, "git", "log", "--oneline", "origin/main..HEAD"); got != "" {
		t.Fatalf("the plan run left commits: %s", got)
	}
}

func TestPlanRunKeepsItsReasonWhenRefused(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, result(map[string]any{"status": "plan", "plan": "The plan."},
		map[string]any{"tool_name": "Bash", "tool_use_id": "t1", "tool_input": map[string]any{"command": "go test ./..."}}))
	p := x.startPlan()
	if p.Marker.Reason != "plan" || !strings.Contains(p.Body, "<details>") || !strings.Contains(p.Body, "go test ./...") {
		t.Fatalf("post = %+v\n%s", p.Marker, p.Body)
	}
}

func TestPlanRunPushesNothing(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("The plan."))
	x.startPlan()
	if x.originHas("switchboard/issue-12") || len(x.f.PRs) != 0 {
		t.Fatal("the plan run pushed or opened a pull request")
	}
}

func TestPlanRunSettingsReadOnly(t *testing.T) {
	s := planSettings()
	for _, denied := range []string{"Edit", "Write", "NotebookEdit", "Bash(git commit)", "Bash(git add *)", "Bash(git push)", "Bash(make *)"} {
		if !slices.Contains(s.Deny, denied) {
			t.Errorf("%s is not denied", denied)
		}
	}
	for _, allowed := range []string{"Read", "Grep", "Glob", "Bash(git diff *)"} {
		if !slices.Contains(s.Allow, allowed) {
			t.Errorf("%s is not allowed", allowed)
		}
	}
	for _, a := range s.Allow {
		if strings.Contains(a, "Write") || strings.Contains(a, "Edit") || strings.Contains(a, "make") {
			t.Errorf("the plan run may %s", a)
		}
	}
}

func TestPlanCommentsInJapanese(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Comments.Language = "ja" })
	x.step(1, questions([3]string{"既定", "どちら？", "A"}))
	p := x.startPlan()
	if !strings.Contains(p.Body, "**Q1** - **既定**") || !strings.Contains(p.Body, "推奨どおり") {
		t.Fatalf("post:\n%s", p.Body)
	}
}

// lastComment returns the agent's last post whose marker has the reason.
func (x *h) lastComment(t *testing.T, reason string) forge.Comment {
	t.Helper()
	cs := x.f.CommentsOf(x.ref)
	for i := len(cs) - 1; i >= 0; i-- {
		if strings.Contains(cs[i].Body, "reason="+reason+" ") {
			return cs[i]
		}
	}
	t.Fatalf("no post with reason %s", reason)
	return forge.Comment{}
}

// lastLine is the last non-blank line of a post above its marker.
func lastLine(body string) string {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	for i := len(lines) - 2; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return ""
}

func TestPlanRunGetsTheUsersInstructionsFreshEachTime(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, questions([3]string{"A", "a?", "a"}))
	x.step(2, planDecl("The plan."))
	x.step(3, commit("a.txt", "a")+done("Done."))
	s := instructions.For(x.cfg.Path, nil)
	if err := s.Save(instructions.Global, "Ask in short sentences."); err != nil {
		t.Fatal(err)
	}
	p := x.startPlan()
	sys := x.runFile(p, "system.md")
	if !strings.Contains(sys, "## Additional instructions from the user") || !strings.Contains(sys, "Ask in short sentences.") {
		t.Fatalf("first plan run:\n%s", sys)
	}
	if err := s.Save("gh:o/r", "Mind the API names."); err != nil {
		t.Fatal(err)
	}
	p = x.replyAndRun("/ok")
	sys = x.runFile(p, "system.md")
	if !strings.Contains(sys, "Ask in short sentences.\n\nMind the API names.") {
		t.Fatalf("second plan run did not read the instructions again:\n%s", sys)
	}
	p = x.replyAndRun("/ok")
	if sys := x.runFile(p, "system.md"); strings.Contains(sys, "Mind the API names.") || strings.Contains(sys, "Additional instructions") {
		t.Fatalf("the approved run got the plan instructions:\n%s", sys)
	}
}

func TestPlanRunWithoutInstructionsHasNoSection(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("The plan."))
	p := x.startPlan()
	if strings.Contains(x.runFile(p, "system.md"), "Additional instructions") {
		t.Fatal("a plan run without instructions got the section")
	}
}
