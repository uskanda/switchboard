package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
)

// fixture is a step that prints a recorded stream-json output from the claudecode package.
func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "claudecode", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	return "sed -e \"s/__SESSION__/$session/g\" -e \"s|__CWD__|$PWD|g\" '" + path + "'\n"
}

func TestDeclaration(t *testing.T) {
	cases := []struct {
		name       string
		res        claudecode.Result
		wantStatus string
		wantErr    string
	}{
		{"waiting", claudecode.Result{HasResult: true, Subtype: "success", Structured: []byte(`{"status":"waiting","summary":"s","question":"q"}`)}, "waiting", ""},
		{"waiting without a question", claudecode.Result{HasResult: true, Subtype: "success", Structured: []byte(`{"status":"waiting","summary":"s"}`)}, "waiting", ""},
		{"done", claudecode.Result{HasResult: true, Subtype: "success", Structured: []byte(`{"status":"done","summary":"s"}`)}, "done", ""},
		{"unknown status", claudecode.Result{HasResult: true, Subtype: "success", Structured: []byte(`{"status":"maybe","summary":"s"}`)}, "", "status"},
		{"extra field", claudecode.Result{HasResult: true, Subtype: "success", Structured: []byte(`{"status":"done","summary":"s","x":1}`)}, "", "x"},
		{"no summary", claudecode.Result{HasResult: true, Subtype: "success", Structured: []byte(`{"status":"done"}`)}, "", "summary"},
		{"no structured output", claudecode.Result{HasResult: true, Subtype: "success"}, "", "structured"},
		{"no result", claudecode.Result{ExitCode: 1}, "", "exit code 1"},
		{"is_error", claudecode.Result{HasResult: true, IsError: true, Subtype: "error_during_execution"}, "", "error_during_execution"},
		{"exit code", claudecode.Result{HasResult: true, ExitCode: 2, Subtype: "success", Structured: []byte(`{"status":"done","summary":"s"}`)}, "", "exit code 2"},
	}
	for _, c := range cases {
		d, problem := declaration(&c.res)
		if c.wantErr != "" {
			if !strings.Contains(problem, c.wantErr) {
				t.Errorf("%s: problem = %q, want it to mention %q", c.name, problem, c.wantErr)
			}
			continue
		}
		if problem != "" || d.Status != c.wantStatus {
			t.Errorf("%s: %+v, %q", c.name, d, problem)
		}
	}
}

func TestRecordedMaxTurnsBecomesAnError(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, fixture(t, "max-turns.jsonl"))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != "error" || !strings.Contains(p.Body, "turn") {
		t.Fatalf("labels=%q post=%+v\n%s", x.labels(x.ref), p.Marker, p.Body)
	}
}

func TestRecordedDenialIsReported(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, fixture(t, "waiting-denied.jsonl"))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if p.Marker.State != "waiting" || p.Marker.Reason != "permission" || !strings.Contains(p.Body, "touch created.txt") {
		t.Fatalf("post = %+v\n%s", p.Marker, p.Body)
	}
}

func TestWaitingMarkerAcknowledgesTheLastComment(t *testing.T) {
	x := newH(t, "", nil)
	c := x.f.AddComment(x.ref, "Keep it short.", x.master)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if p.Marker.Ack != c.ID || p.Marker.Host != "test" || p.Marker.Session == "" || p.Marker.Run == "" {
		t.Fatalf("marker = %+v, want ack %d", p.Marker, c.ID)
	}
	if !strings.Contains(x.file("prompt-1.md"), "Keep it short.") {
		t.Fatal("the comment is not in the prompt")
	}
}

