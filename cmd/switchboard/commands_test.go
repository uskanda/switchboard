package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/cli"
)

func writeConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := `version: 1
host: {name: here}
forges:
  github: {token_ref: t, agent_identity: bot}
trusted_authors: {github: [me]}
projects:
  - {ref: "gh:o/r", path: /src/r, host: here}
`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAgentStatusWithoutAnAgent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var out, errb bytes.Buffer
	code := cli.Run([]string{"agent", "status", "--config", writeConfig(t)}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	for _, want := range []string{"host: here", "agent: not running", "gh:o/r"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("status lacks %q:\n%s", want, out.String())
		}
	}
}

func TestAgentRunWithoutATokenFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent")
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"agent", "run", "--config", writeConfig(t)}, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "token") {
		t.Fatalf("stderr does not explain the missing token: %s", errb.String())
	}
}

func TestMissingConfigFails(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"agent", "status", "--config", "/nonexistent.yaml"}, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}
