package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/cli"
)

// inboxConfig is a GitHub-only configuration with an inbox token, in a directory of its own.
func inboxConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := `version: 1
host: {name: here}
forges:
  github: {token_ref: t, inbox_token_ref: ti, agent_identity: bot}
trusted_authors: {github: [me]}
projects:
  - {ref: "gh:o/r", path: /src/r, host: here}
`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestServiceInstallInboxDryRun(t *testing.T) {
	creds := t.TempDir()
	os.WriteFile(filepath.Join(creds, "ti"), []byte("inbox-token"), 0o600)
	t.Setenv("CREDENTIALS_DIRECTORY", creds)
	t.Setenv("PATH", t.TempDir())
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"service", "install", "--inbox", "--dry-run", "--config", inboxConfig(t)}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"inbox serve --config", "RuntimeDirectory=switchboard", "LoadCredentialEncrypted=ti:", "sudo systemctl restart switchboard-inbox.service"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "LoadCredentialEncrypted=t:") || strings.Contains(s, "inbox-token") {
		t.Errorf("the agent token or a token value is in the output:\n%s", s)
	}
}

func TestServiceInstallWebFindsSwitchboardWeb(t *testing.T) {
	bin := t.TempDir()
	web := filepath.Join(bin, "switchboard-web")
	os.WriteFile(web, []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", bin)
	cfgPath := inboxConfig(t)
	os.WriteFile(filepath.Join(filepath.Dir(cfgPath), "web.yaml"), []byte("listen: 127.0.0.1:8783\n"), 0o600)
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"service", "install", "--web", "--dry-run", "--config", cfgPath}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	s := out.String()
	want := "ExecStart=" + web + " serve --config " + filepath.Join(filepath.Dir(cfgPath), "web.yaml") + "\n"
	if !strings.Contains(s, want) || !strings.Contains(s, "sudo systemctl restart switchboard-web.service") {
		t.Fatalf("output lacks %q:\n%s", want, s)
	}
	if strings.Contains(s, "LoadCredentialEncrypted") || strings.Contains(s, "systemd-creds") {
		t.Fatalf("the web unit gets credentials:\n%s", s)
	}
}

func TestServiceInstallWebWithoutSwitchboardWeb(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out, errb bytes.Buffer
	code := cli.Run([]string{"service", "install", "--web", "--dry-run", "--config", inboxConfig(t)}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "switchboard-web") || strings.Contains(out.String(), "sudo") {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, out.String(), errb.String())
	}
}

func TestServiceKindsAreExclusive(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"service", "install", "--web", "--inbox", "--dry-run", "--config", inboxConfig(t)}, &out, &errb); code != 2 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
}

func TestServiceUninstallWebWithoutSwitchboardWeb(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"service", "uninstall", "--web", "--dry-run", "--config", inboxConfig(t)}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "sudo systemctl disable --now switchboard-web.service") {
		t.Fatalf("output:\n%s", out.String())
	}
}
