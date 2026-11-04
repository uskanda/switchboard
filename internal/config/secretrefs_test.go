package config

import (
	"strings"
	"testing"
)

// Two token references that systemd would keep under one credential name are refused at load.
func TestTokenRefsThatShareACredentialNameAreRefused(t *testing.T) {
	yml := strings.Replace(valid, "inbox_token_ref: switchboard/github-inbox", "inbox_token_ref: switchboard_github", 1)
	_, err := Load(write(t, yml))
	if err == nil || !strings.Contains(err.Error(), "switchboard/github, switchboard_github") {
		t.Fatalf("err = %v", err)
	}
}
