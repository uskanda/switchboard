package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

// save writes body to path and moves its modification time forward, as a later save does.
func save(t *testing.T, path, body string, at time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func newWatcher(t *testing.T, body string) (*Watcher, string) {
	t.Helper()
	p := write(t, body)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewWatcher(c)
	if err != nil {
		t.Fatal(err)
	}
	return w, p
}

func TestWatcherReportsNothingWithoutAChange(t *testing.T) {
	w, _ := newWatcher(t, valid)
	if ch, err := w.Check(); ch != nil || err != nil {
		t.Errorf("Check = %+v, %v; want nothing", ch, err)
	}
}

func TestWatcherIgnoresASaveWithTheSameContent(t *testing.T) {
	w, p := newWatcher(t, valid)
	save(t, p, valid, time.Now().Add(time.Minute))
	if ch, err := w.Check(); ch != nil || err != nil {
		t.Errorf("Check = %+v, %v; want nothing for the same content", ch, err)
	}
}

func TestWatcherReturnsTheNewProjects(t *testing.T) {
	w, p := newWatcher(t, valid)
	save(t, p, valid+"  - ref: gh:alice/notes\n    path: ~/repos/notes\n    host: ubuntu-a\n", time.Now().Add(time.Minute))
	ch, err := w.Check()
	if err != nil || ch == nil {
		t.Fatalf("Check = %+v, %v; want the new projects", ch, err)
	}
	if n := len(ch.Projects); n != 3 || ch.Projects[2].Ref != "gh:alice/notes" || ch.Projects[2].Repo != "notes" {
		t.Errorf("projects = %+v, want sandbox, other and notes, parsed", ch.Projects)
	}
	if len(ch.Ignored) != 0 {
		t.Errorf("ignored = %v, want none", ch.Ignored)
	}
	if ch, err := w.Check(); ch != nil || err != nil {
		t.Errorf("second Check = %+v, %v; want nothing", ch, err)
	}
}

func TestWatcherKeepsTheOldConfigurationWhenTheNewOneIsInvalid(t *testing.T) {
	w, p := newWatcher(t, valid)
	save(t, p, strings.Replace(valid, "    path: ~/repos/sandbox\n", "", 1), time.Now().Add(time.Minute))
	ch, err := w.Check()
	if ch != nil || err == nil || !strings.Contains(err.Error(), "projects[0].path") {
		t.Fatalf("Check = %+v, %v; want the missing projects[0].path reported", ch, err)
	}
	if ch, err := w.Check(); ch != nil || err != nil {
		t.Errorf("Check of the same invalid file = %+v, %v; want it reported once", ch, err)
	}
}

func TestWatcherNamesTheSectionsItDoesNotApply(t *testing.T) {
	w, p := newWatcher(t, valid)
	save(t, p, valid+"agent:\n  poll_interval: 2m\ninbox:\n  refresh_interval: 1m\n", time.Now().Add(time.Minute))
	ch, err := w.Check()
	if err != nil || ch == nil {
		t.Fatalf("Check = %+v, %v; want a change", ch, err)
	}
	if strings.Join(ch.Ignored, ",") != "agent,inbox" {
		t.Errorf("ignored = %v, want [agent inbox]", ch.Ignored)
	}
	if len(ch.Projects) != 2 {
		t.Errorf("projects = %+v, want the two configured", ch.Projects)
	}
}

func TestWatcherAppliesTheAgentPreferPlan(t *testing.T) {
	w, p := newWatcher(t, valid)
	save(t, p, valid+"agent:\n  prefer_plan: true\n", time.Now().Add(time.Minute))
	ch, err := w.Check()
	if err != nil || ch == nil {
		t.Fatalf("Check = %+v, %v; want a change", ch, err)
	}
	if !ch.PreferPlan {
		t.Error("prefer plan = false, want true")
	}
	if len(ch.Ignored) != 0 {
		t.Errorf("ignored = %v, want none: agent.prefer_plan is applied", ch.Ignored)
	}
}

func TestWatcherAppliesTheAgentMaxParallel(t *testing.T) {
	w, p := newWatcher(t, valid)
	save(t, p, valid+"agent:\n  max_parallel: 2\n", time.Now().Add(time.Minute))
	ch, err := w.Check()
	if err != nil || ch == nil {
		t.Fatalf("Check = %+v, %v; want a change", ch, err)
	}
	if ch.MaxParallel != 2 {
		t.Errorf("max parallel = %d, want 2", ch.MaxParallel)
	}
	if len(ch.Ignored) != 0 {
		t.Errorf("ignored = %v, want none: agent.max_parallel is applied", ch.Ignored)
	}
	save(t, p, valid+"agent:\n  max_parallel: 2\n  poll_interval: 2m\n", time.Now().Add(2*time.Minute))
	ch, err = w.Check()
	if err != nil || ch == nil || strings.Join(ch.Ignored, ",") != "agent" {
		t.Errorf("Check = %+v, %v; want agent ignored for poll_interval", ch, err)
	}
}
