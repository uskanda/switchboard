package forge

import "context"

// TokenScopes is what a token says it may do. Known is false when the forge does not say, as for a GitHub
// fine-grained token, whose permissions no API returns.
type TokenScopes struct {
	Known  bool
	Scopes []string
}

// Probe is one read a token was tried with: what was read, and the error, nil when it could.
type Probe struct {
	What string
	Err  error
}

// TokenProber is a forge client that can tell what its token may do, for switchboard doctor. It never writes.
type TokenProber interface {
	// TokenScopes returns the scopes of the client's token.
	TokenScopes(ctx context.Context) (*TokenScopes, error)
	// ProbeReads tries the reads the agent needs on owner/repo (its issues, pull requests and contents), each
	// without a side effect, and returns how each went.
	ProbeReads(ctx context.Context, owner, repo, branch string) []Probe
}
