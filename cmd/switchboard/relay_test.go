package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/cli"
)

func TestRelayServeNeedsAHost(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"relay", "serve", "--config", writeConfig(t)}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "relay token add") {
		t.Fatalf("code %d, stderr %s", code, errb.String())
	}
}

type memKeyring map[string]string

func (m memKeyring) Set(ref, v string) error { m[ref] = v; return nil }
func (m memKeyring) Delete(ref string) error { delete(m, ref); return nil }

func TestRelayTokenAddPrintsTheHashOnly(t *testing.T) {
	store := memKeyring{}
	old := keyring
	keyring = store
	defer func() { keyring = old }()
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"relay", "token", "add", "ubuntu-a", "--config", writeConfig(t)}, &out, &errb); code != 0 {
		t.Fatalf("code %d: %s", code, errb.String())
	}
	tok := store["switchboard/relay-ubuntu-a"]
	if tok == "" || strings.Contains(out.String(), tok) || !strings.Contains(out.String(), "token_hash: sha256:") {
		t.Fatalf("stored %q, output:\n%s", tok, out.String())
	}
}

func TestRelayHookAddNeedsAnInboxToken(t *testing.T) {
	var out, errb bytes.Buffer
	cfg := writeConfig(t)
	code := cli.Run([]string{"relay", "hook", "add", "gh:o/r", "--config", cfg}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "inbox_token_ref") {
		t.Fatalf("code %d, stderr %s", code, errb.String())
	}
}