func TestInterruptedWriteBackIsNotPostedTwice(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.agent()
	// The first SetLabels starts the run; the second, after the comment is posted, fails.
	x.f.FailNthSetLabels(2, errors.New("connection reset"))
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:running" || len(x.posts(x.ref)) != 1 {
		t.Fatalf("before the restart: labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
	x.agent() // restart
	if x.labels(x.ref) != "agent:waiting" || len(x.posts(x.ref)) != 1 {
		t.Fatalf("after the restart: labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
	if p := x.lastPost(x.ref); p.Marker.Reason != "question" {
		t.Fatalf("the recovery replaced the outcome: %+v", p.Marker)
	}
}

func TestDoneOpensADraftPullRequest(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Projects[0].VerifyCommand = "test -f flag.txt" })
	x.step(1, commit("flag.txt", "dry-run")+done("Added --dry-run and its test."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if !x.originHas("switchboard/issue-12") {
		t.Fatal("the issue branch was not pushed")
	}
	if len(x.f.PRs) != 1 {
		t.Fatalf("PRs = %d", len(x.f.PRs))
	}
	pr := x.f.PRs[0]
	if !pr.Draft || pr.Base != "main" || pr.Head != "switchboard/issue-12" || pr.Title != "Add a flag" {
		t.Fatalf("PR = %+v", pr)
	}
	for _, want := range []string{"Closes #12", "Added --dry-run and its test.", "test -f flag.txt"} {
		if !strings.Contains(pr.Body, want) {
			t.Errorf("PR body lacks %q:\n%s", want, pr.Body)
		}
	}
	p := x.lastPost(x.ref)
	if x.labels(x.ref) != "agent:done" || p.Marker.State != "done" || !strings.Contains(p.Body, "https://forge.test/o/r/pull/1") {
		t.Fatalf("labels=%q post=%+v\n%s", x.labels(x.ref), p.Marker, p.Body)
	}
}

func TestFailedVerificationBlocksThePush(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Projects[0].VerifyCommand = "echo boom-from-verify; exit 1" })
	x.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.originHas("switchboard/issue-12") || len(x.f.PRs) != 0 {
		t.Fatal("a change that failed verification was pushed")
	}
	p := x.lastPost(x.ref)
	if x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != "verify" || !strings.Contains(p.Body, "boom-from-verify") {
		t.Fatalf("labels=%q post=%+v\n%s", x.labels(x.ref), p.Marker, p.Body)
	}
}

func TestEmptyVerifyCommandIsStatedInThePullRequest(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if len(x.f.PRs) != 1 || !strings.Contains(x.f.PRs[0].Body, "not verified") {
		t.Fatalf("PRs = %+v", x.f.PRs)
	}
}

func TestProtectedPathBlocksThePush(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, commit(".github/workflows/ci.yml", "on: push")+done("Changed CI."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.originHas("switchboard/issue-12") || p.Marker.Reason != "verify" || !strings.Contains(p.Body, ".github/workflows/ci.yml") {
		t.Fatalf("post = %+v\n%s", p.Marker, p.Body)
	}
}

func TestSecretBlocksThePushWithoutBeingPosted(t *testing.T) {
	x := newH(t, "", nil)
	token := "ghp_" + strings.Repeat("Zx9", 12)
	x.step(1, commit("config.go", "key = "+token)+done("Added the key."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.originHas("switchboard/issue-12") || p.Marker.Reason != "verify" || !strings.Contains(p.Body, "config.go") {
		t.Fatalf("post = %+v\n%s", p.Marker, p.Body)
	}
	if strings.Contains(p.Body, token) {
		t.Fatal("the post contains the secret")
	}
}

func TestSecretInTheOutputIsWithheld(t *testing.T) {
	x := newH(t, "", nil)
	token := "ghp_" + strings.Repeat("Zx9", 12)
	x.step(1, waiting("Found "+token+" in the logs.", "Rotate it?"))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if p := x.lastPost(x.ref); strings.Contains(p.Body, token) || !strings.Contains(p.Body, "withheld") {
		t.Fatalf("post:\n%s", p.Body)
	}
}

func TestJapaneseComments(t *testing.T) {
	x := newH(t, "comments: {language: ja}", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if p := x.lastPost(x.ref); !strings.Contains(p.Body, "エージェントからの質問") {
		t.Fatalf("post:\n%s", p.Body)
	}
	if !strings.Contains(x.file("prompt-1.md"), "Please add --dry-run.") {
		t.Fatal("prompt")
	}
}

func TestFailingCheckCommandStillPosts(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Comments.CheckCommand = "grep -q nothing-like-this" })
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if len(x.posts(x.ref)) != 1 {
		t.Fatal("the comment was not posted")
	}
	if !x.audit.has("comment.check_failed", nil) {
		t.Fatal("the failed check is not in the audit log")
	}
}

func TestPassingCheckCommandGetsTheCommentFile(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Comments.CheckCommand = "grep -q 'Which default'" })
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.audit.has("comment.check_failed", nil) {
		t.Fatal("the check did not get the comment file")
	}
}

func TestVerifyCommandGetsNoToken(t *testing.T) {
	var x *h
	x = newH(t, "", func(c *config.Config) { c.Projects[0].VerifyCommand = "env > ../verify-env" })
	x.envExtra = []string{"GH_TOKEN=ghp_secret", "SWITCHBOARD_TOKEN_T=ghp_other"}
	x.step(1, commit("flag.txt", "dry-run")+done("Added it."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	data, err := os.ReadFile(filepath.Join(x.dir.Path, "worktrees", "o", "r", "verify-env"))
	if err != nil {
		t.Fatal(err)
	}
	env := "\n" + string(data)
	if strings.Contains(env, "\nGH_TOKEN=") || strings.Contains(env, "\nSWITCHBOARD_TOKEN_T=") {
		t.Fatal("verify_command got a token")
	}
	if !strings.Contains(env, "\nPATH=") {
		t.Fatal("verify_command got no PATH")
	}
}
