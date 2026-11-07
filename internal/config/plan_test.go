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
		if p.PlanFirst.Mode() != PlanNever {
			t.Errorf("%s: plan_first defaults to %q, want never", p.Ref, p.PlanFirst.Mode())
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
	if c.Projects[0].PlanFirst.Mode() != PlanAlways || c.Projects[1].PlanFirst.Mode() != PlanNever {
		t.Errorf("plan_first = %q %q, want always never", c.Projects[0].PlanFirst, c.Projects[1].PlanFirst)
	}
	if c.Plan.ApproveWord != "/lgtm" || c.Labels.Plan != "switchboard:plan" || c.Labels.Go != "switchboard:go" {
		t.Errorf("plan settings = %+v %+v", c.Plan, c.Labels)
	}
}

func TestPlanFirstTakesThreeValuesAndTheOldBooleans(t *testing.T) {
	for _, c := range []struct {
		written string
		want    PlanFirst
	}{
		{"always", PlanAlways}, {"auto", PlanAuto}, {"never", PlanNever},
		{"true", PlanAlways}, {"false", PlanNever}, {"True", PlanAlways}, {"\"auto\"", PlanAuto},
	} {
		body := strings.Replace(valid, "    host: ubuntu-a\n", "    host: ubuntu-a\n    plan_first: "+c.written+"\n", 1)
		cfg, err := Load(write(t, body))
		if err != nil {
			t.Errorf("plan_first: %s: %v", c.written, err)
			continue
		}
		if got := cfg.Projects[0].PlanFirst.Mode(); got != c.want {
			t.Errorf("plan_first: %s reads as %q, want %q", c.written, got, c.want)
		}
	}
}

func TestPlanFirstRefusesOtherValues(t *testing.T) {
	for _, written := range []string{"sometimes", "\"true\"", "yes", "1", "[auto]", "\"\""} {
		body := strings.Replace(valid, "    host: ubuntu-a\n", "    host: ubuntu-a\n    plan_first: "+written+"\n", 1)
		_, err := Load(write(t, body))
		if err == nil || !strings.Contains(err.Error(), "plan_first") {
			t.Errorf("plan_first: %s: err = %v, want it to name plan_first", written, err)
		}
	}
}

func TestPlanFirstPlans(t *testing.T) {
	for _, c := range []struct {
		p    PlanFirst
		want bool
	}{{PlanAlways, true}, {PlanAuto, true}, {PlanNever, false}, {"", false}} {
		if got := c.p.Plans(); got != c.want {
			t.Errorf("%q.Plans() = %v, want %v", c.p, got, c.want)
		}
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
