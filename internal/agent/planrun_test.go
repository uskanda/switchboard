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
	"github.com/uskanda/switchboard/internal/postfmt"
)

// questions declares questions given as title, body and reason, each with the options Keep it and Change it and the
// first of them recommended.
func questions(qs ...[3]string) string {
	var items []map[string]any
	for _, q := range qs {
		items = append(items, declared(q[0], q[1], q[2]))
	}
	return result(map[string]any{"status": "questions", "title": "Split the config loading", "summary": "Read the config package.",
		"questions": items})
}

// declared is one declared question with the options Keep it and Change it, the first recommended.
func declared(title, body, reason string) map[string]any {
	return map[string]any{"title": title, "body": body, "options": []string{"Keep it", "Change it"}, "recommended": "A", "reason": reason, "weight": "decision"}
}

func planDecl(text string) string {
	return result(map[string]any{"status": "plan", "title": "Split the config loading", "plan": text})
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
	x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{}}))
	if p := x.startPlan(); p.Marker.Reason != "error" {
		t.Fatalf("marker = %+v\n%s", p.Marker, p.Body)
	}
}

// paragraphAfterHeading is the first paragraph of a post after its ### heading.
func paragraphAfterHeading(body string) string {
	paras := strings.Split(strings.TrimSpace(body), "\n\n")
	if len(paras) < 2 || !strings.HasPrefix(paras[0], "### ") {
		return ""
	}
	return paras[1]
}

