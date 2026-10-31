package fake

import (
	"context"
	"testing"
	"time"
)

func TestFakeRecordsLabelActorAndUpdates(t *testing.T) {
	f := New(nil)
	alice := f.AddUser("alice", 1)
	f.AddRepo("alice", "sandbox", true)
	ref := f.AddIssue("alice", "sandbox", 1, "t", "b", alice)
	since := time.Now().Add(-time.Minute)
	f.AddLabel(ref, "agent:ready", alice)

	got, err := f.LabelActor(context.Background(), ref, "agent:ready")
	if err != nil || got.ID != 1 {
		t.Fatalf("LabelActor = %+v, %v", got, err)
	}
	list, _ := f.UpdatedIssues(context.Background(), "alice", "sandbox", since)
	if len(list) != 1 || list[0].Labels[0] != "agent:ready" {
		t.Fatalf("UpdatedIssues = %+v", list)
	}
}

func TestFakeIssueURL(t *testing.T) {
	gh := New(nil)
	alice := gh.AddUser("alice", 1)
	gh.AddRepo("alice", "sandbox", true)
	is, _, err := gh.Thread(context.Background(), gh.AddIssue("alice", "sandbox", 3, "t", "b", alice))
	if err != nil || is.URL != "https://github.test/alice/sandbox/issues/3" {
		t.Fatalf("GitHub URL = %q, %v", is.URL, err)
	}
	gl := NewGitLab(nil)
	bob := gl.AddUser("bob", 2)
	gl.AddProject("nas", "sb", "private")
	is, _, err = gl.Thread(context.Background(), gl.AddIssue("nas", "sb", 4, "t", "b", bob))
	if err != nil || is.URL != "https://gitlab.test/nas/sb/-/issues/4" {
		t.Fatalf("GitLab URL = %q, %v", is.URL, err)
	}
}
