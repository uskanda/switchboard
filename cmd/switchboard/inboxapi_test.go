package main

import (
	"os"
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
	api := inboxAPI(cfg, nil, instructions.Store{})
	if api.Interval != 45*time.Second {
		t.Fatalf("interval = %v, want 45s", api.Interval)
	}
	if api.Instructions == nil || len(api.Instructions.Projects) != 1 || api.Instructions.Host != "here" {
		t.Fatalf("instructions = %+v", api.Instructions)
	}
}
