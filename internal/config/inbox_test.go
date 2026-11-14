package config

import (
	"strings"
	"testing"
	"time"
)

func TestInboxRefreshIntervalDefaultsTo30s(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Inbox.RefreshInterval != 30*time.Second {
		t.Errorf("refresh interval = %v, want 30s", c.Inbox.RefreshInterval)
	}
}

func TestInboxRefreshIntervalIsRead(t *testing.T) {
	c, err := Load(write(t, valid+"inbox:\n  refresh_interval: 2m\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Inbox.RefreshInterval != 2*time.Minute {
		t.Errorf("refresh interval = %v, want 2m", c.Inbox.RefreshInterval)
	}
}

func TestInboxRefreshIntervalUnder10sIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"inbox:\n  refresh_interval: 5s\n"))
	if err == nil || !strings.Contains(err.Error(), "inbox.refresh_interval") || !strings.Contains(err.Error(), "10s") {
		t.Fatalf("err = %v, want inbox.refresh_interval refused below 10s", err)
	}
}

func TestInboxProjectRootExpandsHome(t *testing.T) {
	t.Setenv("HOME", "/home/alice")
	c, err := Load(write(t, valid+"inbox:\n  project_root: ~/repos\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Inbox.ProjectRoot != "/home/alice/repos" {
		t.Errorf("project root = %q, want /home/alice/repos", c.Inbox.ProjectRoot)
	}
	if !c.Inbox.ProjectAddEnabled() {
		t.Error("adding projects is disabled, want enabled")
	}
}

func TestInboxProjectRootMustBeAbsolute(t *testing.T) {
	_, err := Load(write(t, valid+"inbox:\n  project_root: repos\n"))
	if err == nil || !strings.Contains(err.Error(), "inbox.project_root") {
		t.Fatalf("err = %v, want inbox.project_root refused as relative", err)
	}
}

func TestInboxWithoutProjectRootDisablesAdding(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Inbox.ProjectAddEnabled() {
		t.Error("adding projects is enabled without inbox.project_root")
	}
}

func TestInboxRelayIsRead(t *testing.T) {
	c, err := Load(write(t, valid+"inbox:\n  relay:\n    url: http://127.0.0.1:8782/events\n    token_ref: switchboard/relay-ubuntu-a-inbox\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r := c.Inbox.Relay; r == nil || r.URL != "http://127.0.0.1:8782/events" || r.TokenRef != "switchboard/relay-ubuntu-a-inbox" {
		t.Fatalf("inbox.relay = %+v", c.Inbox.Relay)
	}
}

func TestInboxRelayNeedsURLAndTokenRef(t *testing.T) {
	for want, body := range map[string]string{
		"inbox.relay.url":       "inbox:\n  relay:\n    token_ref: switchboard/relay-ubuntu-a-inbox\n",
		"inbox.relay.token_ref": "inbox:\n  relay:\n    url: http://127.0.0.1:8782/events\n",
	} {
		if _, err := Load(write(t, valid+body)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want %s required", err, want)
		}
	}
}

func TestInboxReadModeDefaults(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Inbox.ReadMode != ReadAdaptive {
		t.Errorf("read mode = %q, want %q", c.Inbox.ReadMode, ReadAdaptive)
	}
	if c.Inbox.IdleInterval != 30*time.Minute {
		t.Errorf("idle interval = %v, want 30m", c.Inbox.IdleInterval)
	}
}

func TestInboxReadModeIsRead(t *testing.T) {
	c, err := Load(write(t, valid+"inbox:\n  read_mode: always\n  idle_interval: 10m\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Inbox.ReadMode != ReadAlways || c.Inbox.IdleInterval != 10*time.Minute {
		t.Errorf("read mode, idle interval = %q, %v, want always, 10m", c.Inbox.ReadMode, c.Inbox.IdleInterval)
	}
}

func TestInboxUnknownReadModeIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"inbox:\n  read_mode: lazy\n"))
	if err == nil || !strings.Contains(err.Error(), "inbox.read_mode") || !strings.Contains(err.Error(), "adaptive") || !strings.Contains(err.Error(), "always") {
		t.Fatalf("err = %v, want inbox.read_mode refused naming adaptive and always", err)
	}
}

func TestInboxIdleIntervalUnderRefreshIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"inbox:\n  refresh_interval: 1m\n  idle_interval: 30s\n"))
	if err == nil || !strings.Contains(err.Error(), "inbox.idle_interval") || !strings.Contains(err.Error(), "inbox.refresh_interval") {
		t.Fatalf("err = %v, want inbox.idle_interval refused below inbox.refresh_interval", err)
	}
}