func TestPlanCommentsStartWithTheTitle(t *testing.T) {
	for _, tc := range []struct{ name, lang, step, reason, want string }{
		{"questions", "", questions([3]string{"A", "a?", "a"}), "plan-questions", "**Plan:** Split the config loading"},
		{"plan", "", planDecl("The plan."), "plan", "**Plan:** Split the config loading"},
		{"questions ja", "ja", result(map[string]any{"status": "questions", "title": "設定の\n読み込みを  分ける",
			"questions": []any{declared("既定", "どちら？", "A")}}),
			"plan-questions", "**計画：** 設定の 読み込みを 分ける"},
		{"plan ja", "ja", result(map[string]any{"status": "plan", "title": "設定の読み込みを分ける", "plan": "計画。"}),
			"plan", "**計画：** 設定の読み込みを分ける"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newH(t, "", func(c *config.Config) { c.Comments.Language = tc.lang })
			x.step(1, tc.step)
			p := x.startPlan()
			if p.Marker.Reason != tc.reason || paragraphAfterHeading(p.Body) != tc.want {
				t.Fatalf("marker=%+v; the paragraph after the heading is not %q:\n%s", p.Marker, tc.want, p.Body)
			}
			if got := postfmt.Question(p.Body); got != tc.want {
				t.Fatalf("Question = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPlanRunWithoutATitleIsAnError(t *testing.T) {
	for _, d := range []map[string]any{
		{"status": "plan", "plan": "The plan."},
		{"status": "plan", "title": " \n ", "plan": "The plan."},
		{"status": "questions", "questions": []any{declared("A", "a?", "a")}},
	} {
		x := newH(t, "", nil)
		x.step(1, result(d))
		if p := x.startPlan(); p.Marker.Reason != "error" {
			t.Fatalf("%v: marker = %+v\n%s", d, p.Marker, p.Body)
		}
	}
}

func TestQuestionOfAnOldPlanCommentWithoutATitle(t *testing.T) {
	old := "### The plan is ready for approval\n\n# Plan\n\n- Add greeting.txt.\n\nReply `/ok` to approve."
	if got := postfmt.Question(old); got != "- Add greeting.txt." {
		t.Fatalf("Question = %q", got)
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
	x.step(1, result(map[string]any{"status": "plan", "title": "T", "plan": "The plan."},
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

// An extra allow of Write does not let a plan run write unless the project allows prototypes; then Edit and Write are
// allowed, and the rest stays denied.
func TestPlanRunSettingsPrototype(t *testing.T) {
	s := planSettings(false, "", "", []string{"Write"})
	if !slices.Contains(s.Deny, "Write") || !slices.Contains(s.Deny, "Edit") {
		t.Errorf("deny = %v, want Edit and Write while prototypes are off", s.Deny)
	}
	p := planSettings(true, "/bin/switchboard shot --browser /bin/chromium", "")
	if slices.Contains(p.Deny, "Write") || slices.Contains(p.Deny, "Edit") || !slices.Contains(p.Allow, "Write") || !slices.Contains(p.Allow, "Edit") {
		t.Errorf("prototype settings = %+v, want Edit and Write allowed", p)
	}
	for _, denied := range []string{"NotebookEdit", "Bash(git commit)", "Bash(git push)", "Bash(make *)"} {
		if !slices.Contains(p.Deny, denied) {
			t.Errorf("%s is not denied with prototypes on", denied)
		}
	}
	if !slices.Contains(p.Allow, "Bash(/bin/switchboard shot --browser /bin/chromium *)") {
		t.Errorf("allow = %v, want the shot command", p.Allow)
	}
	if got := strings.Join(planToolsFor(true, false), ","); got != "Bash,Read,Glob,Grep,WebSearch,Edit,Write" {
		t.Errorf("tools = %s", got)
	}
}

func TestPlanRunSettingsReadOnly(t *testing.T) {
	s := planSettings(false, "", "")
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

func TestPlanQuestionsWriteTheOptionsAndTheRecommendation(t *testing.T) {
	for _, tc := range []struct{ name, lang, rec string }{
		{"English", "", "➡️ **Recommended: B** It loads faster."},
		{"Japanese", "ja", "➡️ **推奨：B** It loads faster."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newH(t, "", func(c *config.Config) { c.Comments.Language = tc.lang })
			x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{
				map[string]any{"title": "Loading", "body": "How should the config load?", "options": []string{"Keep it", "Split it"},
					"recommended": "B", "reason": "It loads faster.", "weight": "decision"},
			}}))
			p := x.startPlan()
			want := "How should the config load?\n\nA: Keep it\nB: Split it\n\n" + tc.rec
			if p.Marker.Reason != "plan-questions" || !strings.Contains(p.Body, want) {
				t.Fatalf("marker=%+v; the question is not written as\n%s\n---\n%s", p.Marker, want, p.Body)
			}
		})
	}
}

func TestPlanRunWithABadOptionOrRecommendationIsAnError(t *testing.T) {
	for _, tc := range []struct {
		name string
		q    map[string]any
	}{
		{"recommended outside the options", map[string]any{"title": "T", "body": "b?", "options": []string{"x", "y", "z"}, "recommended": "D", "reason": "r", "weight": "decision"}},
		{"one option", map[string]any{"title": "T", "body": "b?", "options": []string{"x"}, "recommended": "A", "reason": "r", "weight": "decision"}},
		{"seven options", map[string]any{"title": "T", "body": "b?", "options": []string{"1", "2", "3", "4", "5", "6", "7"}, "recommended": "A", "reason": "r", "weight": "decision"}},
		{"a blank option", map[string]any{"title": "T", "body": "b?", "options": []string{"x", " "}, "recommended": "A", "reason": "r", "weight": "decision"}},
		{"no reason", map[string]any{"title": "T", "body": "b?", "options": []string{"x", "y"}, "recommended": "A", "reason": " ", "weight": "decision"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newH(t, "", nil)
			x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{tc.q}}))
			if p := x.startPlan(); p.Marker.Reason != "error" {
				t.Fatalf("marker = %+v\n%s", p.Marker, p.Body)
			}
		})
	}
}

func TestPlanRunWithABadWeightIsAnError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		weight any
	}{
		{"no weight", nil},
		{"another weight", "maybe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newH(t, "", nil)
			q := declared("T", "b?", "r")
			delete(q, "weight")
			if tc.weight != nil {
				q["weight"] = tc.weight
			}
			x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{q}}))
			if p := x.startPlan(); p.Marker.Reason != "error" {
				t.Fatalf("marker = %+v\n%s", p.Marker, p.Body)
			}
		})
	}
}

