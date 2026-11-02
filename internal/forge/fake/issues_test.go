package fake

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestCreateIssueNumbersAfterTheLastIssueAndRecordsWhoLabeledIt(t *testing.T) {
	f := New(nil)
	me := f.AddUser("me", 1)
	f.AddRepo("o", "r", true)
	f.AddIssue("o", "r", 3, "old", "", me)
	f.PostAs = "me"
	got, err := f.CreateIssue(context.Background(), "o", "r", "new", "body", []string{"agent:ready"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Ref.Number != 4 || got.Title != "new" || got.URL != "https://github.test/o/r/issues/4" {
		t.Fatalf("CreateIssue = %+v", got)
	}
	is, _, _ := f.Thread(context.Background(), got.Ref)
	if is.Body != "body" || is.Author != me || !slices.Equal(is.Labels, []string{"agent:ready"}) {
		t.Errorf("issue = %+v", is)
	}
	if by, err := f.LabelActor(context.Background(), got.Ref, "agent:ready"); err != nil || *by != me {
		t.Errorf("LabelActor = %v, %v; want me", by, err)
	}
}

func TestCreateIssueInAMissingRepository(t *testing.T) {
	if _, err := New(nil).CreateIssue(context.Background(), "o", "nope", "t", "", nil); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
