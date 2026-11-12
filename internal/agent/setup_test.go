package agent

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

// withSetup gives the project a setup command; __CTL__ in cmd is the fake claude's control directory, which lies
// outside the worktree.
func withSetup(t *testing.T, cmd string, edit func(*config.Config)) *h {
	t.Helper()
	x := newH(t, "", edit)
	x.cfg.Projects[0].SetupCommand = strings.ReplaceAll(cmd, "__CTL__", x.ctl)
	return x
}

func (x *h) setupRan(match func(map[string]any) bool) bool { return x.audit.has("setup.ran", match) }

func lines(s string) int { return strings.Count(s, "\n") }

func TestSetupCommandRunsInTheWorktreeBeforeEveryStart(t *testing.T) {
	x := withSetup(t, `pwd >> __CTL__/setup-dirs; cp __CTL__/count __CTL__/setup-saw 2>/dev/null; true`, nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("claude started %d times, want 1", x.starts())
	}
	if _, err := os.Stat(filepath.Join(x.ctl, "setup-saw")); err == nil {
		t.Fatal("the setup command ran after claude had started")
	}
	want := filepath.Join(x.dir.Path, "worktrees", "o", "r", "issue-12") + "\n"
	if got := x.file("setup-dirs"); got != want {
		t.Fatalf("the setup command ran in %q, want %q", got, want)
	}

	x.replyAndRun("Default off.")
	if x.starts() != 2 || lines(x.file("setup-dirs")) != 2 || strings.TrimSpace(x.file("setup-saw")) != "1" {
		t.Fatalf("starts=%d dirs=%q saw=%q, want the setup command again before the second start", x.starts(), x.file("setup-dirs"), x.file("setup-saw"))
	}
}

func TestSetupCommandRunsBeforeAPlanRun(t *testing.T) {
	x := withSetup(t, `echo ran >> __CTL__/setup-runs`, nil)
	x.step(1, planDecl("The plan."))
	x.startPlan()
	if x.starts() != 1 || lines(x.file("setup-runs")) != 1 {
		t.Fatalf("starts=%d, want one plan run after one setup", x.starts())
	}
	if !strings.Contains(x.file("system-1.md"), "## Dependencies") {
		t.Fatalf("the plan run was not told about the setup command:\n%s", x.file("system-1.md"))
	}
}

func TestNoSetupCommandRunsNothing(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.starts() != 1 || x.setupRan(nil) {
		t.Fatalf("starts=%d setup.ran=%v, want a run without a setup", x.starts(), x.setupRan(nil))
	}
	if strings.Contains(x.file("system-1.md"), "## Dependencies") {
		t.Fatal("the system prompt names a setup command the project does not have")
	}
}

