package config

import (
	"strings"
	"testing"
)

func TestRunModelIsOptional(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []RunKind{ImplementRun, PlanRun, FixRun} {
		if m := c.Agent.ModelFor(kind); m != (RunModel{}) {
			t.Errorf("model for %v = %+v, want none", kind, m)
		}
	}
}

func TestRunModelOverridesTheDefaultPerKind(t *testing.T) {
	c, err := Load(write(t, valid+"agent:\n  model: opus\n  effort: high\n  plan:\n    model: sonnet\n  fix:\n    effort: medium\n"))
	if err != nil {
		t.Fatal(err)
	}
	for kind, want := range map[RunKind]RunModel{
		ImplementRun: {Model: "opus", Effort: "high"},
		PlanRun:      {Model: "sonnet", Effort: "high"},
		FixRun:       {Model: "opus", Effort: "medium"},
	} {
		if got := c.Agent.ModelFor(kind); got != want {
			t.Errorf("model for %v = %+v, want %+v", kind, got, want)
		}
	}
}

func TestUnknownEffortFails(t *testing.T) {
	for _, body := range []string{"agent:\n  effort: ultra\n", "agent:\n  plan:\n    effort: ultra\n", "agent:\n  fix:\n    effort: ultra\n"} {
		_, err := Load(write(t, valid+body))
		if err == nil || !strings.Contains(err.Error(), `"ultra"`) || !strings.Contains(err.Error(), "effort") {
			t.Errorf("%q: err = %v, want an error on the effort ultra", body, err)
		}
	}
}

func TestModelNameIsNotChecked(t *testing.T) {
	if _, err := Load(write(t, valid+"agent:\n  model: my-model\n")); err != nil {
		t.Fatalf("err = %v, want none: the CLI checks model names", err)
	}
}