// weighed is a declared question titled title with the weight w.
func weighed(title, w string) map[string]any {
	q := declared(title, "Which one?", "It is simpler.")
	q["weight"] = w
	return q
}

// inOrder reports whether every one of parts appears in s, each after the one before.
func inOrder(s string, parts ...string) bool {
	at := 0
	for _, p := range parts {
		i := strings.Index(s[at:], p)
		if i < 0 {
			return false
		}
		at += i + len(p)
	}
	return true
}

func TestPlanQuestionsPutTheChecksLastUnderAHeading(t *testing.T) {
	for _, tc := range []struct{ name, lang, heading string }{
		{"English", "", "#### Going ahead as recommended"},
		{"Japanese", "ja", "#### 推奨どおりで進めるもの"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newH(t, "", func(c *config.Config) { c.Comments.Language = tc.lang })
			x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{
				weighed("P", "check"), weighed("R", "decision"), weighed("S", "check"),
			}}))
			p := x.startPlan()
			if !inOrder(p.Body, "**Q1** - **R**", "\n\n---\n\n"+tc.heading+"\n\n", "**Q2** - **P**", "**Q3** - **S**") {
				t.Fatalf("the decision is not first and the checks are not under the heading:\n%s", p.Body)
			}
			if p.Marker.Q != 3 {
				t.Fatalf("marker q = %d, want 3", p.Marker.Q)
			}
		})
	}
}

func TestPlanQuestionsWithoutChecksHaveNoHeading(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{weighed("R", "decision"), weighed("S", "decision")}}))
	p := x.startPlan()
	if strings.Contains(p.Body, "Going ahead as recommended") || !inOrder(p.Body, "**Q1** - **R**", "**Q2** - **S**") {
		t.Fatalf("a round without checks:\n%s", p.Body)
	}
}

func TestPlanQuestionsOfChecksAloneArePostedUnderTheHeading(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{weighed("P", "check"), weighed("S", "check")}}))
	p := x.startPlan()
	if p.Marker.Reason != "plan-questions" || !inOrder(p.Body, "#### Going ahead as recommended\n\n", "**Q1** - **P**", "**Q2** - **S**") {
		t.Fatalf("marker = %+v; a round of checks alone:\n%s", p.Marker, p.Body)
	}
}

func TestPlanQuestionWithADiagram(t *testing.T) {
	x := newH(t, "", nil)
	q := declared("Flow", "Where does it branch?", "It is simpler.")
	q["diagram"] = "flowchart TD\n  A[Open] --> B:::focus\n"
	x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{q}}))
	p := x.startPlan()
	want := "Where does it branch?\n\n```mermaid\nflowchart TD\n  A[Open] --> B:::focus\n```\n\nA: Keep it"
	if !strings.Contains(p.Body, want) {
		t.Fatalf("the diagram is not between the body and the options:\n%s", p.Body)
	}
	if x.audit.has("plan.diagram_dropped", nil) {
		t.Fatal("a diagram that passes the check was logged as dropped")
	}
}

func TestPlanQuestionDropsADiagramItDoesNotAllow(t *testing.T) {
	x := newH(t, "", nil)
	q := declared("Share", "How much is each?", "It is even.")
	q["diagram"] = "pie title Share\n  \"a\" : 1\n"
	x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{q}}))
	p := x.startPlan()
	if p.Marker.Reason != "plan-questions" || strings.Contains(p.Body, "```mermaid") || strings.Contains(p.Body, "pie title") {
		t.Fatalf("marker=%+v; the diagram was written:\n%s", p.Marker, p.Body)
	}
	if !x.audit.has("plan.diagram_dropped", func(f map[string]any) bool {
		return f["issue"] == x.ref.String() && f["q"] == 1 && f["run"] == p.Marker.Run && strings.Contains(f["reason"].(string), "pie")
	}) {
		t.Fatal("the dropped diagram is not in the audit log with its issue, run, number and reason")
	}
}

