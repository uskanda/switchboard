package agent

import (
	"errors"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

var errBadGateway = errors.New("502 Bad Gateway")

// An issue whose turn comes while draining the queue is evaluated again when that fails once, even after a later
// update of another issue moved since past it (#27).
func TestDrainFailureIsEvaluatedAgain(t *testing.T) {
	x := newH(t, "", nil)
	second := x.f.AddIssue("o", "r", 13, "Another", "Do another thing.", x.master)
	third := x.f.AddIssue("o", "r", 14, "Unrelated", "Nothing to do.", x.master)
	x.step(1, block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.AddLabel(second, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)

	x.advance(2 * time.Minute)
	x.f.AddComment(third, "an unrelated update", x.master)
	x.poll()
	x.f.FailNthThreadOf(second, 1, errBadGateway)
	x.release(1)
	x.settle()
	if x.labels(second) != "agent:ready" {
		t.Fatalf("the failing drain ran the second issue anyway: %q", x.labels(second))
	}

	x.poll()
	x.settle()
	if x.starts() != 2 || x.labels(second) != "agent:waiting" {
		t.Fatalf("the second issue was lost: starts=%d labels=%q", x.starts(), x.labels(second))
	}
}

// An issue whose evaluation fails in a poll is evaluated again although a later issue of the same poll succeeded.
func TestPollFailureIsEvaluatedAgain(t *testing.T) {
	x := newH(t, "", nil)
	later := x.f.AddIssue("o", "r", 13, "Another", "Nothing to do.", x.master)
	x.poll()

	x.advance(time.Minute)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.advance(2 * time.Minute)
	x.f.AddComment(later, "a later update", x.master)
	x.f.FailNthThreadOf(x.ref, 1, errBadGateway)
	if err := x.pollErr(); err == nil {
		t.Fatal("the poll reported no error")
	}

	x.poll()
	x.settle()
	if x.starts() != 1 || x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("the failed issue was lost: starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
}

// A locked issue is evaluated again once its lock is free, even after a later update moved since past it.
func TestLockedIssueIsEvaluatedAgainAfterLaterUpdates(t *testing.T) {
	x := newH(t, "", nil)
	later := x.f.AddIssue("o", "r", 13, "Another", "Nothing to do.", x.master)
	x.poll()
	unlock, err := x.dir.Lock(lockName(x.ref))
	if err != nil {
		t.Fatal(err)
	}
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()

	x.advance(2 * time.Minute)
	x.f.AddComment(later, "a later update", x.master)
	x.poll()
	unlock()
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("the locked issue was lost: starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
}

// An issue that the forge no longer has is not evaluated again, and since moves on.
func TestMissingIssueStopsHoldingSince(t *testing.T) {
	for _, tc := range []struct {
		name string
		gone func(x *h)
	}{
		{"not found while evaluated", func(x *h) { x.f.FailNthThreadOf(x.ref, 1, forge.ErrNotFound) }},
		{"no longer listed", func(x *h) { x.f.DeleteIssue(x.ref) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := newH(t, "", nil)
			later := x.f.AddIssue("o", "r", 13, "Another", "Nothing to do.", x.master)
			x.poll()
			x.advance(time.Minute)
			x.f.AddLabel(x.ref, "agent:ready", x.master)
			x.f.FailNthThreadOf(x.ref, 1, errBadGateway)
			_ = x.pollErr()
			if !x.unsettled(x.ref) {
				t.Fatal("the failed issue is not kept for another evaluation")
			}

			x.advance(2 * time.Minute)
			x.f.AddComment(later, "a later update", x.master)
			tc.gone(x)
			x.poll()
			if x.unsettled(x.ref) {
				t.Fatal("an issue the forge does not have is still evaluated again")
			}
			p := x.cfg.Projects[0]
			if got, want := x.a.since(p), x.f.UpdatedAt(later).Add(-time.Minute); !got.Equal(want) {
				t.Fatalf("since = %s, want %s", got, want)
			}
		})
	}
}

// The issues of a project removed from the configuration are no longer evaluated again.
func TestRemovedProjectDropsItsReevaluations(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthThreadOf(x.ref, 1, errBadGateway)
	_ = x.pollErr()
	if !x.unsettled(x.ref) {
		t.Fatal("the failed issue is not kept for another evaluation")
	}
	x.a.Reload(nil)
	x.poll()
	if x.unsettled(x.ref) {
		t.Fatal("an issue of a removed project is still evaluated again")
	}
}

// unsettled reports whether the agent keeps ref for another evaluation.
func (x *h) unsettled(ref forge.IssueRef) bool {
	x.a.mu.Lock()
	defer x.a.mu.Unlock()
	_, ok := x.a.unsettled[ref]
	return ok
}
