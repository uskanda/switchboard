package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// uploadCounter is a forge.Uploader that counts its uploads.
type uploadCounter struct{ n int }

func newUploadCounter() *uploadCounter { return &uploadCounter{} }

func (u *uploadCounter) Upload(_ context.Context, _ forge.IssueRef, path string) (string, error) {
	u.n++
	return "![x](/uploads/x/" + filepath.Base(path) + ")", nil
}

// pngFile writes a file that begins as a PNG does into the plan images of the worktree, from a fake claude step.
func pngFile(name string) string {
	return "mkdir -p .switchboard/plan-images\nprintf '\\211PNG\\r\\n\\032\\nimage' > .switchboard/plan-images/" + name + "\n"
}

// imageQuestion declares one question with three options, a diagram, the question's image and the images of options.
func imageQuestion(image string, options map[string]string) string {
	q := map[string]any{"title": "Layout", "body": "Where does the save button go?",
		"options": []string{"Keep it", "Move it up", "Drop it"}, "recommended": "B", "reason": "It is seen first.",
		"weight": "decision", "diagram": "flowchart TD\n  A --> B"}
	if image != "" {
		q["image"] = image
	}
	if options != nil {
		q["option_images"] = options
	}
	return result(map[string]any{"status": "questions", "title": "Move the save button", "questions": []any{q}})
}

// newImageGL is a GitLab world whose agent finds browser, or fails with err.
func newImageGL(t *testing.T, browser string, err error, edit func(*config.Config)) *gl {
	t.Helper()
	x := newGL(t, edit)
	x.edit = func(a *Agent) {
		a.FindBrowser = func() (string, error) { return browser, err }
		a.Executable = "/opt/switchboard/bin/switchboard"
	}
	x.agent()
	return x
}

func (x *gl) startGLPlan() {
	x.t.Helper()
	x.g.AddLabel(x.gref, "agent:plan", x.master)
	if err := x.a.Poll(x.ctx); err != nil {
		x.t.Fatal(err)
	}
	x.settle()
}

func (x *gl) lastGPost() string {
	x.t.Helper()
	ps := x.gposts()
	if len(ps) == 0 {
		x.t.Fatal("no post on the GitLab issue")
	}
	return ps[len(ps)-1].Body
}

func TestGitLabPlanQuestionGetsItsImages(t *testing.T) {
	x := newImageGL(t, "/usr/bin/chromium", nil, nil)
	x.step(1, pngFile("q.png")+pngFile("b.png")+imageQuestion("q.png", map[string]string{"B": "b.png"}))
	x.startGLPlan()
	body := x.lastGPost()
	if !slices.Equal(x.g.Uploaded, []string{"q.png", "b.png"}) {
		t.Fatalf("uploaded %v, want q.png then b.png", x.g.Uploaded)
	}
	q := strings.Index(body, "![q.png](/uploads/")
	a := strings.Index(body, "A: Keep it\nB: Move it up\n![b.png](/uploads/")
	c := strings.Index(body, "/b.png)\nC: Drop it")
	if q < 0 || a < q || c < a || strings.Index(body, "Where does the save button go?") > q {
		t.Fatalf("the images are not in place:\n%s", body)
	}
	if strings.Contains(body, "```mermaid") {
		t.Errorf("the diagram of a question with an image was kept:\n%s", body)
	}
	if !x.audit.has("plan.diagram_dropped", func(e map[string]any) bool { return e["reason"] == imageWithImage }) {
		t.Error("the dropped diagram is not logged")
	}
	sys := x.file("system-1.md")
	if !strings.Contains(sys, "## Images") || !strings.Contains(sys, "/opt/switchboard/bin/switchboard shot --browser /usr/bin/chromium --url") {
		t.Errorf("the plan run was not told how to take pictures:\n%s", sys)
	}
	settings := x.file("settings-1.json")
	if !strings.Contains(settings, `"Bash(/opt/switchboard/bin/switchboard shot --browser /usr/bin/chromium *)"`) {
		t.Errorf("the shot command is not allowed:\n%s", settings)
	}
	if !strings.Contains(settings, `"Edit(//`) || !strings.Contains(settings, `/@gitlab/nas/sb/issue-3/.switchboard/plan-images/**)"`) {
		t.Errorf("the plan run may not write its scripts in the place of the images:\n%s", settings)
	}
	if !strings.Contains(x.file("argv-1"), "Bash,Read,Glob,Grep,WebSearch,Write") {
		t.Errorf("the plan run lacks the Write tool:\n%s", x.file("argv-1"))
	}
	wt := filepath.Join(x.dir.Path, "worktrees", "@gitlab", "nas", "sb", "issue-3", ".switchboard", "plan-images")
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("the plan images are left in the worktree: %v", err)
	}
}

func TestGitLabPlanImagesThatFailTheCheckAreDropped(t *testing.T) {
	x := newImageGL(t, "/usr/bin/chromium", nil, nil)
	x.step(1, "mkdir -p .switchboard/plan-images\necho '<svg/>' > .switchboard/plan-images/layout.svg\n"+pngFile("a.png")+
		imageQuestion("layout.svg", map[string]string{"A": "missing.png", "F": "a.png", "C": "../a.png"}))
	x.startGLPlan()
	body := x.lastGPost()
	if len(x.g.Uploaded) != 0 || strings.Contains(body, "/uploads/") {
		t.Fatalf("uploaded %v:\n%s", x.g.Uploaded, body)
	}
	if !strings.Contains(body, "Some images of this question could not be attached.") {
		t.Fatalf("the question does not say its images were dropped:\n%s", body)
	}
	for _, why := range []string{imageBadType, imageNoFile, imageBadLabel, imageBadName} {
		if !x.audit.has("plan.image_dropped", func(e map[string]any) bool { return e["reason"] == why && e["q"] == 1 }) {
			t.Errorf("no plan.image_dropped for %q", why)
		}
	}
	if !strings.Contains(body, "```mermaid") {
		t.Errorf("a question whose image was dropped lost its diagram as well:\n%s", body)
	}
}

