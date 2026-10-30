package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/cli"
)

// --dry-run shows the unit and every sudo command, runs none, and shows no token.
func TestServiceInstallDryRun(t *testing.T) {
	creds := t.TempDir()
	if err := os.WriteFile(filepath.Join(creds, "t"), []byte("secret-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", creds)
	t.Setenv("PATH", t.TempDir()) // no sudo to find, should anything try to run it
	var out, errb bytes.Buffer
	code := cli.Run([]string{"service", "install", "--dry-run", "--config", writeConfig(t)}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"User=", "LoadCredentialEncrypted=t:/etc/credstore.encrypted/switchboard.t", "sudo systemd-creds encrypt --name=t -", "sudo systemctl restart switchboard-agent.service", "dry run: nothing was run"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "secret-token") {
		t.Fatalf("the token is shown:\n%s", s)
	}
}

// Without a terminal and without --yes, nothing runs.
func TestServiceInstallWithoutATerminalNeedsYes(t *testing.T) {
	creds := t.TempDir()
	if err := os.WriteFile(filepath.Join(creds, "t"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", creds)
	t.Setenv("PATH", t.TempDir())
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"service", "install", "--config", writeConfig(t)}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "--yes") {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
}

func TestServiceStatusRuns(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"service", "status", "--config", writeConfig(t)}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "enabled:") || !strings.Contains(out.String(), "active:") {
		t.Fatalf("status:\n%s", out.String())
	}
}
