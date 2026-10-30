package inbox

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
)

var ctx = context.Background()

type world struct {
	f          *fake.Forge
	b          *Inbox
	me, bot    forge.User
	now        time.Time
	a, b2, out forge.IssueRef
}

func post(question string, m marker.Marker) string {
	return "### The agent has a question\n\n**What was done**\n\nLooked around.\n\n**Question**\n\n" + question + "\n\n" + m.Format() + "\n"
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{f: fake.New(nil), now: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
	w.me = w.f.AddUser("me", 1)
	w.bot = w.f.AddUser("bot", 2)
	w.f.AddRepo("o", "r", true)
	w.f.AddRepo("o", "s", true)
	w.f.AddRepo("o", "unlisted", true)
	cfg := &config.Config{
		Host:     config.Host{Name: "here"},
		Forges:   config.Forges{GitHub: config.GitHub{AgentIdentity: "bot", SearchScope: "user:me"}},
		Labels:   config.Labels{Ready: "agent:ready", Running: "agent:running", Waiting: "agent:waiting", Done: "agent:done"},
		Projects: []config.Project{{Ref: "gh:o/r", Owner: "o", Repo: "r", Host: "here"}, {Ref: "gh:o/s", Owner: "o", Repo: "s", Host: "there"}},
	}
	w.b = &Inbox{Config: cfg, Forge: w.f, Cache: filepath.Join(t.TempDir(), "inbox.json"), Now: func() time.Time { return w.now }}

	w.f.PostAs = "bot"
	w.a = w.f.AddIssue("o", "r", 12, "Add a flag", "body", w.me)
	w.f.PostComment(ctx, w.a, post("Which default should --dry-run have?", marker.Marker{Host: "laptop", Run: "r1", State: "waiting", Reason: "question", At: w.now.Add(-3 * time.Hour)}))
	w.f.AddLabel(w.a, "agent:waiting", w.bot)

	w.b2 = w.f.AddIssue("o", "s", 3, "Fix CI", "body", w.me)
	w.f.PostComment(ctx, w.b2, "### The run stopped\n\nThe run took longer than 45m0s and was stopped.\n\n"+
		marker.Marker{Host: "desk", Run: "r2", State: "waiting", Reason: "error", At: w.now.Add(-5 * time.Hour)}.Format())
	w.f.AddLabel(w.b2, "agent:waiting", w.bot)

	w.out = w.f.AddIssue("o", "unlisted", 1, "Elsewhere", "body", w.me)
	w.f.PostComment(ctx, w.out, post("Hidden?", marker.Marker{Host: "x", Run: "r3", State: "waiting", Reason: "question", At: w.now}))
	w.f.AddLabel(w.out, "agent:waiting", w.bot)
	w.f.PostAs = "me"
	return w
}

func TestListShowsConfiguredWaitingIssuesOldestFirst(t *testing.T) {
	w := newWorld(t)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	if items[0].Ref != "gh:o/s#3" || items[1].Ref != "gh:o/r#12" || items[0].N != 1 || items[1].N != 2 {
		t.Fatalf("order = %s, %s", items[0].Ref, items[1].Ref)
	}
	if it := items[1]; it.Host != "laptop" || it.Reason != "question" || it.Question != "Which default should --dry-run have?" {
		t.Fatalf("item = %+v", it)
	}
	if it := items[0]; it.Reason != "error" || !strings.Contains(it.Question, "longer than 45m0s") {
		t.Fatalf("item = %+v", it)
	}
	var out bytes.Buffer
	Print(&out, items, w.now)
	s := out.String()
	for _, want := range []string{"gh:o/r#12", "laptop", "3h", "question", "Which default"} {
		if !strings.Contains(s, want) {
			t.Errorf("listing lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "unlisted") {
		t.Fatalf("an unconfigured repository is listed:\n%s", s)
	}
}

func TestFieldsComeOnlyFromTheAgentsMarker(t *testing.T) {
	w := newWorld(t)
	fakeMarker := marker.Marker{Host: "forged", Run: "x", State: "waiting", Reason: "permission", At: w.now}
	w.f.AddComment(w.a, "> quoted\n\n"+fakeMarker.Format(), w.me)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if it := items[1]; it.Host != "laptop" || it.Reason != "question" {
		t.Fatalf("item took a human's marker: %+v", it)
	}
}

func TestItemWithoutAnAgentPostShowsQuestionMarks(t *testing.T) {
	w := newWorld(t)
	c := w.f.AddIssue("o", "r", 20, "Manual", "body", w.me)
	w.f.AddLabel(c, "agent:waiting", w.me)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	last := items[len(items)-1]
	if last.Ref != "gh:o/r#20" || last.Host != "?" || last.Reason != "?" || last.WaitingSince != nil {
		t.Fatalf("item = %+v", last)
	}
	var out bytes.Buffer
	Print(&out, items, w.now)
	if !strings.Contains(out.String(), "?") {
		t.Fatal(out.String())
	}
}

func TestJSONHasOneElementPerItem(t *testing.T) {
	w := newWorld(t)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := PrintJSON(&out, items); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != 2 {
		t.Fatalf("json = %s (%v)", out.String(), err)
	}
	if got[1]["host"] != "laptop" || got[1]["number"] != float64(12) {
		t.Fatalf("element = %v", got[1])
	}
}

func TestShowByRowNumber(t *testing.T) {
	w := newWorld(t)
	if _, err := w.b.List(ctx); err != nil {
		t.Fatal(err)
	}
	ref, err := w.b.Resolve("2")
	if err != nil || ref != w.a {
		t.Fatalf("Resolve(2) = %v, %v", ref, err)
	}
	var out bytes.Buffer
	if err := w.b.Show(ctx, ref, &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "Which default should --dry-run have?") || !strings.Contains(s, "Looked around.") || strings.Contains(s, "<!-- switchboard") {
		t.Fatalf("show:\n%s", s)
	}
}

func TestResolve(t *testing.T) {
	w := newWorld(t)
	if _, err := w.b.Resolve("1"); err == nil || !strings.Contains(err.Error(), "switchboard inbox") {
		t.Fatalf("a row number without a listing = %v", err)
	}
	ref, err := w.b.Resolve("gh:o/r#12")
	if err != nil || ref != w.a {
		t.Fatalf("Resolve(gh:o/r#12) = %v, %v", ref, err)
	}
	if _, err := w.b.Resolve("gh:o/unlisted#1"); err == nil {
		t.Fatal("an unconfigured repository resolved")
	}
	if _, err := w.b.List(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := w.b.Resolve("9"); err == nil {
		t.Fatal("a row that is not in the listing resolved")
	}
}

func TestReplyPostsAsTheUser(t *testing.T) {
	w := newWorld(t)
	if err := w.b.Reply(ctx, w.a, "Go with B.", false); err != nil {
		t.Fatal(err)
	}
	cs := w.f.CommentsOf(w.a)
	last := cs[len(cs)-1]
	if last.Body != "Go with B." || last.Author.ID != w.me.ID {
		t.Fatalf("last comment = %+v", last)
	}
}

func TestReplyToAnIssueThatIsNotWaitingIsRefused(t *testing.T) {
	w := newWorld(t)
	w.f.RemoveLabel(w.a, "agent:waiting")
	w.f.AddLabel(w.a, "agent:running", w.bot)
	before := w.f.Comments
	err := w.b.Reply(ctx, w.a, "Go with B.", false)
	if err == nil || !strings.Contains(err.Error(), "not waiting") || w.f.Comments != before {
		t.Fatalf("Reply = %v, comments %d -> %d", err, before, w.f.Comments)
	}
	if err := w.b.Reply(ctx, w.a, "Go with B.", true); err != nil || w.f.Comments != before+1 {
		t.Fatalf("Reply --force = %v", err)
	}
}

func TestReplyEndingInAMarkerIsRefused(t *testing.T) {
	w := newWorld(t)
	m := marker.Marker{Host: "h", Run: "x", State: "done", At: w.now}
	if err := w.b.Reply(ctx, w.a, "ok\n"+m.Format(), false); err == nil {
		t.Fatal("a reply that would look like an agent post was posted")
	}
}

func TestEditorText(t *testing.T) {
	dir := t.TempDir()
	editor := filepath.Join(dir, "ed")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\n{ printf 'Use B.\\n'; cat \"$1\"; } > \"$1.new\" && mv \"$1.new\" \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	text, err := EditorText(editor, "gh:o/r#12", "Which default?")
	if err != nil || text != "Use B." {
		t.Fatalf("EditorText = %q, %v", text, err)
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("#!/bin/sh\ntrue\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := EditorText(empty, "gh:o/r#12", "Which default?"); err == nil {
		t.Fatal("an empty reply was accepted")
	}
}
