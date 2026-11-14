package config

import (
	"strings"
	"testing"
	"time"
)

// webFetchOf is the WebFetch setting of each project of c, in order.
func webFetchOf(c *Config) []bool {
	var out []bool
	for _, p := range c.Projects {
		out = append(out, c.Agent.PlanWebFetchFor(p))
	}
	return out
}

func TestPlanWebFetchIsOnByDefault(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	for i, on := range webFetchOf(c) {
		if !on {
			t.Errorf("%s: plan_web_fetch defaults to off", c.Projects[i].Ref)
		}
	}
}

func TestPlanWebFetchOffForEveryProject(t *testing.T) {
	c, err := Load(write(t, valid+"agent:\n  plan_web_fetch: false\n"))
	if err != nil {
		t.Fatal(err)
	}
	for i, on := range webFetchOf(c) {
		if on {
			t.Errorf("%s: plan_web_fetch is on under agent.plan_web_fetch: false", c.Projects[i].Ref)
		}
	}
}

func TestPlanWebFetchOfAProjectOverridesTheAgent(t *testing.T) {
	body := strings.Replace(valid, "    host: ubuntu-a\n", "    host: ubuntu-a\n    plan_web_fetch: true\n", 1) +
		"agent:\n  plan_web_fetch: false\n"
	c, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if got := webFetchOf(c); len(got) != 2 || !got[0] || got[1] {
		t.Errorf("plan_web_fetch = %v, want [true false]", got)
	}
}

func TestWatcherAppliesThePlanWebFetchOfAProject(t *testing.T) {
	w, p := newWatcher(t, valid)
	body := strings.Replace(valid, "    host: ubuntu-a\n", "    host: ubuntu-a\n    plan_web_fetch: false\n", 1)
	save(t, p, body, time.Now().Add(time.Minute))
	ch, err := w.Check()
	if err != nil || ch == nil {
		t.Fatalf("Check = %+v, %v; want the new projects", ch, err)
	}
	var a Agent
	if a.PlanWebFetchFor(ch.Projects[0]) || !a.PlanWebFetchFor(ch.Projects[1]) {
		t.Errorf("plan_web_fetch of the reloaded projects = %v %v, want false true",
			a.PlanWebFetchFor(ch.Projects[0]), a.PlanWebFetchFor(ch.Projects[1]))
	}
	if len(ch.Ignored) != 0 {
		t.Errorf("ignored = %v, want none", ch.Ignored)
	}
}
