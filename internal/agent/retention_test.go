package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

// age makes every file of a run directory, and the directory, d old.
func age(t *testing.T, dir string, d time.Duration) {
	t.Helper()
	old := time.Now().Add(-d)
	_ = filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err == nil {
			err = os.Chtimes(path, old, old)
		}
		return err
	})
}

func mkRun(t *testing.T, x *h, run string) string {
	t.Helper()
	dir := filepath.Join(x.dir.Path, "runs", run)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte("p"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOldRunDirectoriesArePruned(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	rec, err := x.a.loadRecord(x.ref)
	if err != nil {
		t.Fatal(err)
	}
	latest := filepath.Join(x.dir.Path, "runs", rec.Run)
	age(t, latest, 40*24*time.Hour)
	old := mkRun(t, x, "old")
	age(t, old, 31*24*time.Hour)
	fresh := mkRun(t, x, "fresh")
	age(t, fresh, 29*24*time.Hour)

	if n := x.a.pruneRuns(time.Now()); n != 1 {
		t.Fatalf("removed %d", n)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the old run is still there: %v", err)
	}
	for _, kept := range []string{latest, fresh} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s was removed: %v", kept, err)
		}
	}
}

func TestARetentionOfZeroKeepsEveryRun(t *testing.T) {
	zero := time.Duration(0)
	x := newH(t, "", func(c *config.Config) { c.Agent.RetentionSet = &zero })
	x.agent()
	old := mkRun(t, x, "old")
	age(t, old, 400*24*time.Hour)
	if n := x.a.pruneRuns(time.Now()); n != 0 {
		t.Fatalf("removed %d", n)
	}
}
