package fake

import (
	"context"
	"errors"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

var _ forge.Hooks = (*Hooks)(nil)

func TestHooks(t *testing.T) {
	h := &Hooks{}
	ctx := context.Background()
	if _, err := h.CreateHook(ctx, "o", "r", forge.Hook{URL: "u1", Secret: "s"}); err != nil {
		t.Fatal(err)
	}
	if got := h.Of("o", "r"); len(got) != 1 || got[0].URL != "u1" {
		t.Fatalf("hooks = %v", got)
	}
	if found, err := h.DeleteHook(ctx, "o", "r", "u1"); !found || err != nil || len(h.Of("o", "r")) != 0 {
		t.Fatalf("DeleteHook = %v, %v", found, err)
	}
	h.Err = forge.ErrForbidden
	if _, err := h.CreateHook(ctx, "o", "r", forge.Hook{}); !errors.Is(err, forge.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
}
