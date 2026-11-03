package fake

import (
	"context"
	"errors"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestMergeChangeRequest(t *testing.T) {
	f := New(nil)
	ctx := context.Background()
	if err := f.MergeChangeRequest(ctx, "o", "r", 5, "abc"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("missing: err = %v", err)
	}
	f.SetChange("o", "r", 5, Change{Draft: true, Head: "abc"})
	if err := f.MergeChangeRequest(ctx, "o", "r", 5, "old"); !errors.Is(err, forge.ErrHeadMoved) {
		t.Fatalf("moved: err = %v", err)
	}
	if err := f.MergeChangeRequest(ctx, "o", "r", 5, "abc"); err != nil {
		t.Fatal(err)
	}
	if c, _ := f.ChangeState("o", "r", 5); c.Draft || !c.Merged {
		t.Fatalf("after merge: %+v", c)
	}
	if err := f.MergeChangeRequest(ctx, "o", "r", 5, "abc"); !errors.Is(err, forge.ErrNotMergeable) {
		t.Fatalf("merged twice: err = %v", err)
	}
	f.SetChange("o", "r", 6, Change{Head: "abc", Refuse: "checks failed"})
	var nm *forge.NotMergeableError
	if err := f.MergeChangeRequest(ctx, "o", "r", 6, "abc"); !errors.As(err, &nm) || nm.Reason != "checks failed" {
		t.Fatalf("refused: err = %v", err)
	}
}

// A change request recorded with SetChange is also what ChangeRequestHead reads, so that the inbox service sees the
// same state through both; a conflicting one is refused.
func TestChangeRequestHeadOfASetChange(t *testing.T) {
	f := New(nil)
	ctx := context.Background()
	var m forge.Merges = f
	f.SetChange("o", "r", 5, Change{Head: "abc", Conflict: true})
	st, err := m.ChangeRequestHead(ctx, "o", "r", 5)
	if err != nil || st.State != forge.CROpen || st.Head != "abc" || !st.Conflict {
		t.Fatalf("open: %+v, %v", st, err)
	}
	if err := f.MergeChangeRequest(ctx, "o", "r", 5, "abc"); !errors.Is(err, forge.ErrNotMergeable) {
		t.Fatalf("a conflict was merged: %v", err)
	}
	f.SetChange("o", "r", 6, Change{Head: "abc", Closed: true})
	if st, _ := m.ChangeRequestHead(ctx, "o", "r", 6); st.State != forge.CRClosed {
		t.Fatalf("closed: %+v", st)
	}
	if err := f.MergeChangeRequest(ctx, "o", "r", 6, "abc"); !errors.Is(err, forge.ErrNotMergeable) {
		t.Fatalf("a closed change was merged: %v", err)
	}
	f.SetChange("o", "r", 7, Change{Head: "abc"})
	f.MergeChangeRequest(ctx, "o", "r", 7, "abc")
	if st, _ := m.ChangeRequestHead(ctx, "o", "r", 7); st.State != forge.CRMerged {
		t.Fatalf("merged: %+v", st)
	}
}
