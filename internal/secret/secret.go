// Package secret retrieves forge tokens: from the systemd credentials of the service when it runs as one, from the
// Secret Service, and from the environment only when the configuration allows it for development.
package secret

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

// ErrNotFound reports that the store has no item for a reference.
var ErrNotFound = errors.New("secret not found")

// Store looks up a token by its reference (token_ref in the configuration).
type Store interface {
	Get(ref string) (string, error)
}

// Keyring is the Secret Service store. An item has the attributes service=<ref> and username=token, which is what
// `secret-tool store --label=... service <ref> username token` creates.
type Keyring struct{}

// Get returns the token stored for ref.
func (Keyring) Get(ref string) (string, error) {
	v, err := keyring.Get(ref, "token")
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

// Resolver finds the token for a reference: the store first, then, when AllowEnv is set, the environment variable
// named by EnvName.
type Resolver struct {
	Store    Store
	AllowEnv bool
}

// Token returns the token for ref. A store failure other than a missing item is returned as is, so that a broken
// Secret Service is never silently replaced by the environment.
func (r Resolver) Token(ref string) (string, error) {
	if v, ok, err := credential(ref); ok || err != nil {
		return v, err
	}
	v, err := r.Store.Get(ref)
	if err == nil {
		return v, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("read token %s from the Secret Service: %w", ref, err)
	}
	if r.AllowEnv {
		if v := os.Getenv(EnvName(ref)); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("no token for %s in the Secret Service (store it with: secret-tool store --label='switchboard' service %s username token)", ref, ref)
}

// CredentialName is the name of the systemd credential that holds the token of ref: ref with every character that
// is not a letter or a digit replaced by an underscore, since a credential name cannot hold a slash.
func CredentialName(ref string) string {
	var b strings.Builder
	for _, r := range ref {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// credential reads the token of ref from $CREDENTIALS_DIRECTORY, which systemd sets for a service with credentials.
// ok is false when there is no such directory or no credential for ref.
func credential(ref string) (v string, ok bool, err error) {
	dir := os.Getenv("CREDENTIALS_DIRECTORY")
	if dir == "" {
		return "", false, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, CredentialName(ref)))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read the credential of %s: %w", ref, err)
	}
	return strings.TrimSpace(string(data)), true, nil
}

// EnvName is the development-only environment variable for ref: SWITCHBOARD_TOKEN_ followed by ref in upper case,
// with every character that is not a letter or a digit replaced by an underscore.
func EnvName(ref string) string {
	var b strings.Builder
	b.WriteString("SWITCHBOARD_TOKEN_")
	for _, r := range strings.ToUpper(ref) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
