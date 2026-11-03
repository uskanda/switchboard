package automerge

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/state"
)

var t0 = time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)

func open(t *testing.T, d state.Dir, now *time.Time) *Store {
	t.Helper()
	s, err := Open(d)
	if err != nil {
		t.Fatal(err)
	}
	s.Now = func() time.Time { return *now }
	return s
}

func TestARequestSurvivesAReopen(t *testing.T) {
	d := state.Dir{Path: t.TempDir()}
	now := t0
	s := open(t, d, &now)
	r := Request{Ref: "gh:o/r#50", CR: 9, SHA: "abc", Since: t0, After: 1001}
	if err := s.Put(r); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(d.Path, "inbox", "automerge.json")); err != nil {
		t.Fatalf("the requests are not in inbox/automerge.json: %v", err)
	}
	again := open(t, d, &now)
	got, ok := again.Get("gh:o/r#50")
	if !ok || got != r {
		t.Fatalf("after a reopen: %+v, %v", got, ok)
	}
	if p := again.Pending(); len(p) != 1 || p[0] != r {
		t.Fatalf("pending = %+v", p)
	}
}

func TestAPressReplacesTheRequestOfTheSameIssue(t *testing.T) {
	now := t0
	s := open(t, state.Dir{Path: t.TempDir()}, &now)
	s.Put(Request{Ref: "gh:o/r#50", CR: 9, SHA: "abc", Since: t0})
	s.Put(Request{Ref: "gh:o/r#51", CR: 10, SHA: "x", Since: t0})
	s.Put(Request{Ref: "gh:o/r#50", CR: 9, SHA: "def", Since: t0.Add(time.Minute)})
	p := s.Pending()
	if len(p) != 2 {
		t.Fatalf("pending = %+v", p)
	}
	if got, _ := s.Get("gh:o/r#50"); got.SHA != "def" {
		t.Fatalf("the request was not replaced: %+v", got)
	}
}

func TestFinishingKeepsTheResultForADay(t *testing.T) {
	d := state.Dir{Path: t.TempDir()}
	now := t0
	s := open(t, d, &now)
	r := Request{Ref: "gh:o/r#50", CR: 9, SHA: "abc", Since: t0}
	s.Put(r)
	now = t0.Add(time.Hour)
	if err := s.Finish(r, Merged, ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(r.Ref); ok {
		t.Fatal("a finished request still waits")
	}
	want := Result{Ref: "gh:o/r#50", CR: 9, Outcome: Merged, At: now}
	if res := open(t, d, &now).Results(); len(res) != 1 || res[0] != want {
		t.Fatalf("results = %+v", res)
	}
	now = t0.Add(26 * time.Hour)
	if res := s.Results(); len(res) != 0 {
		t.Fatalf("a result older than a day is kept: %+v", res)
	}
}

// A request replaced by a new press while it was looked at is not finished with the old one's result.
func TestFinishingAReplacedRequestLeavesTheNewOne(t *testing.T) {
	now := t0
	s := open(t, state.Dir{Path: t.TempDir()}, &now)
	old := Request{Ref: "gh:o/r#50", CR: 9, SHA: "abc", Since: t0}
	s.Put(old)
	s.Put(Request{Ref: "gh:o/r#50", CR: 9, SHA: "def", Since: t0.Add(time.Minute)})
	s.Finish(old, Cancelled, ReasonPushed)
	if got, ok := s.Get("gh:o/r#50"); !ok || got.SHA != "def" {
		t.Fatalf("the new request is gone: %+v, %v", got, ok)
	}
	if res := s.Results(); len(res) != 0 {
		t.Fatalf("results = %+v", res)
	}
}

func TestCancel(t *testing.T) {
	now := t0
	s := open(t, state.Dir{Path: t.TempDir()}, &now)
	s.Put(Request{Ref: "gh:o/r#50", CR: 9, SHA: "abc", Since: t0})
	if ok, err := s.Cancel("gh:o/r#51"); ok || err != nil {
		t.Fatalf("cancel of no request: %v, %v", ok, err)
	}
	if ok, err := s.Cancel("gh:o/r#50"); !ok || err != nil {
		t.Fatalf("cancel: %v, %v", ok, err)
	}
	if res := s.Results(); len(res) != 1 || res[0].Outcome != Cancelled || res[0].Reason != ReasonUser {
		t.Fatalf("results = %+v", res)
	}
	if len(s.Pending()) != 0 {
		t.Fatal("a cancelled request still waits")
	}
}

func TestABrokenFileIsAnError(t *testing.T) {
	d := state.Dir{Path: t.TempDir()}
	os.MkdirAll(filepath.Join(d.Path, "inbox"), 0o700)
	os.WriteFile(filepath.Join(d.Path, "inbox", "automerge.json"), []byte("{"), 0o600)
	if _, err := Open(d); err == nil {
		t.Fatal("a broken file was read as empty")
	}
}
