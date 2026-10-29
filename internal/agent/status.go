package agent

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/state"
)

// agentInfo is agent.json: which process runs the agent.
type agentInfo struct {
	PID     int       `json:"pid"`
	Host    string    `json:"host"`
	Started time.Time `json:"started"`
}

func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Status writes this host's projects, the issues being run, and the result of the last run.
func Status(cfg *config.Config, d state.Dir, w io.Writer) error {
	var info agentInfo
	running := d.ReadJSON("agent.json", &info) == nil && alive(info.PID)
	fmt.Fprintf(w, "host: %s\n", cfg.Host.Name)
	if running {
		fmt.Fprintf(w, "agent: running (pid %d, since %s)\n", info.PID, info.Started.Format(time.RFC3339))
	} else {
		fmt.Fprintln(w, "agent: not running")
	}
	fmt.Fprintln(w, "projects:")
	for _, p := range cfg.ProjectsFor(cfg.Host.Name) {
		fmt.Fprintf(w, "  %s  %s\n", p.Ref, p.Path)
	}
	var recs []record
	root := filepath.Join(d.Path, "issues")
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		rel, _ := filepath.Rel(d.Path, path)
		var r record
		if d.ReadJSON(rel, &r) == nil && r.State == "running" {
			recs = append(recs, r)
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Issue < recs[j].Issue })
	fmt.Fprintln(w, "runs:")
	if len(recs) == 0 {
		fmt.Fprintln(w, "  none")
	}
	for _, r := range recs {
		how := "running since " + r.Started.Format(time.RFC3339)
		if !running {
			how = "interrupted (started " + r.Started.Format(time.RFC3339) + "; recovered when the agent starts)"
		}
		fmt.Fprintf(w, "  %s  %s  run %s\n", r.Issue, how, r.Run)
	}
	var last record
	if d.ReadJSON("last.json", &last) == nil {
		fmt.Fprintf(w, "last run: %s  %s  ended %s\n", last.Issue, last.Result, last.Ended.Format(time.RFC3339))
	}
	return nil
}
