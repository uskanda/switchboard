package config

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestAllowToolsExtraIsOptional(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Agent.AllowToolsExtra) != 0 || len(c.Projects[0].AllowToolsExtra) != 0 {
		t.Errorf("extra allows = %v, %v; want none", c.Agent.AllowToolsExtra, c.Projects[0].AllowToolsExtra)
	}
}

func TestAllowToolsExtraIsRead(t *testing.T) {
	body := strings.Replace(valid, "    path: ~/repos/sandbox\n", "    path: ~/repos/sandbox\n    allow_tools_extra: [\"Bash(openspec *)\"]\n", 1)
	c, err := Load(write(t, body+"agent:\n  allow_tools_extra: [Skill, \"Bash(git *)\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(c.Agent.AllowToolsExtra, []string{"Skill", "Bash(git *)"}) {
		t.Errorf("agent extra allows = %v", c.Agent.AllowToolsExtra)
	}
	if !slices.Equal(c.Projects[0].AllowToolsExtra, []string{"Bash(openspec *)"}) {
		t.Errorf("project extra allows = %v", c.Projects[0].AllowToolsExtra)
	}
}

func TestAllowToolsExtraRefusesWhatIsTooBroadOrDenied(t *testing.T) {
	for _, rule := range []string{"", "  ", "Bash", "Bash(*)", "Bash(:*)", "Bash(git push)", "Bash(git push *)", "Bash(git push:*)",
		"Bash(git  rebase *)", "Bash(gh)", "Bash(gh pr create *)", "WebFetch", "WebFetch(domain:example.com)"} {
		_, err := Load(write(t, valid+"agent:\n  allow_tools_extra: [\""+rule+"\"]\n"))
		if err == nil || !strings.Contains(err.Error(), "agent.allow_tools_extra") {
			t.Errorf("rule %q: err = %v, want agent.allow_tools_extra reported", rule, err)
		}
	}
}

func TestAllowToolsExtraNamesTheProject(t *testing.T) {
	body := strings.Replace(valid, "    path: ~/repos/sandbox\n", "    path: ~/repos/sandbox\n    allow_tools_extra: [\"Bash(*)\"]\n", 1)
	_, err := Load(write(t, body))
	if err == nil || !strings.Contains(err.Error(), "projects[0].allow_tools_extra") || !strings.Contains(err.Error(), "Bash(*)") {
		t.Fatalf("err = %v, want projects[0].allow_tools_extra and Bash(*) reported", err)
	}
}

func TestAllowToolsExtraLetsNarrowRulesThrough(t *testing.T) {
	for _, rule := range []string{"Skill", "Bash(git *)", "Bash(openspec *)", "Bash(github-linguist *)", "Bash(ghq list)"} {
		if _, err := Load(write(t, valid+"agent:\n  allow_tools_extra: [\""+rule+"\"]\n")); err != nil {
			t.Errorf("rule %q: %v", rule, err)
		}
	}
}

func TestWatcherAppliesTheProjectExtraAllowsButNotTheAgents(t *testing.T) {
	w, p := newWatcher(t, valid)
	body := strings.Replace(valid, "    path: ~/repos/sandbox\n", "    path: ~/repos/sandbox\n    allow_tools_extra: [Skill]\n", 1)
	save(t, p, body, time.Now().Add(time.Minute))
	ch, err := w.Check()
	if err != nil || ch == nil {
		t.Fatalf("Check = %+v, %v; want a change", ch, err)
	}
	if !slices.Equal(ch.Projects[0].AllowToolsExtra, []string{"Skill"}) || len(ch.Ignored) != 0 {
		t.Errorf("projects[0] extra allows = %v, ignored = %v; want [Skill] applied", ch.Projects[0].AllowToolsExtra, ch.Ignored)
	}
	save(t, p, body+"agent:\n  allow_tools_extra: [Skill]\n", time.Now().Add(2*time.Minute))
	ch, err = w.Check()
	if err != nil || ch == nil || strings.Join(ch.Ignored, ",") != "agent" {
		t.Errorf("Check = %+v, %v; want agent left for a restart", ch, err)
	}
}
