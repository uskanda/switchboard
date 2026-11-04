package agent

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// pruneEvery is how often a running agent removes the run directories past agent.retention.
const pruneEvery = 24 * time.Hour

// pruneRuns removes the directories of the runs (runs/<run-id>/) nothing wrote to for agent.retention, and returns how
// many it removed. It keeps every run that still goes, every run whose write-back is still to be tried, and the run
// each issue's record names: the next plan run reads that run's launch.json to fork from its session. A retention of 0
// keeps every run.
func (a *Agent) pruneRuns(now time.Time) int {
	keepFor := a.Config.Agent.Retention()
	if keepFor <= 0 {
		return 0
	}
	root := filepath.Join(a.State.Path, "runs")
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return 0
	}
	if err != nil {
		a.logf("prune the run directories: %v", err)
		return 0
	}
	keep := a.keptRuns()
	removed := 0
	for _, e := range entries {
		if !e.IsDir() || keep[e.Name()] {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if now.Sub(lastWrite(dir)) < keepFor {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			a.logf("prune the run directory %s: %v", dir, err)
			continue
		}
		removed++
	}
	if removed > 0 {
		a.logf("removed the directories of %d run(s) past agent.retention (%s)", removed, keepFor)
		a.Audit.Log("runs.pruned", map[string]any{"removed": removed, "retention": keepFor.String()})
	}
	return removed
}

// keptRuns are the runs pruneRuns leaves alone whatever their age: the running ones, the ones waiting for their
// write-back, and the ones the issues' records name.
func (a *Agent) keptRuns() map[string]bool {
	keep := map[string]bool{}
	a.mu.Lock()
	for _, act := range a.active {
		keep[act.run] = true
	}
	for _, w := range a.writebacks {
		keep[w.o.Run] = true
	}
	a.mu.Unlock()
	_ = filepath.WalkDir(filepath.Join(a.State.Path, "issues"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var rec record
		if json.Unmarshal(data, &rec) == nil && rec.Run != "" {
			keep[rec.Run] = true
		}
		return nil
	})
	return keep
}

// lastWrite is the newest modification time of dir and the files in it.
func lastWrite(dir string) time.Time {
	var last time.Time
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if fi, err := d.Info(); err == nil && fi.ModTime().After(last) {
			last = fi.ModTime()
		}
		return nil
	})
	return last
}