func TestGitLabPlanImageUploadFailure(t *testing.T) {
	x := newImageGL(t, "/usr/bin/chromium", nil, nil)
	x.g.FailUpload = errors.New("gitlab: 500")
	x.step(1, pngFile("q.png")+imageQuestion("q.png", nil))
	x.startGLPlan()
	body := x.lastGPost()
	if !strings.Contains(body, "Some images of this question could not be attached.") || !strings.Contains(body, "A: Keep it") {
		t.Fatalf("post:\n%s", body)
	}
	if !x.audit.has("plan.image_dropped", func(e map[string]any) bool {
		r, _ := e["reason"].(string)
		return strings.HasPrefix(r, imageUploadFail)
	}) {
		t.Error("the failed upload is not logged")
	}
}

func TestPlanRunWithoutABrowserGetsNoImages(t *testing.T) {
	x := newImageGL(t, "", errors.New("no browser"), nil)
	x.step(1, imageQuestion("", nil))
	x.startGLPlan()
	if sys := x.file("system-1.md"); strings.Contains(sys, "## Images") {
		t.Fatal("a plan run without a browser was told about images")
	}
}

func TestGitHubPlanRunGetsNoImages(t *testing.T) {
	x := newH(t, "", nil)
	x.agentWith(func(a *Agent) { a.FindBrowser = func() (string, error) { return "/usr/bin/chromium", nil } })
	x.step(1, pngFile("q.png")+imageQuestion("q.png", map[string]string{"A": "q.png"}))
	p := x.startPlan()
	if sys := x.file("system-1.md"); strings.Contains(sys, "## Images") {
		t.Fatal("a GitHub plan run was told about images")
	}
	if len(x.f.Uploaded) != 0 || strings.Contains(p.Body, "could not be attached") || strings.Contains(p.Body, "q.png") {
		t.Fatalf("a GitHub question got images: %v\n%s", x.f.Uploaded, p.Body)
	}
}

func TestPlanPrototypeChangesAreThrownAwayQuietly(t *testing.T) {
	x := newImageGL(t, "/usr/bin/chromium", nil, nil)
	x.cfg.Projects[len(x.cfg.Projects)-1].PlanPrototype = true
	x.agent() // again, with the project changed
	x.step(1, "echo tried > README.md\necho new > Draft.tsx\n"+pngFile("b.png")+imageQuestion("", map[string]string{"B": "b.png"}))
	x.startGLPlan()
	body := x.lastGPost()
	if strings.Contains(body, "discarded") || !strings.Contains(body, "![b.png](/uploads/") {
		t.Fatalf("post:\n%s", body)
	}
	wt := filepath.Join(x.dir.Path, "worktrees", "@gitlab", "nas", "sb", "issue-3")
	if b, _ := os.ReadFile(filepath.Join(wt, "README.md")); string(b) != "gitlab sandbox\n" {
		t.Errorf("README.md = %q, want the prototype thrown away", b)
	}
	if _, err := os.Stat(filepath.Join(wt, "Draft.tsx")); !os.IsNotExist(err) {
		t.Errorf("Draft.tsx is still there: %v", err)
	}
	if settings := x.file("settings-1.json"); !strings.Contains(settings, `"Write"`) {
		t.Errorf("Write is not allowed:\n%s", settings)
	}
	if !strings.Contains(x.file("system-1.md"), "Edit and Write") {
		t.Error("the prototype step is missing from the system prompt")
	}
}

// The images past the first 10 of a round are dropped, counted in the order of the questions, the question's image
// first and then the options by label.
func TestPlanImagesStopAtTen(t *testing.T) {
	dir := t.TempDir()
	var qs []PlanQuestion
	for range 6 {
		q := PlanQuestion{Title: "T", Body: "B", Options: []string{"x", "y"}, Recommended: "A", Reason: "r", Weight: weightDecision,
			OptionImages: map[string]string{"A": "a.png", "B": "b.png"}}
		qs = append(qs, q)
	}
	for _, n := range []string{"a.png", "b.png"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("\x89PNG\r\n\x1a\nimg"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rec := &recorder{}
	a := &Agent{Audit: rec}
	f := newUploadCounter()
	d := &PlanDeclaration{Status: "questions", Questions: qs}
	a.attachPlanImages(t.Context(), forge.IssueRef{Forge: forge.GitLab, Owner: "nas", Repo: "sb", Number: 3}, "run", d, 1, dir, f)
	if f.n != 10 {
		t.Fatalf("uploaded %d, want 10", f.n)
	}
	if !d.Questions[5].imageDropped || d.Questions[4].imageDropped {
		t.Fatalf("dropped = %v %v, want only the sixth question", d.Questions[4].imageDropped, d.Questions[5].imageDropped)
	}
	if !rec.has("plan.image_dropped", func(e map[string]any) bool { return e["reason"] == imageTooMany && e["q"] == 6 }) {
		t.Error("the images past ten are not logged")
	}
}