func TestPlanQuestionDroppedDiagramHasTheNumberItIsPostedWith(t *testing.T) {
	x := newH(t, "", nil)
	q := weighed("Share", "check")
	q["diagram"] = "pie title Share\n  \"a\" : 1\n"
	x.step(1, result(map[string]any{"status": "questions", "title": "T", "questions": []any{q, weighed("Scope", "decision")}}))
	p := x.startPlan()
	if !inOrder(p.Body, "**Q1** - **Scope**", "**Q2** - **Share**") {
		t.Fatalf("the check is not posted second:\n%s", p.Body)
	}
	if !x.audit.has("plan.diagram_dropped", func(f map[string]any) bool { return f["q"] == 2 }) {
		t.Fatal("the dropped diagram is not logged with Q2, the number the check is posted with")
	}
}

func TestDiagramProblem(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string // want is the diagram as written, or "" when it is dropped
	}{
		{"flowchart", "flowchart TD\n  A --> B", "flowchart TD\n  A --> B"},
		{"state", "\n\nstateDiagram-v2\n  [*] --> Idle\n", "stateDiagram-v2\n  [*] --> Idle"},
		{"sequence", "sequenceDiagram\n  A->>B: hi", "sequenceDiagram\n  A->>B: hi"},
		{"focus mark", "flowchart TD\n  A --> B:::focus", "flowchart TD\n  A --> B:::focus"},
		{"wrapped in a fence", "```mermaid\nflowchart TD\n  A --> B\n```", "flowchart TD\n  A --> B"},
		{"a fence inside", "flowchart TD\n  A --> B\n```\nnot a diagram", ""},
		{"another kind", "pie title x\n  \"a\" : 1", ""},
		{"graph", "graph TD\n  A --> B", ""},
		{"classDef", "flowchart TD\n  A --> B\n  classDef warn fill:#f00", ""},
		{"style", "flowchart TD\n  A --> B\n  style A fill:#f00", ""},
		{"linkStyle", "flowchart TD\n  A --> B\n  linkStyle 0 stroke:#f00", ""},
		{"click", "flowchart TD\n  A --> B\n  click A callback", ""},
		{"directive", "%%{init: {'theme': 'dark'}}%%\nflowchart TD\n  A --> B", ""},
		{"a label that says style", "flowchart TD\n  A[style of the page] --> B", "flowchart TD\n  A[style of the page] --> B"},
		{"empty", "  \n ", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, problem := diagramOf(tc.src)
			if got != tc.want || (tc.want == "") != (problem != "") {
				t.Fatalf("diagramOf(%q) = %q, %q; want %q", tc.src, got, problem, tc.want)
			}
		})
	}
}

// A plan run that may take pictures may write in their place, and nowhere else: it writes the scripts for the
// pictures there (V58: a heredoc with such a script can be refused).
func TestPlanRunSettingsWriteThePlaceOfTheImages(t *testing.T) {
	s := planSettings(false, "/bin/switchboard shot --browser /bin/hs", "/state/worktrees/@gitlab/nas/sb/issue-3/.switchboard/plan-images")
	// The Write tool is checked against the Edit rules: a Write(...) rule lets nothing through (V58).
	if !slices.Contains(s.Allow, "Edit(//state/worktrees/@gitlab/nas/sb/issue-3/.switchboard/plan-images/**)") {
		t.Errorf("allow = %v, want Write in the place of the images", s.Allow)
	}
	if slices.Contains(s.Allow, "Write") || slices.Contains(s.Deny, "Write") || !slices.Contains(s.Deny, "Edit") {
		t.Errorf("settings = %+v, want Write only by its rule and Edit denied", s)
	}
	if got := strings.Join(planToolsFor(false, true), ","); got != "Bash,Read,Glob,Grep,WebSearch,Write" {
		t.Errorf("tools = %s", got)
	}
}
