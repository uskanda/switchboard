package config

import (
	"strings"
	"testing"
	"time"
)

func TestResumeWithin(t *testing.T) {
	for yml, want := range map[string]time.Duration{
		"":                               time.Hour,
		"agent:\n  resume_within: 20m\n": 20 * time.Minute,
		"agent:\n  resume_within: 0s\n":  0,
	} {
		c, err := Load(write(t, valid+yml))
		if err != nil {
			t.Fatal(err)
		}
		if got := c.Agent.ResumeWithin(); got != want {
			t.Errorf("%q: ResumeWithin = %v, want %v", yml, got, want)
		}
	}
}

func TestNegativeResumeWithinIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"agent:\n  resume_within: -1m\n"))
	if err == nil || !strings.Contains(err.Error(), "agent.resume_within") {
		t.Fatalf("err = %v, want agent.resume_within reported", err)
	}
}
