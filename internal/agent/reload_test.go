package agent

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/relay"
)

// lockedLog is a log the runs and the test may write and read at once.
type lockedLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// reloadH is the harness with a second repository, o/notes, that is not configured yet: a bare origin for it, the
// path it is to be cloned to, and a ready issue in it.
type reloadH struct {
	*h
	notes  config.Project
	origin string // the bare origin of o/notes
	issue  forge.IssueRef
	log    lockedLog
}

func newReloadH(t *testing.T) *reloadH {
	x := &reloadH{h: newH(t, "", nil)}
	root := t.TempDir()
	x.origin = filepath.Join(root, "notes.git")
	seed := filepath.Join(root, "seed")
	run(t, root, "git", "init", "-q", "--bare", "-b", "main", x.origin)
	run(t, root, "git", "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, seed, "git", "add", "README.md")
	run(t, seed, "git", "commit", "-q", "-m", "init")
	run(t, seed, "git", "push", "-q", x.origin, "main")
	x.notes = config.Project{Ref: "gh:o/notes", Path: filepath.Join(root, "repos", "notes"), Host: "test", BaseBranch: "main",
		Forge: "gh", Owner: "o", Repo: "notes"}
	x.f.AddRepo("o", "notes", true)
	x.issue = x.f.AddIssue("o", "notes", 1, "Write notes", "Please write notes.", x.master)
	x.f.AddLabel(x.issue, "agent:ready", x.master)
	x.agentWith(func(a *Agent) {
		a.Log = &x.log
		a.CloneURL = func(p config.Project) string {
			if p.Ref == "gh:o/notes" {
				return x.origin
			}
			return x.h.origin
		}
	})
	return x
}

// reload hands the agent the configured projects plus extra, as a new configuration file does.
func (x *reloadH) reload(extra ...config.Project) {
	x.a.Reload(append(slices.Clone(x.cfg.Projects), extra...))
}

func TestReloadAcceptsANewProject(t *testing.T) {
	x := newReloadH(t)
	x.poll()
	x.reload(x.notes)
	x.poll()
	x.settle()
	if _, err := os.Stat(filepath.Join(x.notes.Path, "README.md")); err != nil {
		t.Fatalf("o/notes was not cloned: %v", err)
	}
	labels, _ := x.f.LabelNames(x.ctx, "o", "notes")
	for _, want := range []string{"agent:ready", "agent:plan", "agent:go", "agent:running", "agent:waiting", "agent:done"} {
		if !slices.Contains(labels, want) {
			t.Errorf("o/notes lacks the label %s: %v", want, labels)
		}
	}
	if x.starts() != 1 || x.labels(x.issue) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q, want the ready issue of o/notes run", x.starts(), x.labels(x.issue))
	}
}

func TestStartCreatesNoLabels(t *testing.T) {
	x := newReloadH(t)
	x.poll()
	if labels, _ := x.f.LabelNames(x.ctx, "o", "r"); len(labels) != 0 {
		t.Errorf("labels of o/r = %v, want none created at start", labels)
	}
}

func TestReloadRetriesAFailedAcceptance(t *testing.T) {
	x := newReloadH(t)
	x.f.FailEnsureLabel = errors.New("label service down")
	x.reload(x.notes)
	x.poll()
	x.settle()
	if x.starts() != 0 {
		t.Fatalf("starts = %d; a project whose labels failed was run", x.starts())
	}
	if !strings.Contains(x.log.String(), "label service down") {
		t.Errorf("the failure is not logged:\n%s", x.log.String())
	}
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("starts = %d; the acceptance was not tried again", x.starts())
	}
}

func TestReloadRetriesAFailedClone(t *testing.T) {
	x := newReloadH(t)
	good := x.origin
	x.origin = filepath.Join(t.TempDir(), "missing.git")
	x.reload(x.notes)
	x.poll()
	x.settle()
	if x.starts() != 0 || !strings.Contains(x.log.String(), "clone") {
		t.Fatalf("starts=%d log:\n%s\nwant the clone failure logged and nothing run", x.starts(), x.log.String())
	}
	if _, err := os.Stat(x.notes.Path); !os.IsNotExist(err) {
		t.Fatalf("the failed clone left %s (%v)", x.notes.Path, err)
	}
	x.origin = good
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("starts = %d; the clone was not tried again", x.starts())
	}
}

func TestReloadRefusesAPathCloneOfAnotherRepository(t *testing.T) {
	x := newReloadH(t)
	x.notes.Path = x.clone // a clone of o/r
	x.reload(x.notes)
	x.poll()
	x.settle()
	if x.starts() != 0 {
		t.Fatalf("starts = %d; a project at another repository's clone was run", x.starts())
	}
	if !strings.Contains(x.log.String(), "origin") {
		t.Errorf("the mismatch is not logged:\n%s", x.log.String())
	}
}

func TestReloadRefusesAPublicProjectWithUntrustedInput(t *testing.T) {
	x := newReloadH(t)
	x.f.AddRepo("o", "notes", false)
	x.issue = x.f.AddIssue("o", "notes", 1, "Write notes", "Please write notes.", x.master)
	x.f.AddLabel(x.issue, "agent:ready", x.master)
	x.notes.IncludeUntrusted = true
	x.reload(x.notes)
	x.poll()
	x.poll()
	x.settle()
	if x.starts() != 0 {
		t.Fatalf("starts = %d; a refused project was run", x.starts())
	}
	if !x.audit.has("project.refused", nil) {
		t.Error("the refusal is not in the audit log")
	}
	if _, err := os.Stat(x.notes.Path); !os.IsNotExist(err) {
		t.Errorf("a refused project was cloned (%v)", err)
	}
}

func TestReloadIgnoresProjectsOfOtherHosts(t *testing.T) {
	x := newReloadH(t)
	x.notes.Host = "elsewhere"
	x.reload(x.notes)
	x.poll()
	x.settle()
	if x.starts() != 0 {
		t.Fatalf("starts = %d; another host's project was run", x.starts())
	}
	if _, err := os.Stat(x.notes.Path); !os.IsNotExist(err) {
		t.Errorf("another host's project was cloned (%v)", err)
	}
}

func TestReloadDropsARemovedProjectButFinishesItsRun(t *testing.T) {
	x := newReloadH(t)
	x.step(1, block(1, waiting("Looked.", "Which default?")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.a.Reload(nil)
	x.poll()
	x.release(1)
	x.settle()
	if got := x.labels(x.ref); got != "agent:waiting" {
		t.Fatalf("labels after the run = %q; the run of a removed project did not finish", got)
	}
	other := x.f.AddIssue("o", "r", 13, "Another", "More.", x.master)
	x.f.AddLabel(other, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("starts = %d; a removed project was run", x.starts())
	}
}

func TestRelayEventsReachAnAcceptedProject(t *testing.T) {
	x := newReloadH(t)
	e := relay.Event{Forge: "gh", Project: "o/notes"}
	if _, ok := x.a.projectOf(e); ok {
		t.Fatal("an event of an unconfigured project was taken")
	}
	x.reload(x.notes)
	x.poll()
	x.settle()
	if p, ok := x.a.projectOf(e); !ok || p.Ref != "gh:o/notes" {
		t.Fatalf("projectOf = %+v, %v; want o/notes after it was accepted", p, ok)
	}
}