func TestFailedSetupCommandStartsNoClaude(t *testing.T) {
	x := withSetup(t, `echo boom-from-setup; exit 3`, nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.starts() != 0 || x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != "error" {
		t.Fatalf("starts=%d labels=%q post=%+v\n%s", x.starts(), x.labels(x.ref), p.Marker, p.Body)
	}
	if !strings.Contains(p.Body, "echo boom-from-setup; exit 3") || !strings.Contains(p.Body, "boom-from-setup\n") {
		t.Fatalf("the post lacks the command or its output:\n%s", p.Body)
	}
	if !x.setupRan(func(e map[string]any) bool { return e["ok"] == false && e["reason"] == "exit" }) {
		t.Fatalf("no setup.ran with ok=false reason=exit: %+v", x.audit.events)
	}
}

func TestSetupCommandOverTheTimeoutStartsNoClaude(t *testing.T) {
	x := withSetup(t, `sleep 30`, func(c *config.Config) { c.Agent.Timeout = 300 * time.Millisecond })
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.starts() != 0 || x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != "error" || !strings.Contains(p.Body, "300ms") {
		t.Fatalf("starts=%d labels=%q post=%+v\n%s", x.starts(), x.labels(x.ref), p.Marker, p.Body)
	}
	if !x.setupRan(func(e map[string]any) bool { return e["ok"] == false && e["reason"] == "timeout" }) {
		t.Fatalf("no setup.ran with reason=timeout: %+v", x.audit.events)
	}
}

func TestSetupCommandThatChangesTheWorktreeStartsNoClaude(t *testing.T) {
	x := withSetup(t, `echo changed > README.md; echo new > made.txt`, nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.starts() != 0 || x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != "error" {
		t.Fatalf("starts=%d labels=%q post=%+v\n%s", x.starts(), x.labels(x.ref), p.Marker, p.Body)
	}
	if !strings.Contains(p.Body, "`README.md`") || !strings.Contains(p.Body, "`made.txt`") {
		t.Fatalf("the post lacks the changed paths:\n%s", p.Body)
	}
	if !x.setupRan(func(e map[string]any) bool { return e["ok"] == false && e["reason"] == "changed" }) {
		t.Fatalf("no setup.ran with reason=changed: %+v", x.audit.events)
	}
	// The changes stay, for a person to look at.
	if data, _ := os.ReadFile(filepath.Join(x.dir.Path, "worktrees", "o", "r", "issue-12", "made.txt")); string(data) != "new\n" {
		t.Fatal("the agent threw away what the setup command left")
	}
}

func TestCancelDuringTheSetupCommandWritesNothingBack(t *testing.T) {
	x := withSetup(t, `touch __CTL__/setup-started; sleep 30`, nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitFor("the setup command", func() bool {
		_, err := os.Stat(filepath.Join(x.ctl, "setup-started"))
		return err == nil
	})
	x.f.Close(x.ref)
	x.poll()
	x.settle()
	if x.starts() != 0 || len(x.posts(x.ref)) != 0 || x.setupRan(nil) {
		t.Fatalf("starts=%d comments=%d setup.ran=%v, want a canceled run that wrote nothing", x.starts(), len(x.posts(x.ref)), x.setupRan(nil))
	}
}

func TestSetupCommandGetsNoToken(t *testing.T) {
	x := withSetup(t, `env > __CTL__/setup-env`, nil)
	x.envExtra = []string{"GH_TOKEN=ghp_secret", "SWITCHBOARD_TOKEN_T=ghp_other"}
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	env := "\n" + x.file("setup-env")
	if strings.Contains(env, "\nGH_TOKEN=") || strings.Contains(env, "\nSWITCHBOARD_TOKEN_T=") {
		t.Fatal("setup_command got a token")
	}
	if !strings.Contains(env, "\nPATH=") {
		t.Fatal("setup_command got no PATH")
	}
}

func TestSetupCommandIsRecorded(t *testing.T) {
	x := withSetup(t, `echo fetched-everything`, nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	var run string
	ok := x.setupRan(func(e map[string]any) bool {
		run, _ = e["run"].(string)
		_, hasMS := e["ms"].(int64)
		return e["ok"] == true && e["command"] == "echo fetched-everything" && e["issue"] == x.ref.String() && hasMS
	})
	if !ok {
		t.Fatalf("no complete setup.ran: %+v", x.audit.events)
	}
	for _, e := range x.audit.events {
		if e["event"] != "setup.ran" {
			continue
		}
		for k, v := range e {
			if s, _ := v.(string); strings.Contains(s, "fetched-everything") && k != "command" {
				t.Fatalf("setup.ran holds the output in %q", k)
			}
		}
	}
	log := filepath.Join(x.dir.Path, "runs", run, "setup.log")
	data, err := os.ReadFile(log)
	if err != nil || string(data) != "fetched-everything\n" {
		t.Fatalf("setup.log = %q, %v", data, err)
	}
	if st, _ := os.Stat(log); st.Mode().Perm() != 0o600 {
		t.Fatalf("setup.log mode = %v, want 0600", st.Mode().Perm())
	}
}

func TestSetupCommandIsNotGivenToClaude(t *testing.T) {
	x := withSetup(t, `true`, nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	plain := newH(t, "", nil)
	plain.f.AddLabel(plain.ref, "agent:ready", plain.master)
	plain.poll()
	plain.settle()
	strip := func(y *h) string { return strings.ReplaceAll(y.file("settings-1.json"), filepath.Dir(y.ctl), "") }
	if strip(x) != strip(plain) {
		t.Fatalf("the settings differ with a setup command:\n%s\n---\n%s", strip(x), strip(plain))
	}
}

func TestSystemPromptNamesTheSetupCommand(t *testing.T) {
	x := withSetup(t, `true  &&   echo deps`, nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	sys := x.file("system-1.md")
	if !strings.Contains(sys, "## Dependencies") || !strings.Contains(sys, "`true && echo deps`") {
		t.Fatalf("the system prompt lacks the setup command:\n%s", sys)
	}
}

func TestReloadTakesANewSetupCommand(t *testing.T) {
	x := newH(t, "", nil)
	x.poll()
	ps := slices.Clone(x.cfg.Projects)
	ps[0].SetupCommand = "echo ran >> " + filepath.Join(x.ctl, "setup-runs")
	x.a.Reload(&config.Change{Projects: ps, MaxParallel: x.cfg.Agent.MaxParallel})
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.starts() != 1 || lines(x.file("setup-runs")) != 1 {
		t.Fatalf("starts=%d, want the run after a reload to run the new setup command", x.starts())
	}
}
