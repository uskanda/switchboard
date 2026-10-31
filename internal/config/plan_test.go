package config

import (
	"strings"
	"testing"
)

func TestPlanSettingsDefault(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Plan.ApproveWord != "/ok" {
		t.Errorf("approve word = %q, want /ok", c.Plan.ApproveWord)
	}
	if c.Labels.Plan != "agent:plan" || c.Labels.Go != "agent:go" {
		t.Errorf("start labels = %q %q, want agent:plan agent:go", c.Labels.Plan, c.Labels.Go)
	}
	for _, p := range c.Projects {
		if p.PlanFirst {
			t.Errorf("%s: plan_first defaults to true", p.Ref)
		}
	}
}

func TestPlanSettingsAreRead(t *testing.T) {
	body := strings.Replace(valid, "    host: ubuntu-a\n", "    host: ubuntu-a\n    plan_first: true\n", 1) +
		"plan:\n  approve_word: /lgtm\nlabels:\n  plan: \"switchboard:plan\"\n  go: \"switchboard:go\"\n"
	c, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Projects[0].PlanFirst || c.Projects[1].PlanFirst {
		t.Errorf("plan_first = %v %v, want true false", c.Projects[0].PlanFirst, c.Projects[1].PlanFirst)
	}
	if c.Plan.ApproveWord != "/lgtm" || c.Labels.Plan != "switchboard:plan" || c.Labels.Go != "switchboard:go" {
		t.Errorf("plan settings = %+v %+v", c.Plan, c.Labels)
	}
}

func TestApproveWordWithSpaceIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"plan:\n  approve_word: ok please\n"))
	if err == nil || !strings.Contains(err.Error(), "plan.approve_word") {
		t.Fatalf("err = %v, want it to name plan.approve_word", err)
	}
}

func TestBlankApproveWordIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"plan:\n  approve_word: \"  \"\n"))
	if err == nil || !strings.Contains(err.Error(), "plan.approve_word") {
		t.Fatalf("err = %v, want it to name plan.approve_word", err)
	}
}

func TestEmptyApproveWordIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"plan:\n  approve_word: \"\"\n"))
	if err == nil || !strings.Contains(err.Error(), "plan.approve_word") {
		t.Fatalf("err = %v, want it to name plan.approve_word", err)
	}
}

func TestUnknownPlanKeyIsNamed(t *testing.T) {
	_, err := Load(write(t, valid+"plan:\n  aprove_word: /ok\n"))
	if err == nil || !strings.Contains(err.Error(), "aprove_word") {
		t.Fatalf("err = %v, want it to name aprove_word", err)
	}
}
