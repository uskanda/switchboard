package agent

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/instructions"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/workspace"
)

// shots is a step fragment that leaves files in the place of the screenshots.
func shots(names ...string) string {
	s := "mkdir -p '" + workspace.ScreenshotDir + "'\n"
	for _, n := range names {
		s += "printf 'img' > '" + workspace.ScreenshotDir + "/" + n + "'\n"
	}
	return s
}

// attachments are the agent's comments that attach the images of a run.
func (x *h) attachments() []post {
	var out []post
	for _, p := range x.posts(x.ref) {
		if p.Marker != nil && p.Marker.State == marker.Attachment {
			out = append(out, p)
		}
	}
	return out
}

// donePost is the agent's latest done post.
func (x *h) donePost() post {
	x.t.Helper()
	ps := x.posts(x.ref)
	for i := len(ps) - 1; i >= 0; i-- {
		if ps[i].Marker != nil && ps[i].Marker.State == "done" {
			return ps[i]
		}
	}
	x.t.Fatal("no done post")
	return post{}
}

func TestTheImplementInstructionsAndTheScreenshotRulesReachOnlyTheImplementingRuns(t *testing.T) {
	x := newH(t, "", nil)
	x.saveInstructions(instructions.Run, instructions.Global, "Record an OpenSpec change first.\n")
	impl := x.saveInstructions(instructions.Implement, "gh:o/r", "Take screenshots of the changed pages.\n")
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.startPlan()
	if sys := x.file("system-1.md"); strings.Contains(sys, "Take screenshots") || strings.Contains(sys, workspace.ScreenshotDir) {
		t.Fatalf("the plan run got the implement instructions or the screenshot rules:\n%s", sys)
	}
	x.replyAndRun("/ok")
	sys := x.file("system-2.md")
	if !strings.Contains(sys, "Record an OpenSpec change first.\n\nTake screenshots of the changed pages.") {
		t.Fatalf("the approved run lacks run.md then implement.md:\n%s", sys)
	}
	if rules := strings.Index(sys, workspace.ScreenshotDir+"/"); rules < 0 || rules > strings.Index(sys, "## Additional instructions from the user") {
		t.Fatalf("the approved run lacks the screenshot rules before the user's instructions:\n%s", sys)
	}
	s := x.started()
	if len(s) != 2 {
		t.Fatalf("run.started = %v", s)
	}
	if h, _ := s[0]["implement"].(map[string]string); h["global"] != "" || h["project"] != "" {
		t.Errorf("the plan run's implement hashes = %v, want none", s[0]["implement"])
	}
	if h, _ := s[1]["implement"].(map[string]string); h["global"] != "" || h["project"] != impl {
		t.Errorf("the approved run's implement hashes = %v, want project %s", s[1]["implement"], impl)
	}
	x.noInstructionText("Take screenshots")
	if a := x.attachments(); len(a) != 0 || strings.Contains(x.donePost().Body, english.shotsNext) {
		t.Fatalf("a run without images attached something: %v", a)
	}
}

func TestAFixRunLacksTheImplementInstructionsAndTheScreenshotRules(t *testing.T) {
	w := newCI(t, "", func(c *config.Config) {
		if err := instructions.For(c.Path, nil).Of(instructions.Implement).Save("gh:o/r", "Take screenshots.\n"); err != nil {
			t.Fatal(err)
		}
	})
	if sys := w.file("system-1.md"); !strings.Contains(sys, "Take screenshots.") || !strings.Contains(sys, workspace.ScreenshotDir) {
		t.Fatalf("the direct run lacks the implement instructions or the screenshot rules:\n%s", sys)
	}
	sha := w.syncHead()
	w.f.SetChecks("o", "r", sha, failedCheck("test", 7))
	w.f.SetLog(7, "--- FAIL: TestFlag\n")
	w.step(2, commit("fix.txt", "fixed")+done("Fixed TestFlag."))
	w.poll()
	w.settle()
	if sys := w.file("system-2.md"); strings.Contains(sys, "Take screenshots.") || strings.Contains(sys, workspace.ScreenshotDir) {
		t.Fatalf("the fix run got the implement instructions or the screenshot rules:\n%s", sys)
	}
}

