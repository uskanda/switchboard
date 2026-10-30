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
