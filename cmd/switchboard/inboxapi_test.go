package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/instructions"
)

func TestInboxAPITakesTheRefreshInterval(t *testing.T) {
	path := writeConfig(t)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("inbox: {refresh_interval: 45s}\n")
	f.Close()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	api, _ := inboxAPI(cfg, nil, instructions.Store{})
	if api.Interval != 45*time.Second {
		t.Fatalf("interval = %v, want 45s", api.Interval)
	}
	if api.Instructions == nil || len(api.Instructions.Projects) != 1 || api.Instructions.Host != "here" {
		t.Fatalf("instructions = %+v", api.Instructions)
	}
}

func TestInboxAPIReloadTakesTheProjectsOnly(t *testing.T) {
	cfg := load(t, bothForges)
	api, reload := inboxAPI(cfg, nil, instructions.Store{})
	next := load(t, strings.Replace(bothForges, "  - {ref: \"gl:nas/b\"", "  - {ref: \"gh:o/notes\", path: /src/notes, host: here}\n  - {ref: \"gl:nas/b\"", 1))
	next.Agent.PollInterval = time.Hour
	reload(next.Projects)
	if got := strings.Join(api.Instructions.Projects, " "); got != "gh:o/r gh:o/notes gl:nas/b" {
		t.Errorf("instructions' projects = %s", got)
	}
	c := api.Projects.Config
	if len(c.Projects) != 3 || c.Agent.PollInterval != cfg.Agent.PollInterval || c.Path != cfg.Path {
		t.Errorf("config = %+v, want the new projects and the rest as started", c)
	}
}
