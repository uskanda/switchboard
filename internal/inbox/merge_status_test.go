package inbox

import (
	"errors"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
)

func TestMergeStatusReadsTheChangeAndItsChecks(t *testing.T) {
	w := newWorld(t)
	ref := mergeable(w, 50)
	w.f.SetChecks("o", "r", "abc",
		forge.Check{Name: "test", Done: true, Conclusion: forge.Success, URL: "https://ci/1"},
		forge.Check{Name: "lint", Done: true, Conclusion: forge.Failure, URL: "https://ci/2"},
		forge.Check{Name: "build", URL: "https://ci/3"},
		forge.Check{Name: "deploy", Done: true, Conclusion: forge.Skipped, URL: "https://ci/4"},
	)
	st, err := w.b.MergeStatus(ctx, w.f, ref, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if st.State != forge.CROpen || st.HeadMoved || st.Mergeable != MergeClean || st.CI != CIRead {
		t.Fatalf("status = %+v", st)
	}
	want := []CheckStatus{
		{Name: "test", Status: CheckSuccess, URL: "https://ci/1"},
		{Name: "lint", Status: CheckFailure, URL: "https://ci/2"},
		{Name: "build", Status: CheckRunning, URL: "https://ci/3"},
		{Name: "deploy", Status: CheckOther, URL: "https://ci/4"},
	}
	if len(st.Checks) != len(want) {
		t.Fatalf("checks = %+v", st.Checks)
	}
	for i := range want {
		if st.Checks[i] != want[i] {
			t.Errorf("check %d = %+v, want %+v", i, st.Checks[i], want[i])
		}
	}
}

func TestMergeStatusOfTheChange(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    fake.Change
		mergeable string
		moved     bool
		state     string
	}{
		{"a conflict", fake.Change{Head: "abc", Conflict: true}, MergeConflict, false, forge.CROpen},
		{"computing", fake.Change{Head: "abc", Computing: true}, MergeComputing, false, forge.CROpen},
		{"a conflict while computing", fake.Change{Head: "abc", Conflict: true, Computing: true}, MergeConflict, false, forge.CROpen},
		{"a moved head", fake.Change{Head: "def"}, MergeClean, true, forge.CROpen},
		{"merged", fake.Change{Head: "abc", Merged: true}, MergeClean, false, forge.CRMerged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t)
			ref := mergeable(w, 50)
			w.f.SetChange("o", "r", 9, tc.change)
			st, err := w.b.MergeStatus(ctx, w.f, ref, "abc")
			if err != nil {
				t.Fatal(err)
			}
			if st.Mergeable != tc.mergeable || st.HeadMoved != tc.moved || st.State != tc.state {
				t.Fatalf("status = %+v", st)
			}
		})
	}
}

func TestMergeStatusWithoutChecks(t *testing.T) {
	w := newWorld(t)
	ref := mergeable(w, 50)
	st, err := w.b.MergeStatus(ctx, w.f, ref, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if st.CI != CINone || st.Checks == nil || len(st.Checks) != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestMergeStatusWithATokenThatCannotReadCI(t *testing.T) {
	w := newWorld(t)
	ref := mergeable(w, 50)
	w.f.SetChange("o", "r", 9, fake.Change{Head: "abc", Conflict: true})
	w.f.FailCI = forge.ErrForbidden
	st, err := w.b.MergeStatus(ctx, w.f, ref, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if st.CI != CIForbidden || len(st.Checks) != 0 || st.Mergeable != MergeConflict {
		t.Fatalf("status = %+v", st)
	}
}

func TestMergeStatusPassesOtherCIErrorsOn(t *testing.T) {
	w := newWorld(t)
	ref := mergeable(w, 50)
	w.f.FailCI = errors.New("boom")
	if _, err := w.b.MergeStatus(ctx, w.f, ref, "abc"); err == nil {
		t.Fatal("no error")
	}
}

func TestMergeStatusRefusesWhatThePageDidNotShow(t *testing.T) {
	t.Run("an older sha", func(t *testing.T) {
		w := newWorld(t)
		ref := mergeable(w, 50)
		if _, err := w.b.MergeStatus(ctx, w.f, ref, "old"); !errors.Is(err, ErrStale) {
			t.Fatalf("err = %v, want ErrStale", err)
		}
	})
	t.Run("not done", func(t *testing.T) {
		w := newWorld(t)
		ref := mergeable(w, 50)
		w.f.SetLabels(ctx, ref, []string{"agent:running"})
		if _, err := w.b.MergeStatus(ctx, w.f, ref, "abc"); !errors.Is(err, ErrStale) {
			t.Fatalf("err = %v, want ErrStale", err)
		}
	})
	t.Run("a done post without cr", func(t *testing.T) {
		w := newWorld(t)
		ref := doneIssue(w, 50, "Pull request: https://forge.test/o/r/pull/9", time.Hour)
		if _, err := w.b.MergeStatus(ctx, w.f, ref, "abc"); !errors.Is(err, ErrStale) {
			t.Fatalf("err = %v, want ErrStale", err)
		}
	})
	t.Run("an empty sha", func(t *testing.T) {
		w := newWorld(t)
		ref := mergeable(w, 50)
		if _, err := w.b.MergeStatus(ctx, w.f, ref, ""); !errors.Is(err, ErrInvalid) {
			t.Fatalf("err = %v, want ErrInvalid", err)
		}
	})
	t.Run("no client", func(t *testing.T) {
		w := newWorld(t)
		ref := mergeable(w, 50)
		if _, err := w.b.MergeStatus(ctx, nil, ref, "abc"); err == nil {
			t.Fatal("no error")
		}
	})
}
