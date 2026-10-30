package secret

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCredentialName(t *testing.T) {
	if got := CredentialName("switchboard/gitlab-inbox"); got != "switchboard_gitlab_inbox" {
		t.Fatalf("CredentialName = %q", got)
	}
}

// failing is a store that must not be asked.
type failing struct{ t *testing.T }

func (f failing) Get(ref string) (string, error) {
	f.t.Errorf("the Secret Service was asked for %s", ref)
	return "", errors.New("asked")
}

// Under systemd, a credential for the ref is read, and the Secret Service is never asked.
func TestTokenFromSystemdCredential(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "switchboard_github"), []byte("tok-from-creds\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", dir)
	v, err := Resolver{Store: failing{t}}.Token("switchboard/github")
	if err != nil || v != "tok-from-creds" {
		t.Fatalf("Token = %q, %v", v, err)
	}
}

// Without a credential for the ref, the Secret Service is read as before.
func TestMissingCredentialFallsBackToTheStore(t *testing.T) {
	t.Setenv("CREDENTIALS_DIRECTORY", t.TempDir())
	v, err := Resolver{Store: fakeStore{"switchboard/github": "from-store"}}.Token("switchboard/github")
	if err != nil || v != "from-store" {
		t.Fatalf("Token = %q, %v", v, err)
	}
}
