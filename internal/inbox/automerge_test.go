package inbox

import (
	"errors"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/automerge"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
)

// conflicting is mergeable with pull request 9 conflicting with its base, and "me" trusted.
func conflicting(w *world, n int) forge.IssueRef {
	w.b.Config.TrustedAuthors.GitHub = []string{"me"}
	ref := mergeable(w, n)
	w.f.SetChange("o", "r", 9, fake.Change{Draft: true, Head: "abc", Conflict: true})
	return ref
}

// requested is the request the inbox makes of ref after the merge at "abc" was refused.
func requested(t *testing.T, w *world, ref forge.IssueRef) automerge.Request {
	t.Helper()
	r, err := w.b.AutoMergeCandidate(ctx, w.f, ref, "abc")
	if err != nil || r == nil {
		t.Fatalf("candidate = %+v, %v", r, err)
	}
	return *r
}

// as posts body as the agent and puts the state label to.
func (w *world) as(ref forge.IssueRef, m marker.Marker, label string) {
	w.f.PostAs = "bot"
	w.f.PostComment(ctx, ref, "### A post\n\n"+m.Format())
	w.f.PostAs = "me"
	w.f.SetLabels(ctx, ref, []string{label})
}

// fixed is a fix run that fixed what it was started for and pushed sha.
func (w *world) fixed(ref forge.IssueRef, sha, fix string) {
	w.as(ref, marker.Marker{Host: "desk", Run: "f-" + sha, State: marker.Running, Reason: marker.Fix}, "agent:running")
	w.as(ref, marker.Marker{Host: "desk", Run: "f-" + sha, State: marker.Done, SHA: sha, CR: 9, Fix: fix}, "agent:done")
	w.f.SetChange("o", "r", 9, fake.Change{Head: sha})
}

func step(t *testing.T, w *world, r automerge.Request) AutoMergeStep {
	t.Helper()
	s, err := w.b.AutoMergeStep(ctx, w.f, r)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAMergeRefusedForAConflictIsACandidate(t *testing.T) {
	w := newWorld(t)
	ref := conflicting(w, 50)
	_, comments, _ := w.f.Thread(ctx, ref)
	r := requested(t, w, ref)
	want := automerge.Request{Ref: "gh:o/r#50", CR: 9, SHA: "abc", Since: w.now, After: comments[len(comments)-1].ID}
	if r != want {
		t.Fatalf("request = %+v, want %+v", r, want)
	}
}

func TestAMergeRefusedForAnotherReasonIsNoCandidate(t *testing.T) {
	for name, c := range map[string]fake.Change{
		"CI":           {Head: "abc", Refuse: "checks failed"},
		"a moved head": {Head: "def", Conflict: true},
		"closed":       {Head: "abc", Conflict: true, Closed: true},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t)
			ref := mergeable(w, 50)
			w.f.SetChange("o", "r", 9, c)
			if r, err := w.b.AutoMergeCandidate(ctx, w.f, ref, "abc"); r != nil || err != nil {
				t.Fatalf("candidate = %+v, %v", r, err)
			}
		})
	}
}

func TestAutoMergeMergesTheFixOfTheConflict(t *testing.T) {
	w := newWorld(t)
	ref := conflicting(w, 50)
	r := requested(t, w, ref)
	if s := step(t, w, r); s.Outcome != "" {
		t.Fatalf("before the fix: %+v", s)
	}
	w.as(ref, marker.Marker{Host: "desk", Run: "f", State: marker.Running, Reason: marker.Fix}, "agent:running")
	if s := step(t, w, r); s.Outcome != "" {
		t.Fatalf("while the fix runs: %+v", s)
	}
	w.as(ref, marker.Marker{Host: "desk", Run: "f", State: marker.Done, SHA: "def", CR: 9, Fix: marker.Conflict}, "agent:done")
	w.f.SetChange("o", "r", 9, fake.Change{Draft: true, Head: "def"})
	if s := step(t, w, r); s.Outcome != automerge.Merged {
		t.Fatalf("after the fix: %+v", s)
	}
	if c, _ := w.f.ChangeState("o", "r", 9); !c.Merged || c.Head != "def" || c.Draft {
		t.Fatalf("change = %+v", c)
	}
}

func TestAutoMergeWaitsThroughACIFix(t *testing.T) {
	w := newWorld(t)
	ref := conflicting(w, 50)
	r := requested(t, w, ref)
	w.fixed(ref, "def", marker.Conflict)
	w.f.SetChange("o", "r", 9, fake.Change{Head: "def", Refuse: "Required status check is expected"})
	if s := step(t, w, r); s.Outcome != "" {
		t.Fatalf("a refusal for the CI ended the request: %+v", s)
	}
	w.fixed(ref, "ghi", marker.CI)
	if s := step(t, w, r); s.Outcome != automerge.Merged {
		t.Fatalf("after the CI fix: %+v", s)
	}
	if c, _ := w.f.ChangeState("o", "r", 9); !c.Merged || c.Head != "ghi" {
		t.Fatalf("change = %+v", c)
	}
}

