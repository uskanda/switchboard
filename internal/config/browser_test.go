package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrowserAndPlanPrototypeDefault(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Agent.Browser != "" {
		t.Errorf("agent.browser = %q, want empty", c.Agent.Browser)
	}
	for _, p := range c.Projects {
		if p.PlanPrototype {
			t.Errorf("%s: plan_prototype defaults to true", p.Ref)
		}
	}
}

func TestBrowserAndPlanPrototypeAreRead(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	exe := filepath.Join(home, "chrome")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(valid, "    host: ubuntu-a\n", "    host: ubuntu-a\n    plan_prototype: true\n", 1) +
		"agent:\n  browser: ~/chrome\n"
	c, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if c.Agent.Browser != exe {
		t.Errorf("agent.browser = %q, want %q", c.Agent.Browser, exe)
	}
	if !c.Projects[0].PlanPrototype || c.Projects[1].PlanPrototype {
		t.Errorf("plan_prototype = %v %v, want true false", c.Projects[0].PlanPrototype, c.Projects[1].PlanPrototype)
	}
}

func TestBrowserMustExist(t *testing.T) {
	_, err := Load(write(t, valid+"agent:\n  browser: /nonexistent/chrome\n"))
	if err == nil || !strings.Contains(err.Error(), "agent.browser") {
		t.Fatalf("err = %v, want agent.browser reported", err)
	}
}
