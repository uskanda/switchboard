package config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"
)

// WatchInterval is how often a Watcher looks at the configuration file.
const WatchInterval = 10 * time.Second

// Change is a new, valid configuration file as a running process applies it: the new projects, agent.max_parallel
// and agent.prefer_plan, and the sections that changed too but take a restart.
type Change struct {
	Projects    []Project
	MaxParallel int      // agent.max_parallel
	PreferPlan  bool     // agent.prefer_plan
	Ignored     []string // the YAML keys of the changed sections other than projects, in file order
}

// Watcher notices edits of the configuration file, so that a running agent or inbox service can take new projects
// without a restart. Only projects, agent.max_parallel and agent.prefer_plan are applied; everything else stays as
// the process started with it.
type Watcher struct {
	path    string
	running Config // the configuration the process started with, without its projects
	data    []byte // the content last seen
	mod     time.Time
	size    int64
}

// NewWatcher watches the file c was loaded from, starting from its current content.
func NewWatcher(c *Config) (*Watcher, error) {
	w := &Watcher{path: c.Path, running: *c}
	w.running.Projects = nil
	fi, err := os.Stat(c.Path)
	if err != nil {
		return nil, err
	}
	if w.data, err = os.ReadFile(c.Path); err != nil {
		return nil, err
	}
	w.mod, w.size = fi.ModTime(), fi.Size()
	return w, nil
}

// Check looks at the file once. It returns nil, nil when the content is as last seen, and an error when the new
// content does not validate; the process then keeps its configuration. A file is reported once per content.
func (w *Watcher) Check() (*Change, error) {
	fi, err := os.Stat(w.path)
	if err != nil {
		return nil, err
	}
	if fi.ModTime().Equal(w.mod) && fi.Size() == w.size {
		return nil, nil
	}
	w.mod, w.size = fi.ModTime(), fi.Size()
	data, err := os.ReadFile(w.path)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(data, w.data) {
		return nil, nil
	}
	w.data = data
	c, err := parse(w.path, data)
	if err != nil {
		return nil, err
	}
	return &Change{Projects: c.Projects, MaxParallel: c.Agent.MaxParallel, PreferPlan: c.Agent.PreferPlan, Ignored: w.changedSections(c)}, nil
}

// changedSections names the top-level sections other than projects in which c differs from the running one, leaving
// agent.max_parallel and agent.prefer_plan out.
func (w *Watcher) changedSections(c *Config) []string {
	var out []string
	next := *c
	next.Agent.MaxParallel = w.running.Agent.MaxParallel
	next.Agent.PreferPlan = w.running.Agent.PreferPlan
	a, b := reflect.ValueOf(w.running), reflect.ValueOf(next)
	t := a.Type()
	for i := range t.NumField() {
		key, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
		if key == "" || key == "-" || key == "projects" {
			continue
		}
		if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
			out = append(out, key)
		}
	}
	return out
}

// Run checks the file every WatchInterval until ctx ends. apply gets each new valid file; errors and the sections
// left for a restart go to log.
func (w *Watcher) Run(ctx context.Context, log io.Writer, apply func(*Change)) {
	t := time.NewTicker(WatchInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		ch, err := w.Check()
		if err != nil {
			fmt.Fprintf(log, "config: %v; keeping the configuration in use\n", err)
			continue
		}
		if ch == nil {
			continue
		}
		if len(ch.Ignored) > 0 {
			fmt.Fprintf(log, "config: %s changed; restart to apply them (only projects, agent.max_parallel and agent.prefer_plan are read again)\n", strings.Join(ch.Ignored, ", "))
		}
		apply(ch)
	}
}
