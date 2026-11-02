package inbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// doneIssue adds an open issue the agent finished ago, with its done post naming link.
func doneIssue(w *world, n int, link string, ago time.Duration) forge.IssueRef {
	ref := w.f.AddIssue("o", "r", n, "Finished work", "body", w.me)
	w.f.PostAs = "bot"
	w.f.PostComment(ctx, ref, "### The work is done\n\n"+link+"\n\nAdded the flag.\n\n"+
		marker.Marker{Host: "desk", Run: "d", State: marker.Done, At: w.now.Add(-ago)}.Format())
	w.f.PostAs = "me"
	w.f.AddLabel(ref, "agent:done", w.bot)
	return ref
}

// An open done issue stays in the inbox until it is closed (ADR-042): after the waiting ones, oldest done first,
// numbered on from them.
func TestListShowsOpenDoneIssuesAfterTheWaitingOnes(t *testing.T) {
	w := newWorld(t)
	w.running(true)
	doneIssue(w, 50, "Pull request updated: https://forge.test/o/r/pull/9", time.Hour)
	doneIssue(w, 51, "PR：https://forge.test/o/r/pull/8", 2*time.Hour)
	closed := doneIssue(w, 52, "Pull request: https://forge.test/o/r/pull/7", 3*time.Hour)
	w.f.Close(closed)

	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("items = %+v; want 2 waiting, 2 done, 1 running", items)
	}
	d1, d2 := items[2], items[3]
	if d1.State != Done || d1.Ref != "gh:o/r#51" || d1.N != 3 || d2.Ref != "gh:o/r#50" || d2.N != 4 {
		t.Fatalf("done items = %+v, %+v", d1, d2)
	}
	if d1.Host != "desk" || d1.ChangeURL != "https://forge.test/o/r/pull/8" || d1.DoneSince == nil || !d1.DoneSince.Equal(w.now.Add(-2*time.Hour)) {
		t.Fatalf("done item = %+v", d1)
	}
	if d2.ChangeURL != "https://forge.test/o/r/pull/9" {
		t.Fatalf("change url = %q", d2.ChangeURL)
	}
	if items[4].State != Running {
		t.Fatalf("last item = %+v", items[4])
	}

	var out bytes.Buffer
	Print(&out, items, w.now)
	s := out.String()
	i := strings.Index(s, "Done (waiting for close)")
	if i < 0 || i < strings.Index(s, "Running") || !strings.Contains(s[i:], "3  gh:o/r#51") || !strings.Contains(s[i:], "https://forge.test/o/r/pull/8") || !strings.Contains(s[i:], "2h") {
		t.Fatalf("print:\n%s", s)
	}

	data, err := os.ReadFile(w.b.Cache)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct{ Rows []string }
	if err := json.Unmarshal(data, &saved); err != nil || len(saved.Rows) != 4 || saved.Rows[2] != "gh:o/r#51" {
		t.Fatalf("rows = %v, %v", saved.Rows, err)
	}
	if ref, err := w.b.Resolve("4"); err != nil || ref.Number != 50 {
		t.Fatalf("Resolve(4) = %v, %v", ref, err)
	}

	var js bytes.Buffer
	if err := PrintJSON(&js, items[2:3]); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"state": "done"`, `"done_since": "2026-10-29T10:00:00Z"`, `"change_url": "https://forge.test/o/r/pull/8"`} {
		if !strings.Contains(js.String(), want) {
			t.Errorf("json lacks %s:\n%s", want, js.String())
		}
	}
}

func TestDoneIssueTakesAReplyUntilItIsClosed(t *testing.T) {
	w := newWorld(t)
	ref := doneIssue(w, 50, "Pull request: https://forge.test/o/r/pull/9", time.Hour)
	d, err := w.b.Detail(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if d.State != Done || !d.Done || d.Waiting || !strings.Contains(d.Post, "Added the flag.") {
		t.Fatalf("detail = %+v", d)
	}
	if err := w.b.Reply(ctx, ref, "Please fix the conflict.", false); err != nil {
		t.Fatalf("a reply to a done issue was refused: %v", err)
	}
	w.f.Close(ref)
	err = w.b.Reply(ctx, ref, "Again.", false)
	if !errors.Is(err, ErrNotWaiting) || !strings.Contains(err.Error(), "neither waiting nor done (closed)") {
		t.Fatalf("a reply to a closed issue: %v", err)
	}
	if d, err := w.b.Detail(ctx, ref); err != nil || d.Done {
		t.Fatalf("detail of the closed issue = %+v, %v", d, err)
	}
}
