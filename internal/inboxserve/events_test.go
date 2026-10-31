package inboxserve

import (
	"context"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/marker"
)

// listen follows the event stream until the test ends, passing each listing on.
func listen(t *testing.T, w *world) (<-chan *inboxapi.Inbox, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	got := make(chan *inboxapi.Inbox, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.c.Events(ctx, func(in *inboxapi.Inbox) { got <- in })
	}()
	t.Cleanup(func() { cancel(); <-done })
	return got, cancel
}

func next(t *testing.T, got <-chan *inboxapi.Inbox) *inboxapi.Inbox {
	t.Helper()
	select {
	case in := <-got:
		return in
	case <-time.After(5 * time.Second):
		t.Fatal("no listing arrived")
		return nil
	}
}

func quiet(t *testing.T, got <-chan *inboxapi.Inbox, d time.Duration) {
	t.Helper()
	select {
	case in := <-got:
		t.Fatalf("a listing arrived: %+v", in.Items)
	case <-time.After(d):
	}
}

func (w *world) searches() int32 { return w.gh.searches.Load() + w.gl.searches.Load() }

func TestEventsSendTheListingRightAwayAndThenOnlyChanges(t *testing.T) {
	w := newWorld(t)
	w.srv.Interval = 30 * time.Millisecond
	got, _ := listen(t, w)
	first := next(t, got)
	if len(first.Items) != 1 || first.Items[0].Ref != "gh:o/r#12" || first.Items[0].UpdatedAt == nil {
		t.Fatalf("first = %+v", first.Items)
	}
	// Several intervals pass with nothing changed: nothing is sent, though the forges are read again.
	before := w.searches()
	quiet(t, got, 150*time.Millisecond)
	if w.searches() <= before {
		t.Fatalf("the forges were not read again: %d searches", w.searches())
	}

	w.f.PostAs = "bot"
	ref := w.f.AddIssue("o", "r", 13, "Fix a typo", "body", w.bot)
	w.f.PostComment(ctx, ref, "### The agent has a question\n\n**Question**\n\nWhich typo?\n\n"+
		marker.Marker{Host: "laptop", Run: "r2", State: "waiting", Reason: "question", At: w.now}.Format()+"\n")
	w.f.AddLabel(ref, "agent:waiting", w.bot)
	if in := next(t, got); len(in.Items) != 2 {
		t.Fatalf("after a new wait: %+v", in.Items)
	}
}

func TestNoOneListeningMeansNoReads(t *testing.T) {
	w := newWorld(t)
	w.srv.Interval = 20 * time.Millisecond
	time.Sleep(100 * time.Millisecond)
	if n := w.searches(); n != 0 {
		t.Fatalf("searches with no one listening = %d", n)
	}
	got, cancel := listen(t, w)
	next(t, got)
	cancel()
	// Once the last listener leaves, the reads stop.
	time.Sleep(60 * time.Millisecond)
	after := w.searches()
	time.Sleep(100 * time.Millisecond)
	if n := w.searches(); n != after {
		t.Fatalf("searches went on after the last listener left: %d then %d", after, n)
	}
}

func TestAReplyReadsAgainWithoutWaiting(t *testing.T) {
	w := newWorld(t)
	w.srv.Interval = time.Hour
	got, _ := listen(t, w)
	next(t, got)
	if err := w.c.Reply(ctx, "gh:o/r#12", "false, please", false); err != nil {
		t.Fatal(err)
	}
	// The reply itself does not change the listing on the fake forge (the label stays until the agent picks it up),
	// but the forges are read again at once.
	deadline := time.Now().Add(5 * time.Second)
	for w.searches() < 4 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := w.searches(); n < 4 {
		t.Fatalf("searches = %d, want a second listing right after the reply", n)
	}
}

func TestTheStreamsListingIsTheCache(t *testing.T) {
	w := newWorld(t)
	w.srv.Interval = time.Hour
	got, _ := listen(t, w)
	next(t, got)
	before := w.searches()
	if _, err := w.c.List(ctx); err != nil {
		t.Fatal(err)
	}
	if n := w.searches(); n != before {
		t.Fatalf("a GET right after the stream's listing asked the forges again: %d then %d", before, n)
	}
}

func TestEventsEndsWithAnErrorWhenTheServiceStops(t *testing.T) {
	w := newWorld(t)
	w.srv.Interval = time.Hour
	errc := make(chan error, 1)
	first := make(chan struct{}, 1)
	go func() {
		errc <- w.c.Events(context.Background(), func(*inboxapi.Inbox) { first <- struct{}{} })
	}()
	<-first
	w.stop()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("Events returned nil when the stream broke")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Events did not return after the service stopped")
	}
}
