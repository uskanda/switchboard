package labels

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
)

var names = config.Labels{Ready: "agent:ready", Plan: "agent:plan", Go: "agent:go", Running: "agent:running", Waiting: "agent:waiting", Done: "agent:done"}

func setup(t *testing.T) (*fake.Forge, forge.IssueRef, forge.User) {
	t.Helper()
	f := fake.New(nil)
	alice := f.AddUser("alice", 1)
	f.AddRepo("alice", "sandbox", true)
	ref := f.AddIssue("alice", "sandbox", 1, "t", "b", alice)
	return f, ref, alice
}

func TestSyncCreatesOnlyMissingLabels(t *testing.T) {
	f, _, _ := setup(t)
	projects := []config.Project{{Owner: "alice", Repo: "sandbox"}}
	created, err := Sync(context.Background(), f, projects, names)
	if err != nil || created != 6 {
		t.Fatalf("first Sync = %d, %v", created, err)
	}
	created, err = Sync(context.Background(), f, projects, names)
	if err != nil || created != 0 {
		t.Fatalf("second Sync = %d, %v", created, err)
	}
	if got := f.RepoLabels("alice", "sandbox"); len(got) != 6 {
		t.Fatalf("labels = %v", got)
	}
}

func TestNormalizeKeepsTheHighestPriority(t *testing.T) {
	got, changed := Normalize([]string{"agent:ready", "bug", "agent:waiting"}, names)
	if !changed || !reflect.DeepEqual(got, []string{"bug", "agent:waiting"}) {
		t.Fatalf("Normalize = %v, %v", got, changed)
	}
	got, changed = Normalize([]string{"agent:done", "agent:running"}, names)
	if !changed || !reflect.DeepEqual(got, []string{"agent:running"}) {
		t.Fatalf("Normalize = %v, %v", got, changed)
	}
	if _, changed := Normalize([]string{"bug", "agent:done"}, names); changed {
		t.Fatal("one state label must stay unchanged")
	}
}

func TestSwapKeepsOtherLabels(t *testing.T) {
	f, ref, alice := setup(t)
	f.AddLabel(ref, "agent:running", alice)
	f.AddLabel(ref, "bug", alice)
	s := Swapper{Forge: f, Names: names, Audit: auditFunc(func(string) {})}
	if err := s.Swap(context.Background(), ref, names.Waiting); err != nil {
		t.Fatal(err)
	}
	if got := f.Labels(ref); !reflect.DeepEqual(got, []string{"agent:waiting", "bug"}) {
		t.Fatalf("labels = %v", got)
	}
}

func TestSwapNormalizesWhenAHumanRacesIt(t *testing.T) {
	f, ref, alice := setup(t)
	f.AddLabel(ref, "agent:running", alice)
	raced := false
	f.AfterSetLabels = func(r forge.IssueRef) {
		if !raced {
			raced = true
			f.AddLabel(r, "agent:ready", alice)
		}
	}
	var logged []string
	s := Swapper{Forge: f, Names: names, Audit: auditFunc(func(event string) { logged = append(logged, event) })}
	if err := s.Swap(context.Background(), ref, names.Waiting); err != nil {
		t.Fatal(err)
	}
	if got := f.Labels(ref); !reflect.DeepEqual(got, []string{"agent:waiting"}) {
		t.Fatalf("labels = %v", got)
	}
	if len(logged) != 1 || logged[0] != "labels.normalized" {
		t.Fatalf("audit = %v", logged)
	}
}

func TestStateOf(t *testing.T) {
	if got := StateOf([]string{"bug", "agent:waiting"}, names); got != names.Waiting {
		t.Fatalf("StateOf = %q", got)
	}
	if got := StateOf([]string{"bug"}, names); got != "" {
		t.Fatalf("StateOf = %q", got)
	}
}

type auditFunc func(event string)

func (f auditFunc) Log(event string, _ map[string]any) { f(event) }

