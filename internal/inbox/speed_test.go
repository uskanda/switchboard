package inbox

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

// countingForge counts the lookups of users, to tell that the agent's account is resolved once.
type countingForge struct {
	forge.Forge
	users atomic.Int32
}

func (c *countingForge) User(ctx context.Context, login string) (*forge.User, error) {
	c.users.Add(1)
	return c.Forge.User(ctx, login)
}

func TestTheAgentAccountIsResolvedOnce(t *testing.T) {
	w := newWorld(t)
	cf := &countingForge{Forge: w.f}
	w.b.Forge = cf
	for range 3 {
		if _, err := w.b.Detail(ctx, w.a); err != nil {
			t.Fatal(err)
		}
	}
	if _, errs := w.b.Collect(ctx); len(errs) > 0 {
		t.Fatal(errs)
	}
	if n := cf.users.Load(); n != 1 {
		t.Fatalf("the agent's account was looked up %d times, want 1", n)
	}
}

func TestAnUnresolvedAgentAccountIsAskedAgain(t *testing.T) {
	w := newWorld(t)
	w.b.Config.Forges.GitHub.AgentIdentity = "nobody"
	cf := &countingForge{Forge: w.f}
	w.b.Forge = cf
	for range 2 {
		if _, err := w.b.Detail(ctx, w.a); err == nil {
			t.Fatal("Detail resolved an unknown account")
		}
	}
	if n := cf.users.Load(); n != 2 {
		t.Fatalf("an unknown account was looked up %d times, want 2", n)
	}
}