func TestADoneRunAttachesItsScreenshots(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, commit("a.txt", "a")+shots("a.png", "b.webp", "notes.txt")+done("Changed the page."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	d := x.donePost()
	if !strings.Contains(d.Body, english.shotsNext) || x.labels(x.ref) != "agent:done" {
		t.Fatalf("done post = %s, labels = %q", d.Body, x.labels(x.ref))
	}
	a := x.attachments()
	if len(a) != 1 || a[0].Marker.Run != d.Marker.Run || x.lastPost(x.ref).Marker.State != marker.Attachment {
		t.Fatalf("attachments = %v", a)
	}
	if len(x.f.Attached) != 1 || !slices.Equal(x.f.Attached[0], []string{"a.png", "b.webp"}) {
		t.Fatalf("attached = %v", x.f.Attached)
	}
	if !strings.Contains(a[0].Body, "notes.txt") || !strings.Contains(a[0].Body, english.shotsNotImage) {
		t.Errorf("the comment does not name the file left out:\n%s", a[0].Body)
	}
	runDir := x.a.runDir(d.Marker.Run)
	if _, err := os.Stat(filepath.Join(runDir, "screenshots", "a.png")); err != nil {
		t.Fatalf("the image was not kept in the run directory: %v", err)
	}
	wt := x.a.spaceOf(x.cfg.Projects[0], x.ref).Dir()
	if _, err := os.Stat(filepath.Join(wt, workspace.ScreenshotDir)); !os.IsNotExist(err) {
		t.Fatalf("the place is still in the worktree: %v", err)
	}
	if strings.Contains(run(t, wt, "git", "log", "--stat", "origin/main..HEAD"), workspace.ScreenshotDir) {
		t.Fatal("an image was committed")
	}

	// A write-back done again, as after a failure, finds the comment and posts no second one.
	o := &outcome{msg: x.a.msg}
	if err := x.dir.ReadJSON(filepath.Join("runs", d.Marker.Run, "outcome.json"), o); err != nil {
		t.Fatal(err)
	}
	if err := x.a.writeBack(x.ctx, x.ref, o); err != nil {
		t.Fatal(err)
	}
	if n := len(x.attachments()); n != 1 || len(x.f.Attached) != 1 {
		t.Fatalf("attachments after a second write-back = %d, attach calls = %d", n, len(x.f.Attached))
	}
}

func TestAFailedAttachmentKeepsTheDoneState(t *testing.T) {
	x := newH(t, "", nil)
	x.f.FailAttach = errors.New("gh: command not found")
	x.step(1, commit("a.txt", "a")+shots("a.png")+done("Changed the page."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	a := x.attachments()
	if len(a) != 1 || x.labels(x.ref) != "agent:done" {
		t.Fatalf("attachments = %v, labels = %q", a, x.labels(x.ref))
	}
	if !strings.Contains(a[0].Body, "gh: command not found") || strings.Contains(a[0].Body, "fake.test/attachments") {
		t.Fatalf("the comment does not say why:\n%s", a[0].Body)
	}
	if !x.audit.has("attachment.failed", nil) {
		t.Error("the failure is not in the audit log")
	}
}

func TestAWaitingRunAttachesNothingAndClearsThePlace(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, shots("a.png")+waiting("Half done.", "Which colour?"))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if a := x.attachments(); len(a) != 0 || len(x.f.Attached) != 0 {
		t.Fatalf("a waiting run attached %v", a)
	}
	wt := x.a.spaceOf(x.cfg.Projects[0], x.ref).Dir()
	if _, err := os.Stat(filepath.Join(wt, workspace.ScreenshotDir)); !os.IsNotExist(err) {
		t.Fatalf("the place is still in the worktree: %v", err)
	}
}

func TestACommittedScreenshotBlocksThePush(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, commit("a.txt", "a")+shots("a.png")+
		"git add -f '"+workspace.ScreenshotDir+"/a.png'\ngit commit -q -m shot\n"+done("Done."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if p.Marker.State != "waiting" || p.Marker.Reason != "verify" || !strings.Contains(p.Body, workspace.ScreenshotDir+"/a.png") {
		t.Fatalf("post = %+v\n%s", p.Marker, p.Body)
	}
	if x.originHas("switchboard/issue-12") {
		t.Fatal("the branch was pushed")
	}
}