func TestSyncCreatesTheLabelsOnGitLab(t *testing.T) {
	g := fake.NewGitLab(nil)
	g.AddProject("nas/tools", "sb", "private")
	p := config.Project{Ref: "gl:nas/tools/sb", Forge: "gl", Owner: "nas/tools", Repo: "sb"}
	n, err := Sync(context.Background(), g, []config.Project{p}, names)
	if err != nil || n != 6 {
		t.Fatalf("Sync = %d, %v", n, err)
	}
	if got := strings.Join(g.RepoLabels("nas/tools", "sb"), ","); got != "agent:done,agent:go,agent:plan,agent:ready,agent:running,agent:waiting" {
		t.Fatalf("labels = %s", got)
	}
}

func TestSyncAddsTheStartLabelsToAnOlderRepository(t *testing.T) {
	f, _, _ := setup(t)
	projects := []config.Project{{Owner: "alice", Repo: "sandbox"}}
	old := names
	old.Plan, old.Go = "", ""
	if n, err := Sync(context.Background(), f, projects, old); err != nil || n != 4 {
		t.Fatalf("Sync of the four labels = %d, %v", n, err)
	}
	n, err := Sync(context.Background(), f, projects, names)
	if err != nil || n != 2 {
		t.Fatalf("Sync = %d, %v; want the two start labels only", n, err)
	}
}

func TestStartLabelsRankAsReady(t *testing.T) {
	if got := StateOf([]string{"bug", "agent:plan"}, names); got != names.Plan {
		t.Fatalf("StateOf = %q, want agent:plan", got)
	}
	if got := StateOf([]string{"agent:go", "agent:done"}, names); got != names.Go {
		t.Fatalf("StateOf = %q, want agent:go", got)
	}
	if got := StateOf([]string{"agent:plan", "agent:waiting"}, names); got != names.Waiting {
		t.Fatalf("StateOf = %q, want agent:waiting", got)
	}
	for _, l := range []string{"agent:ready", "agent:plan", "agent:go"} {
		if !IsStart(l, names) {
			t.Errorf("IsStart(%q) = false", l)
		}
	}
	if IsStart("agent:waiting", names) || IsStart("", names) {
		t.Error("IsStart is true for a label that does not start a run")
	}
}

func TestNormalizeKeepsRunningOverAStartLabel(t *testing.T) {
	got, changed := Normalize([]string{"agent:running", "agent:plan"}, names)
	if !changed || !reflect.DeepEqual(got, []string{"agent:running"}) {
		t.Fatalf("Normalize = %v, %v", got, changed)
	}
}

func TestTwoStartLabelsAreLeftForTheAgent(t *testing.T) {
	in := []string{"agent:plan", "bug", "agent:go", "agent:done"}
	got, changed := Normalize(in, names)
	if !changed || !reflect.DeepEqual(got, []string{"agent:plan", "bug", "agent:go"}) {
		t.Fatalf("Normalize = %v, %v; want both start labels kept and done removed", got, changed)
	}
	if !MultipleStarts(got, names) {
		t.Fatal("MultipleStarts = false for agent:plan and agent:go")
	}
	if MultipleStarts([]string{"agent:plan", "agent:running"}, names) {
		t.Fatal("MultipleStarts = true when running outranks the start label")
	}
	if MultipleStarts([]string{"agent:ready"}, names) {
		t.Fatal("MultipleStarts = true for one start label")
	}
}

func TestSwapRemovesTheStartLabel(t *testing.T) {
	f, ref, alice := setup(t)
	f.AddLabel(ref, "agent:plan", alice)
	f.AddLabel(ref, "bug", alice)
	s := Swapper{Forge: f, Names: names, Audit: auditFunc(func(string) {})}
	if err := s.Swap(context.Background(), ref, names.Running); err != nil {
		t.Fatal(err)
	}
	if got := f.Labels(ref); !reflect.DeepEqual(got, []string{"agent:running", "bug"}) {
		t.Fatalf("labels = %v", got)
	}
}
