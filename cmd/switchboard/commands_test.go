package main

import (
	"bytes"
	"flag"
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

func TestOptionsMayFollowPositionalArguments(t *testing.T) {
	cases := []struct {
		args  []string
		force bool
		pos   string
	}{
		{[]string{"1", "Go with B.", "--force"}, true, "1|Go with B."},
		{[]string{"--force", "1", "text"}, true, "1|text"},
		{[]string{"1", "--", "--force"}, false, "1|--force"},
		{[]string{"gh:o/r#2", "x"}, false, "gh:o/r#2|x"},
	}
	for _, c := range cases {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		force := fs.Bool("force", false, "")
		if err := parseInterspersed(fs, c.args); err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		if *force != c.force || strings.Join(fs.Args(), "|") != c.pos {
			t.Errorf("%v: force=%v args=%q", c.args, *force, fs.Args())
		}
	}
}

func TestInboxWithoutAnInboxTokenRefFails(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"inbox", "--config", writeConfig(t)}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "inbox_token_ref") {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
}

func TestDoctorReportsAMissingToken(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent")
	var out, errb bytes.Buffer
	code := cli.Run([]string{"doctor", "--config", writeConfig(t)}, &out, &errb)
	if code != 1 || !strings.Contains(out.String(), "fail  agent token") || !strings.Contains(errb.String(), "problem") {
		t.Fatalf("exit %d\n%s\n%s", code, out.String(), errb.String())
	}
}
