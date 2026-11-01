package inbox

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

func queuedComment(w *world, ref forge.IssueRef, ago time.Duration) {
	w.f.PostAs = "bot"
	w.f.PostComment(ctx, ref, "Queued: host desk runs as many issues at once as it may (1).\n\n"+
		marker.Marker{Host: "desk", State: marker.Queued, Reason: marker.QueuedHost, At: w.now.Add(-ago)}.Format())
	w.f.PostAs = "me"
}

func TestListShowsQueuedIssuesLast(t *testing.T) {
	w := newWorld(t)
	w.running(true)
	ready := w.f.AddIssue("o", "r", 40, "Queued work", "body", w.me)
	w.f.AddLabel(ready, "agent:ready", w.me)
	queuedComment(w, ready, 2*time.Minute)
	fresh := w.f.AddIssue("o", "r", 41, "Just labeled", "body", w.me)
	w.f.AddLabel(fresh, "agent:ready", w.me)

	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 || items[2].State != Running {
		t.Fatalf("items = %+v; want 2 waiting, 1 running, 1 queued, and #41 left out", items)
	}
	q := items[3]
	if q.State != Queued || q.Ref != "gh:o/r#40" || q.N != 0 || q.Host != "desk" || q.Reason != marker.QueuedHost {
		t.Fatalf("queued item = %+v", q)
	}
	if q.QueuedSince == nil || !q.QueuedSince.Equal(w.now.Add(-2*time.Minute)) {
		t.Fatalf("queued since = %v", q.QueuedSince)
	}
	var out bytes.Buffer
	Print(&out, items, w.now)
	s := out.String()
	i := strings.Index(s, "Queued")
	if i < 0 || i < strings.Index(s, "Running") || !strings.Contains(s[i:], "gh:o/r#40") || !strings.Contains(s[i:], "2m") {
		t.Fatalf("print:\n%s", s)
	}
}

func TestWaitingIssueQueuedOnAReplyIsQueued(t *testing.T) {
	w := newWorld(t)
	w.f.AddComment(w.a, "Use false.", w.me)
	queuedComment(w, w.a, time.Minute)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got *Item
	for i := range items {
		if items[i].Ref == "gh:o/r#12" {
			got = &items[i]
		}
	}
	if got == nil || got.State != Queued || got.N != 0 {
		t.Fatalf("o/r#12 = %+v; want it queued, without a row number", got)
	}

	d, err := w.b.Detail(ctx, w.a)
	if err != nil {
		t.Fatal(err)
	}
	if d.State != Queued || !d.Waiting || !strings.Contains(d.Post, "Which default should --dry-run have?") {
		t.Fatalf("detail = %+v; want queued, still waiting, with the question", d)
	}
	if d.Since == nil || !d.Since.Equal(w.now.Add(-time.Minute)) {
		t.Fatalf("since = %v", d.Since)
	}
	if err := w.b.Reply(ctx, w.a, "And keep it quiet.", false); err != nil {
		t.Fatalf("a reply to a queued waiting issue was refused: %v", err)
	}
}

func TestDetailOfAQueuedReadyIssue(t *testing.T) {
	w := newWorld(t)
	ready := w.f.AddIssue("o", "r", 40, "Queued work", "body", w.me)
	w.f.AddLabel(ready, "agent:ready", w.me)
	queuedComment(w, ready, time.Minute)
	d, err := w.b.Detail(ctx, ready)
	if err != nil {
		t.Fatal(err)
	}
	if d.State != Queued || d.Waiting || d.Reason != marker.QueuedHost || !strings.HasPrefix(d.Post, "Queued:") {
		t.Fatalf("detail = %+v", d)
	}
	if len(d.Comments) != 1 || d.Comments[0].Kind != Queued || strings.Contains(d.Comments[0].Body, "<!--") {
		t.Fatalf("comments = %+v", d.Comments)
	}
}
