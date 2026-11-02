package workspace

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifyPush(t *testing.T) {
	for out, want := range map[string]string{
		" ! [rejected]        switchboard/issue-12 -> switchboard/issue-12 (fetch first)\nerror: failed to push some refs":                                       PushNonFastForward,
		" ! [rejected]        switchboard/issue-12 -> switchboard/issue-12 (non-fast-forward)":                                                                   PushNonFastForward,
		"remote: error: GH006: Protected branch update failed for refs/heads/switchboard/issue-12.\n ! [remote rejected] (protected branch hook declined)":       PushRejected,
		"remote: GitLab: Commit message does not follow the pattern\n ! [remote rejected] (pre-receive hook declined)":                                           PushRejected,
		"remote: Permission to o/r.git denied to switchboard-bot.\nfatal: unable to access 'https://github.com/o/r.git/': The requested URL returned error: 403": PushAuth,
		"remote: HTTP Basic: Access denied\nfatal: Authentication failed for 'https://gl.example/nas/x.git/'":                                                    PushAuth,
		"fatal: unable to access 'https://github.com/o/r.git/': Could not resolve host: github.com":                                                              PushTransient,
		"error: RPC failed; HTTP 502 curl 22 The requested URL returned error: 502\nfatal: the remote end hung up unexpectedly":                                  PushTransient,
		"fatal: unable to access 'https://github.com/o/r.git/': Failed to connect to github.com port 443: Connection timed out":                                  PushTransient,
		"something nobody has seen before": PushRejected,
	} {
		if got := ClassifyPush(out); got != want {
			t.Errorf("ClassifyPush(%q) = %s, want %s", out, got, want)
		}
	}
}

func TestPushNonFastForwardAndFetch(t *testing.T) {
	s, origin := space(t)
	dir, _ := s.Prepare(ctx)
	commit(t, dir, "a.go", "package a\n")
	if err := s.Push(ctx); err != nil {
		t.Fatal(err)
	}
	// Someone else adds a commit to the issue branch on origin.
	other := t.TempDir()
	git(t, other, "clone", "-q", "-b", "switchboard/issue-12", origin, "o")
	commit(t, other+"/o", "b.go", "package b\n")
	git(t, other+"/o", "push", "-q", "origin", "switchboard/issue-12")
	remote := git(t, other+"/o", "rev-parse", "HEAD")

	commit(t, dir, "c.go", "package c\n")
	err := s.Push(ctx)
	var pe *PushError
	if !errors.As(err, &pe) || pe.Kind != PushNonFastForward || !strings.Contains(pe.Output, "rejected") {
		t.Fatalf("Push = %v, want a non-fast-forward PushError", err)
	}
	if err := s.FetchIssueBranch(ctx); err != nil {
		t.Fatal(err)
	}
	if got := git(t, dir, "rev-parse", "origin/switchboard/issue-12"); got != remote {
		t.Fatalf("origin/switchboard/issue-12 = %s, want %s", got, remote)
	}
}
