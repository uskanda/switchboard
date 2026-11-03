package inboxserve

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/automerge"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
)

// recorder is a state.Auditor that keeps the events.
type recorder struct{ events []string }

func (r *recorder) Log(event string, _ map[string]any) { r.events = append(r.events, event) }

// withAutoMerge is withMerges with pull request 9 conflicting with its base, "me" trusted, and a store of requests.
func withAutoMerge(t *testing.T) *issuesWorld {
	t.Helper()
	w := withMerges(t)
	w.cfg.TrustedAuthors.GitHub = []string{"me"}
	w.gh.SetChange("o", "r", 9, fake.Change{Draft: true, Head: "abc", Conflict: true})
	st, err := automerge.Open(state.Dir{Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	st.Now = w.srv.Now
	w.srv.AutoMerge = st
	w.srv.Audit = &recorder{}
	return w
}

var ref50 = forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: 50}

// fix is a fix run of the conflict that pushed "def", in the order an agent takes the steps: it claims the issue,
// pushes, posts the done marker, then labels the issue done. A watch running beside it sees only states a real run
// passes through.
func (w *issuesWorld) fix() {
	w.gh.SetLabels(ctx, ref50, []string{"agent:running"})
	w.gh.PostAs = "bot"
	w.gh.PostComment(ctx, ref50, "Fixing\n\n"+marker.Marker{Host: "here", Run: "f", State: marker.Running, Reason: marker.Fix}.Format())
	w.gh.SetChange("o", "r", 9, fake.Change{Head: "def"})
	w.gh.PostComment(ctx, ref50, "### Done\n\n"+marker.Marker{Host: "here", Run: "f", State: marker.Done, SHA: "def", CR: 9, Fix: marker.Conflict}.Format())
	w.gh.PostAs = "me"
	w.gh.SetLabels(ctx, ref50, []string{"agent:done"})
}

func (w *issuesWorld) list(t *testing.T) inboxapi.Inbox {
	t.Helper()
	rec := w.do(http.MethodGet, "/v1/inbox", nil)
	var in inboxapi.Inbox
	if err := json.Unmarshal(rec.Body.Bytes(), &in); err != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	return in
}

func doneItem(in inboxapi.Inbox) *inboxapi.Item {
	for i := range in.Items {
		if in.Items[i].Ref == "gh:o/r#50" {
			return &in.Items[i]
		}
	}
	return nil
}

func TestAMergeRefusedForAConflictWaitsForTheFix(t *testing.T) {
	w := withAutoMerge(t)
	rec := w.do(http.MethodPost, "/v1/merge", inboxapi.MergeRequest{Ref: "gh:o/r#50", SHA: "abc"})
	var a inboxapi.MergeAnswer
	if json.Unmarshal(rec.Body.Bytes(), &a); rec.Code != http.StatusAccepted || a.Code != inboxapi.CodeAutoMergePending {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if r, ok := w.srv.AutoMerge.Get("gh:o/r#50"); !ok || r.SHA != "abc" || r.CR != 9 {
		t.Fatalf("request = %+v, %v", r, ok)
	}
	it := doneItem(w.list(t))
	if it == nil || it.AutoMerge == nil || it.AutoMerge.SHA != "abc" {
		t.Fatalf("item = %+v", it)
	}
	var d inboxapi.Detail
	json.Unmarshal(w.do(http.MethodGet, "/v1/issue?ref=gh:o/r%2350", nil).Body.Bytes(), &d)
	if d.AutoMerge == nil || d.AutoMerge.SHA != "abc" {
		t.Fatalf("detail = %+v", d.AutoMerge)
	}
	is, _, _ := w.gh.Thread(ctx, ref50)
	if len(is.Labels) != 1 || is.Labels[0] != "agent:done" {
		t.Fatalf("the labels changed: %v", is.Labels)
	}
}

func TestWithoutAStoreAConflictIsRefusedAsBefore(t *testing.T) {
	w := withMerges(t)
	w.gh.SetChange("o", "r", 9, fake.Change{Head: "abc", Conflict: true})
	if st, e := w.merge("gh:o/r#50", "abc"); st != http.StatusConflict || e.Code != inboxapi.CodeNotMergeable {
		t.Fatalf("%d %+v", st, e)
	}
}

func TestARefusalForTheCIIsNoRequest(t *testing.T) {
	w := withAutoMerge(t)
	w.gh.SetChange("o", "r", 9, fake.Change{Head: "abc", Refuse: "checks failed"})
	if st, e := w.merge("gh:o/r#50", "abc"); st != http.StatusConflict || e.Code != inboxapi.CodeNotMergeable {
		t.Fatalf("%d %+v", st, e)
	}
	if len(w.srv.AutoMerge.Pending()) != 0 {
		t.Fatal("a request was kept")
	}
}

func TestCancelAnAutoMerge(t *testing.T) {
	w := withAutoMerge(t)
	cancel := func(body any) int { return w.do(http.MethodPost, "/v1/automerge/cancel", body).Code }
	if st := cancel(inboxapi.CancelAutoMergeRequest{Ref: "gh:o/r#50"}); st != http.StatusNotFound {
		t.Fatalf("cancel of no request: %d", st)
	}
	w.merge("gh:o/r#50", "abc")
	if st := cancel("not json"); st != http.StatusBadRequest {
		t.Fatalf("a body that is not a cancel: %d", st)
	}
	if st := cancel(inboxapi.CancelAutoMergeRequest{Ref: "gh:o/r#50"}); st != http.StatusNoContent {
		t.Fatalf("cancel: %d", st)
	}
	in := w.list(t)
	if it := doneItem(in); it == nil || it.AutoMerge != nil {
		t.Fatalf("item = %+v", it)
	}
	if len(in.AutoMergeResults) != 1 || in.AutoMergeResults[0].Outcome != inboxapi.AutoMergeCancelled || in.AutoMergeResults[0].Reason != automerge.ReasonUser {
		t.Fatalf("results = %+v", in.AutoMergeResults)
	}
	w.fix()
	w.srv.autoMergeOnce(ctx)
	if c, _ := w.gh.ChangeState("o", "r", 9); c.Merged {
		t.Fatal("a cancelled merge was made")
	}
	if rec := w.do(http.MethodGet, "/v1/automerge/cancel", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", rec.Code)
	}
}

func TestTheWatchMergesAfterTheFix(t *testing.T) {
	w := withAutoMerge(t)
	w.merge("gh:o/r#50", "abc")
	w.srv.autoMergeOnce(ctx)
	if _, ok := w.srv.AutoMerge.Get("gh:o/r#50"); !ok {
		t.Fatal("the request ended before the fix")
	}
	w.list(t) // cached
	w.fix()
	w.srv.autoMergeOnce(ctx)
	if c, _ := w.gh.ChangeState("o", "r", 9); !c.Merged || c.Head != "def" {
		t.Fatalf("change = %+v", c)
	}
	in := w.list(t)
	if len(in.AutoMergeResults) != 1 || in.AutoMergeResults[0].Outcome != inboxapi.AutoMergeMerged || in.AutoMergeResults[0].Number != 9 {
		t.Fatalf("results = %+v: the cached listing was served", in.AutoMergeResults)
	}
	if it := doneItem(in); it == nil || it.AutoMerge != nil {
		t.Fatalf("item = %+v", it)
	}
	ev := w.srv.Audit.(*recorder).events
	if len(ev) != 2 || ev[0] != "automerge.requested" || ev[1] != "automerge.merged" {
		t.Fatalf("audit = %v", ev)
	}
}

// The watch runs from the start of the service, whether or not a page listens.
func TestTheWatchRunsWithoutListeners(t *testing.T) {
	w := withAutoMerge(t)
	w.srv.Interval = 5 * time.Millisecond
	c, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { w.srv.RunAutoMerge(c); close(done) }()
	defer func() { stop(); <-done }()
	w.merge("gh:o/r#50", "abc")
	w.fix()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if ch, _ := w.gh.ChangeState("o", "r", 9); ch.Merged {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the watch did not merge")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if w.srv.listeners() != 0 {
		t.Fatal("a listener appeared")
	}
}
