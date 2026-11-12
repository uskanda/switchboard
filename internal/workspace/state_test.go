package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func put(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func state(t *testing.T, s *Space) GitState {
	t.Helper()
	st, err := s.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// stateSpace is a worktree whose branch ignores node_modules and tracks lock.yaml.
func stateSpace(t *testing.T) (*Space, string) {
	t.Helper()
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, dir, ".gitignore", "node_modules\n")
	commit(t, dir, "lock.yaml", "v1\n")
	return s, dir
}

func TestStateIgnoresIgnoredFiles(t *testing.T) {
	s, dir := stateSpace(t)
	before := state(t, s)
	put(t, dir, "node_modules/dep/index.js", "x\n")
	if paths, changed := Changed(before, state(t, s)); changed {
		t.Fatalf("changed = true, paths = %v, want no change for ignored files", paths)
	}
}

func TestStateSeesATrackedFileRewritten(t *testing.T) {
	s, dir := stateSpace(t)
	before := state(t, s)
	put(t, dir, "lock.yaml", "v2\n")
	paths, changed := Changed(before, state(t, s))
	if !changed || !reflect.DeepEqual(paths, []string{"lock.yaml"}) {
		t.Fatalf("changed = %v, paths = %v, want lock.yaml", changed, paths)
	}
}

func TestStateSeesANewFileThatIsNotIgnored(t *testing.T) {
	s, dir := stateSpace(t)
	before := state(t, s)
	put(t, dir, "cache/made.txt", "x\n")
	paths, changed := Changed(before, state(t, s))
	if !changed || !reflect.DeepEqual(paths, []string{"cache/made.txt"}) {
		t.Fatalf("changed = %v, paths = %v, want cache/made.txt", changed, paths)
	}
}

func TestStateAllowsWhatAnEarlierRunLeft(t *testing.T) {
	s, dir := stateSpace(t)
	put(t, dir, "lock.yaml", "left by a run\n")
	put(t, dir, "note.txt", "left by a run\n")
	before := state(t, s)
	put(t, dir, "node_modules/dep/index.js", "x\n")
	if paths, changed := Changed(before, state(t, s)); changed {
		t.Fatalf("changed = true, paths = %v, want no change", paths)
	}
}

func TestStateSeesAnAlreadyChangedFileRewrittenAgain(t *testing.T) {
	s, dir := stateSpace(t)
	put(t, dir, "lock.yaml", "left by a run\n")
	before := state(t, s)
	put(t, dir, "lock.yaml", "rewritten by setup\n")
	paths, changed := Changed(before, state(t, s))
	if !changed || len(paths) != 0 {
		t.Fatalf("changed = %v, paths = %v, want a change that names no new path", changed, paths)
	}
}

func TestStateSeesANewCommit(t *testing.T) {
	s, dir := stateSpace(t)
	before := state(t, s)
	commit(t, dir, "made.txt", "x\n")
	if _, changed := Changed(before, state(t, s)); !changed {
		t.Fatal("changed = false, want a change for a new commit")
	}
}
