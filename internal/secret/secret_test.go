package secret

import (
	"errors"
	"strings"
	"testing"
)

type fakeStore map[string]string

func (f fakeStore) Get(ref string) (string, error) {
	if v, ok := f[ref]; ok {
		return v, nil
	}
	return "", ErrNotFound
}

func TestEnvName(t *testing.T) {
	if got := EnvName("switchboard/github"); got != "SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB" {
		t.Errorf("EnvName = %q", got)
	}
	if got := EnvName("switchboard/github-inbox"); got != "SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB_INBOX" {
		t.Errorf("EnvName = %q", got)
	}
}

func TestTokenFromStore(t *testing.T) {
	r := Resolver{Store: fakeStore{"switchboard/github": "s3cret"}}
	got, err := r.Token("switchboard/github")
	if err != nil || got != "s3cret" {
		t.Fatalf("Token = %q, %v", got, err)
	}
}

func TestTokenFromEnvWhenAllowed(t *testing.T) {
	t.Setenv("SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB", "from-env")
	r := Resolver{Store: fakeStore{}, AllowEnv: true}
	got, err := r.Token("switchboard/github")
	if err != nil || got != "from-env" {
		t.Fatalf("Token = %q, %v", got, err)
	}
}

func TestEnvIgnoredWhenNotAllowed(t *testing.T) {
	t.Setenv("SWITCHBOARD_TOKEN_SWITCHBOARD_GITHUB", "from-env")
	r := Resolver{Store: fakeStore{}}
	_, err := r.Token("switchboard/github")
	if err == nil || !strings.Contains(err.Error(), "switchboard/github") {
		t.Fatalf("err = %v, want a missing token naming the ref", err)
	}
}

func TestStoreErrorIsNotHiddenByEnv(t *testing.T) {
	boom := errors.New("dbus down")
	r := Resolver{Store: errStore{boom}, AllowEnv: true}
	t.Setenv("SWITCHBOARD_TOKEN_X", "env")
	if _, err := r.Token("x"); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the store error", err)
	}
}

type errStore struct{ err error }

func (e errStore) Get(string) (string, error) { return "", e.err }
