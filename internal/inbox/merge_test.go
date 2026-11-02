package inbox

import (
	"errors"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
)

// mergeable adds an open done issue whose done post names pull request 9 at head sha "abc", a draft on the forge.
func mergeable(w *world, n int) forge.IssueRef {
	ref := w.f.AddIssue("o", "r", n, "Finished work", "body", w.me)
	w.f.PostAs = "bot"
	w.f.PostComment(ctx, ref, "### The work is done\n\nPull request: https://forge.test/o/r/pull/9\n\n"+
		marker.Marker{Host: "desk", Run: "d", State: marker.Done, At: w.now.Add(-time.Hour), SHA: "abc", CR: 9}.Format())
	w.f.PostAs = "me"
	w.f.AddLabel(ref, "agent:done", w.bot)
	w.f.SetChange("o", "r", 9, fake.Change{Draft: true, Head: "abc"})
	return ref
}

func TestListAndDetailCarryTheChangeOfTheDonePost(t *testing.T) {
	w := newWorld(t)
	ref := mergeable(w, 50)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var done *Item
	for i := range items {
		if items[i].State == Done {
			done = &items[i]
		}
	}
	if done == nil || done.ChangeNumber != 9 || done.ChangeSHA != "abc" {
		t.Fatalf("done item = %+v", done)
	}
	d, err := w.b.Detail(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if d.ChangeNumber != 9 || d.ChangeSHA != "abc" || d.ChangeURL != "https://forge.test/o/r/pull/9" {
		t.Fatalf("detail = %+v", d)
	}
}

func TestMergeMergesTheChangeOfTheLatestDonePost(t *testing.T) {
	w := newWorld(t)
	ref := mergeable(w, 50)
	if err := w.b.Merge(ctx, w.f, ref, "abc"); err != nil {
		t.Fatal(err)
	}
	if c, _ := w.f.ChangeState("o", "r", 9); !c.Merged || c.Draft {
		t.Fatalf("change = %+v", c)
	}
	is, _, _ := w.f.Thread(ctx, ref)
	if is.State != "open" || len(is.Labels) != 1 || is.Labels[0] != "agent:done" {
		t.Fatalf("the issue was touched: %+v", is)
	}
}

func TestMergeRefusesWhatThePageDidNotShow(t *testing.T) {
	t.Run("an older sha", func(t *testing.T) {
		w := newWorld(t)
		ref := mergeable(w, 50)
		if err := w.b.Merge(ctx, w.f, ref, "old"); !errors.Is(err, ErrStale) {
			t.Fatalf("err = %v, want ErrStale", err)
		}
		if c, _ := w.f.ChangeState("o", "r", 9); c.Merged {
			t.Fatal("merged")
		}
	})
	t.Run("not done", func(t *testing.T) {
		w := newWorld(t)
		ref := mergeable(w, 50)
		w.f.SetLabels(ctx, ref, []string{"agent:running"})
		if err := w.b.Merge(ctx, w.f, ref, "abc"); !errors.Is(err, ErrStale) {
			t.Fatalf("err = %v, want ErrStale", err)
		}
	})
	t.Run("closed", func(t *testing.T) {
		w := newWorld(t)
		ref := mergeable(w, 50)
		w.f.Close(ref)
		if err := w.b.Merge(ctx, w.f, ref, "abc"); !errors.Is(err, ErrStale) {
			t.Fatalf("err = %v, want ErrStale", err)
		}
	})
	t.Run("a done post without cr", func(t *testing.T) {
		w := newWorld(t)
		ref := doneIssue(w, 50, "Pull request: https://forge.test/o/r/pull/9", time.Hour)
		if err := w.b.Merge(ctx, w.f, ref, "abc"); !errors.Is(err, ErrStale) {
			t.Fatalf("err = %v, want ErrStale", err)
		}
	})
	t.Run("an empty sha", func(t *testing.T) {
		w := newWorld(t)
		ref := mergeable(w, 50)
		if err := w.b.Merge(ctx, w.f, ref, ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("err = %v, want ErrInvalid", err)
		}
	})
}

func TestMergePassesTheForgesRefusalOn(t *testing.T) {
	w := newWorld(t)
	ref := mergeable(w, 50)
	w.f.SetChange("o", "r", 9, fake.Change{Head: "abc", Refuse: "checks failed"})
	var nm *forge.NotMergeableError
	if err := w.b.Merge(ctx, w.f, ref, "abc"); !errors.As(err, &nm) || nm.Reason != "checks failed" {
		t.Fatalf("err = %v", err)
	}
	w.f.SetChange("o", "r", 9, fake.Change{Head: "def"})
	if err := w.b.Merge(ctx, w.f, ref, "abc"); !errors.Is(err, forge.ErrHeadMoved) {
		t.Fatalf("err = %v, want ErrHeadMoved", err)
	}
}

func TestMergeWithoutAClientForTheForge(t *testing.T) {
	w := newWorld(t)
	ref := mergeable(w, 50)
	if err := w.b.Merge(ctx, nil, ref, "abc"); err == nil {
		t.Fatal("merged without a client")
	}
}
