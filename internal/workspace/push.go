package workspace

import (
	"fmt"
	"regexp"
)

// The kinds of a failed push.
const (
	PushTransient      = "transient"        // the network or the forge failed for a moment; trying again may work
	PushNonFastForward = "non-fast-forward" // origin's issue branch has commits the worktree does not
	PushAuth           = "auth"             // the token was refused or may not write
	PushRejected       = "rejected"         // a rule of the remote refused it: branch protection, a hook, a size limit
)

// PushError is a push git refused, with git's output (which never holds the token: it comes through GIT_ASKPASS).
type PushError struct {
	Kind   string
	Output string
	err    error
}

func (e *PushError) Error() string { return fmt.Sprintf("git push: %v: %s", e.err, e.Output) }

func (e *PushError) Unwrap() error { return e.err }

// The outputs of git and the forges a kind is known by, tried in this order. What none matches counts as rejected,
// which leaves it to a person.
var pushKinds = []struct {
	kind string
	re   *regexp.Regexp
}{
	{PushRejected, regexp.MustCompile(`\[remote rejected\]|pre-receive hook declined|GH0\d\d:`)},
	{PushNonFastForward, regexp.MustCompile(`\[rejected\].*\((?:fetch first|non-fast-forward)\)`)},
	{PushAuth, regexp.MustCompile(`(?i)authentication failed|access denied|permission to \S+ denied|returned error: 40[13]\b|invalid username or password`)},
	{PushTransient, regexp.MustCompile(`(?i)could not resolve host|connection timed out|connection reset|connection refused|operation timed out|rpc failed|returned error: 5\d\d\b|early eof|the remote end hung up unexpectedly|tls|ssl_`)},
}

// ClassifyPush names the kind of a failed push from git's output.
func ClassifyPush(output string) string {
	for _, k := range pushKinds {
		if k.re.MatchString(output) {
			return k.kind
		}
	}
	return PushRejected
}
