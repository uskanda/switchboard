package workspace

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	in := "remote: https://x-access-token:ghs_abcdefghijklmnopqrstuvwxyz0123@github.com/o/r.git\n" +
		"token ghp_abcdefghijklmnopqrstuvwxyz0123 here\n" +
		"-----BEGIN OPENSSH PRIVATE KEY-----\nAAAAB3NzaC1yc2E\n-----END OPENSSH PRIVATE KEY-----\nafter\n"
	got := Redact(in)
	for _, leak := range []string{"ghs_abc", "ghp_abc", "x-access-token", "AAAAB3NzaC1yc2E"} {
		if strings.Contains(got, leak) {
			t.Errorf("Redact left %q in:\n%s", leak, got)
		}
	}
	if !strings.Contains(got, "github.com/o/r.git") || !strings.Contains(got, "after") || !strings.Contains(got, "[redacted]") {
		t.Errorf("Redact removed too much:\n%s", got)
	}
}
