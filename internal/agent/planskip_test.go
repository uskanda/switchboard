package agent

import (
	"strconv"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/marker"
)

// implementDecl is the declaration of a plan run that skips the questions and the approval.
func implementDecl(text string) string {
	return result(map[string]any{"status": "implement", "title": "Add the greeting", "plan": text})
}

func autoProject(c *config.Config) { c.Projects[0].PlanFirst = config.PlanAuto }

// startReady puts agent:ready on the issue and lets every run it leads to end.
func (x *h) startReady() {
	x.t.Helper()
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
}

// skipComment is the agent's comment that skipped the plan stage, with its marker.
func (x *h) skipComment() (int64, string, *marker.Marker) {
	x.t.Helper()
	for _, c := range x.claims(x.ref) {
		if m, _ := marker.Parse(c.Body); m.Reason == marker.PlanSkipped {
			return c.ID, c.Body, m
		}
	}
	x.t.Fatal("no comment skipped the plan stage")
	return 0, "", nil
}

func TestAutoReadyWithAClearIssueSkipsToTheImplementation(t *testing.T) {
	x := newH(t, "", autoProject)
	x.step(1, implementDecl("1. Add greeting.txt.\n2. Test it."))
	x.step(2, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	x.startReady()
	if x.starts() != 2 || x.labels(x.ref) != "agent:done" {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
	if got := claimMode(t, x, 0); got != marker.ModeAuto {
		t.Fatalf("the first claim has mode %q, want auto", got)
	}
	id, body, m := x.skipComment()
	if m.Mode != marker.ModeDirect || m.State != marker.Running || m.Host == "" || m.Run == "" {
		t.Fatalf("skip marker = %+v, want a claim in direct mode", m)
	}
	for _, want := range []string{"**Plan:** Add the greeting", "1. Add greeting.txt.", "skip"} {
		if !strings.Contains(body, want) {
			t.Errorf("the skip comment lacks %q:\n%s", want, body)
		}
	}
	// No wait was written for the plan run: the only post besides the claims is the done.
	if ps := x.posts(x.ref); len(ps) != 1 || ps[0].Marker.State != marker.Done {
		t.Fatalf("posts = %+v, want only the done", ps)
	}
	d := x.lastPost(x.ref)
	if d.Marker.Run != m.Run {
		t.Errorf("the done post is of run %s, want the run the skip comment claimed (%s)", d.Marker.Run, m.Run)
	}
	if !strings.Contains(x.file("argv-1"), `"implement"`) {
		t.Error("the first plan run of an auto cycle was not offered the implement status")
	}
	if sys := x.file("system-1.md"); !strings.Contains(sys, `status "implement"`) || !strings.Contains(sys, "When in doubt") {
		t.Errorf("the first plan run's system prompt lacks the rules of the skip:\n%s", sys)
	}
	sys := x.file("system-2.md")
	for _, want := range []string{"comment " + strconv.FormatInt(id, 10), `status "done"`, "Screenshots for the issue", "## The sandbox"} {
		if !strings.Contains(sys, want) {
			t.Errorf("the implementing run's system prompt lacks %q:\n%s", want, sys)
		}
	}
	if strings.Contains(sys, "plan stage") || strings.Contains(sys, "approved the plan") {
		t.Errorf("the implementing run got the rules of another run:\n%s", sys)
	}
	if !strings.Contains(x.file("prompt-2.md"), "1. Add greeting.txt.") {
		t.Error("the skip comment is not in the implementing run's prompt")
	}
	if !strings.Contains(x.file("argv-2"), "Edit") {
		t.Error("the implementing run did not get the tools that write files")
	}
	if !x.audit.has("run.plan_skipped", nil) {
		t.Error("the skip is not in the audit log")
	}
}

func TestAutoReadyAsksWhenTheIssueIsUnclear(t *testing.T) {
	x := newH(t, "", autoProject)
	x.step(1, questions([3]string{"Default", "Which default?", "A."}))
	x.step(2, implementDecl("Skip after all."))
	x.startReady()
	if p := x.lastPost(x.ref); p.Marker.Reason != marker.PlanQuestions || x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("post = %+v labels=%q", p.Marker, x.labels(x.ref))
	}
	// After a round of questions the plan waits for the approval: implement is no longer offered.
	p := x.replyAndRun("/ok")
	if strings.Contains(x.file("argv-2"), `"implement"`) || strings.Contains(x.file("system-2.md"), `status "implement"`) {
		t.Error("a plan run after the questions was offered the skip")
	}
	if p.Marker.Reason != "error" || x.starts() != 2 {
		t.Fatalf("an implement declared after the questions: post = %+v, starts = %d; want an error", p.Marker, x.starts())
	}
}

func TestImplementIsRefusedOutsideAnAutoCycle(t *testing.T) {
	for name, edit := range map[string]func(*config.Config){
		"agent:plan":                         nil,
		"agent:ready with plan_first always": func(c *config.Config) { c.Projects[0].PlanFirst = config.PlanAlways },
		"agent:plan with plan_first auto":    autoProject,
	} {
		t.Run(name, func(t *testing.T) {
			x := newH(t, "", edit)
			x.step(1, implementDecl("The plan."))
			label := "agent:plan"
			if name == "agent:ready with plan_first always" {
				label = "agent:ready"
			}
			x.f.AddLabel(x.ref, label, x.master)
			x.poll()
			x.settle()
			if strings.Contains(x.file("argv-1"), `"implement"`) {
				t.Error("the schema offers the implement status")
			}
			if p := x.lastPost(x.ref); p.Marker.Reason != "error" || x.starts() != 1 {
				t.Fatalf("post = %+v starts = %d, want an error and no implementing run", p.Marker, x.starts())
			}
		})
	}
}

func TestImplementWithoutAPlanIsAnError(t *testing.T) {
	for _, d := range []map[string]any{
		{"status": "implement", "title": "T"},
		{"status": "implement", "title": "T", "plan": " \n"},
		{"status": "implement", "plan": "The plan."},
	} {
		x := newH(t, "", autoProject)
		x.step(1, result(d))
		x.startReady()
		if p := x.lastPost(x.ref); p.Marker.Reason != "error" || x.starts() != 1 {
			t.Fatalf("%v: post = %+v starts = %d", d, p.Marker, x.starts())
		}
	}
}

func TestAReplyAfterTheSkipImplementsDirectly(t *testing.T) {
	x := newH(t, "", autoProject)
	x.step(1, implementDecl("Add greeting.txt."))
	x.step(2, waiting("Looked at it.", "Which greeting?"))
	x.step(3, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	x.startReady()
	if p := x.lastPost(x.ref); p.Marker.Reason != "question" {
		t.Fatalf("post = %+v, want the implementing run's question", p.Marker)
	}
	p := x.replyAndRun("Say hi.")
	if p.Marker.State != marker.Done || x.starts() != 3 {
		t.Fatalf("post = %+v starts = %d, want done after a direct run", p.Marker, x.starts())
	}
	if sys := x.file("system-3.md"); strings.Contains(sys, "plan stage") || !strings.Contains(sys, `status "done"`) {
		t.Fatalf("the run after the reply is not a direct run:\n%s", sys)
	}
}

func TestTheSkipCommentInJapanese(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { autoProject(c); c.Comments.Language = "ja" })
	x.step(1, implementDecl("greeting.txtを足す。"))
	x.step(2, commit("greeting.txt", "hi")+done("足しました。"))
	x.startReady()
	_, body, _ := x.skipComment()
	if !strings.Contains(body, "要件が明確なので、質問と承認を省いて実装します。") || !strings.Contains(body, "greeting.txtを足す。") {
		t.Fatalf("skip comment:\n%s", body)
	}
}
