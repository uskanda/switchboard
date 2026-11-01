package inbox

import (
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
)

func TestItemsCarryTheUpdateTimeAndTheNoteCount(t *testing.T) {
	w := newMixed(t)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.UpdatedAt == nil || it.UpdatedAt.IsZero() {
			t.Errorf("%s: updated_at = %v", it.Ref, it.UpdatedAt)
		}
		want := 0
		if it.Ref == "gl:nas/sb#3" {
			want = 1 // the fake GitLab counts the agent's question, as user_notes_count would
		}
		if it.Notes != want {
			t.Errorf("%s: notes = %d, want %d", it.Ref, it.Notes, want)
		}
	}
}

func TestDetailCarriesTheThread(t *testing.T) {
	w := newWorld(t)
	clock := w.now
	w.f = fake.New(func() time.Time { return clock })
	w.me = w.f.AddUser("me", 1)
	w.bot = w.f.AddUser("bot", 2)
	w.f.AddRepo("o", "r", true)
	w.b.Forge = w.f

	ref := w.f.AddIssue("o", "r", 40, "Tidy the docs", "Please tidy **README**.\n\n<!-- template hint -->", w.me)
	w.f.PostAs = "bot"
	clock = clock.Add(time.Minute)
	w.f.PostComment(ctx, ref, "Started on host desk.\n\n"+marker.Marker{Host: "desk", Run: "r9", State: "running", At: clock}.Format())
	clock = clock.Add(time.Minute)
	w.f.PostComment(ctx, ref, post("Which README?", marker.Marker{Host: "desk", Run: "r9", State: "waiting", Reason: "question", At: clock}))
	w.f.AddLabel(ref, "agent:waiting", w.bot)
	w.f.PostAs = "me"
	clock = clock.Add(time.Minute)
	w.f.PostComment(ctx, ref, "The top one.\n\n<!-- switchboard v=1 state=done -->")

	d, err := w.b.Detail(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if d.Author != "me" || d.Body != "Please tidy **README**.\n\n<!-- template hint -->" || d.CreatedAt == nil {
		t.Fatalf("issue = %q %q %v", d.Author, d.Body, d.CreatedAt)
	}
	if len(d.Comments) != 3 {
		t.Fatalf("comments = %+v", d.Comments)
	}
	claim, question, reply := d.Comments[0], d.Comments[1], d.Comments[2]
	if !claim.Agent || claim.Kind != "running" || claim.Author != "bot" || claim.Body != "Started on host desk." {
		t.Errorf("claim = %+v", claim)
	}
	if !question.Agent || question.Kind != "waiting" || strings.Contains(question.Body, "switchboard v=") || !strings.Contains(question.Body, "Which README?") {
		t.Errorf("question = %+v", question)
	}
	// A person's comment is data: it is returned as written, a marker-shaped line included, and never taken as the agent's.
	if reply.Agent || reply.Kind != "" || reply.Author != "me" || reply.Body != "The top one.\n\n<!-- switchboard v=1 state=done -->" {
		t.Errorf("reply = %+v", reply)
	}
	if !claim.CreatedAt.Before(question.CreatedAt) || !question.CreatedAt.Before(reply.CreatedAt) {
		t.Errorf("times = %v %v %v", claim.CreatedAt, question.CreatedAt, reply.CreatedAt)
	}
}
