package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpListsEverySubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"--help"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, name := range []string{"labels sync", "agent run", "agent status", "inbox serve", "inbox", "show", "reply", "doctor"} {
		if !strings.Contains(out.String(), name) {
			t.Errorf("help does not list %q:\n%s", name, out.String())
		}
	}
}

func TestNoArgumentsPrintsUsageAndFails(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(nil, &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "usage") {
		t.Errorf("stderr does not show usage: %q", errOut.String())
	}
}

func TestUnknownCommandFails(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"frobnicate"}, &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "frobnicate") {
		t.Errorf("stderr does not name the unknown command: %q", errOut.String())
	}
}

func TestIncompleteGroupFails(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run([]string{"agent"}, &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
