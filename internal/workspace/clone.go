package workspace

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Clone clones url into dir, which must not exist, offering token to git through GIT_ASKPASS as Push does; the
// token never appears in an argument. A failed clone leaves no directory behind, so that a later try does not take
// it for someone else's clone.
func Clone(ctx context.Context, env []string, url, dir, token, tokenUser string) error {
	s := &Space{Env: env, Token: token, TokenUser: tokenUser}
	auth, cleanup, err := s.authEnv()
	if err != nil {
		return err
	}
	defer cleanup()
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	if _, err := s.git(ctx, parent, auth, "clone", "-q", url, dir); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	return nil
}

// Origin returns the URL of the origin remote of the clone at dir.
func Origin(ctx context.Context, env []string, dir string) (string, error) {
	s := &Space{Env: env}
	return s.git(ctx, dir, nil, "remote", "get-url", "origin")
}

// SameRepo reports whether two remote URLs name the same repository: the same host and path, ignoring the scheme
// (https or ssh), a user name, a trailing .git and letter case.
func SameRepo(a, b string) bool {
	return strings.EqualFold(remoteKey(a), remoteKey(b))
}

func remoteKey(remote string) string {
	r := strings.TrimSuffix(strings.TrimRight(remote, "/"), ".git")
	if u, err := url.Parse(r); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Hostname() + "/" + strings.Trim(u.Path, "/")
	}
	// scp-like: [user@]host:path
	if at, rest, ok := strings.Cut(r, ":"); ok && !strings.Contains(at, "/") {
		if _, host, ok := strings.Cut(at, "@"); ok {
			at = host
		}
		return at + "/" + strings.Trim(rest, "/")
	}
	return filepath.Clean(r)
}
