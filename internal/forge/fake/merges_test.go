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
