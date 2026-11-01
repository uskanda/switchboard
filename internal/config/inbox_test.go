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
