package inbox

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/marker"
)

func TestQueuedOnAPauseShowsWhenItEnds(t *testing.T) {
	w := newWorld(t)
	ready := w.f.AddIssue("o", "r", 40, "Limited work", "body", w.me)
	w.f.AddLabel(ready, "agent:ready", w.me)
	until := time.Date(2026, 11, 3, 18, 30, 0, 0, time.UTC)
	w.f.PostAs = "bot"
	w.f.PostComment(ctx, ready, "The run hit a usage limit.\n\n"+
		marker.Marker{Host: "desk", Run: "r1", State: marker.Queued, Reason: marker.QueuedLimit, At: w.now.Add(-time.Minute), Until: until}.Format())
	w.f.PostAs = "me"

	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var q *Item
	for i := range items {
		if items[i].Ref == "gh:o/r#40" {
			q = &items[i]
		}
	}
	if q == nil || q.State != Queued || q.Reason != marker.QueuedLimit || q.QueuedUntil == nil || !q.QueuedUntil.Equal(until) {
		t.Fatalf("queued item = %+v, want reason limit until %v", q, until)
	}
	var out bytes.Buffer
	Print(&out, items, w.now)
	if want := "limit (until " + until.Local().Format("01-02 15:04") + ")"; !strings.Contains(out.String(), want) {
		t.Fatalf("print lacks %q:\n%s", want, out.String())
	}
}

func TestQueuedOnASlotHasNoUntil(t *testing.T) {
	w := newWorld(t)
	ready := w.f.AddIssue("o", "r", 40, "Queued work", "body", w.me)
	w.f.AddLabel(ready, "agent:ready", w.me)
	queuedComment(w, ready, time.Minute)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Ref == "gh:o/r#40" && it.QueuedUntil != nil {
			t.Fatalf("queued_until = %v on a full slot", it.QueuedUntil)
		}
	}
}