func TestAutoMergeIsCancelled(t *testing.T) {
	for _, tc := range []struct {
		name   string
		then   func(w *world, ref forge.IssueRef)
		reason string
	}{
		{"closed", func(w *world, ref forge.IssueRef) { w.f.Close(ref) }, automerge.ReasonClosed},
		{"waiting", func(w *world, ref forge.IssueRef) {
			w.as(ref, marker.Marker{Host: "desk", Run: "w", State: marker.Waiting, Reason: marker.Conflict}, "agent:waiting")
		}, automerge.ReasonWaiting},
		{"a person commented", func(w *world, ref forge.IssueRef) {
			w.f.AddComment(ref, "Also rename the flag.", w.me)
		}, automerge.ReasonInstruction},
		{"a run that is not a fix run", func(w *world, ref forge.IssueRef) {
			w.as(ref, marker.Marker{Host: "desk", Run: "x", State: marker.Running}, "agent:running")
		}, automerge.ReasonInstruction},
		{"a done post of another run", func(w *world, ref forge.IssueRef) {
			w.as(ref, marker.Marker{Host: "desk", Run: "x", State: marker.Done, SHA: "def", CR: 9}, "agent:done")
			w.f.SetChange("o", "r", 9, fake.Change{Head: "def"})
		}, automerge.ReasonNotFix},
		{"another change request", func(w *world, ref forge.IssueRef) {
			w.as(ref, marker.Marker{Host: "desk", Run: "x", State: marker.Done, SHA: "def", CR: 10, Fix: marker.Conflict}, "agent:done")
			w.f.SetChange("o", "r", 10, fake.Change{Head: "def"})
		}, automerge.ReasonChanged},
		{"a person pushed", func(w *world, ref forge.IssueRef) {
			w.fixed(ref, "def", marker.Conflict)
			w.f.SetChange("o", "r", 9, fake.Change{Head: "mine"})
		}, automerge.ReasonPushed},
		{"a person pushed before the fix", func(w *world, ref forge.IssueRef) {
			w.f.SetChange("o", "r", 9, fake.Change{Head: "mine", Conflict: true})
		}, automerge.ReasonPushed},
		{"the change request was closed", func(w *world, ref forge.IssueRef) {
			w.f.SetChange("o", "r", 9, fake.Change{Head: "abc", Closed: true})
		}, automerge.ReasonChangeClosed},
		{"the change request was merged", func(w *world, ref forge.IssueRef) {
			w.fixed(ref, "def", marker.Conflict)
			w.f.SetChange("o", "r", 9, fake.Change{Head: "def", Merged: true})
		}, automerge.ReasonChangeClosed},
		{"the token may not merge", func(w *world, ref forge.IssueRef) {
			w.fixed(ref, "def", marker.Conflict)
			w.f.FailMerge = forge.ErrForbidden
		}, automerge.ReasonForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			ref := conflicting(w, 50)
			r := requested(t, w, ref)
			tc.then(w, ref)
			if s := step(t, w, r); s.Outcome != automerge.Cancelled || s.Reason != tc.reason {
				t.Fatalf("step = %+v, want cancelled for %s", s, tc.reason)
			}
			if c, _ := w.f.ChangeState("o", "r", 9); c.Merged && tc.reason != automerge.ReasonChangeClosed {
				t.Fatal("the change request was merged")
			}
		})
	}
}

// What came before the press does not count: a person's comment and the first run's claim are older.
func TestAutoMergeCountsOnlyWhatCameAfterThePress(t *testing.T) {
	w := newWorld(t)
	w.b.Config.TrustedAuthors.GitHub = []string{"me"}
	ref := w.f.AddIssue("o", "r", 50, "Finished work", "body", w.me)
	w.as(ref, marker.Marker{Host: "desk", Run: "d", State: marker.Running, Mode: marker.ModeDirect}, "agent:running")
	w.f.AddComment(ref, "Looks good.", w.me)
	w.as(ref, marker.Marker{Host: "desk", Run: "d", State: marker.Done, SHA: "abc", CR: 9, At: w.now.Add(-time.Hour)}, "agent:done")
	w.f.SetChange("o", "r", 9, fake.Change{Head: "abc", Conflict: true})
	r := requested(t, w, ref)
	w.fixed(ref, "def", marker.Conflict)
	if s := step(t, w, r); s.Outcome != automerge.Merged {
		t.Fatalf("step = %+v", s)
	}
}

// A forge that cannot be read is an error, which leaves the request for the next look.
func TestAutoMergeReportsAForgeThatCannotBeRead(t *testing.T) {
	w := newWorld(t)
	ref := conflicting(w, 50)
	r := requested(t, w, ref)
	w.fixed(ref, "def", marker.Conflict)
	w.f.FailHead = errors.New("connection refused")
	if _, err := w.b.AutoMergeStep(ctx, w.f, r); err == nil {
		t.Fatal("no error")
	}
	w.f.FailHead = nil
	w.f.FailMerge = errors.New("connection reset")
	if _, err := w.b.AutoMergeStep(ctx, w.f, r); err == nil {
		t.Fatal("no error for a merge that could not be sent")
	}
}
