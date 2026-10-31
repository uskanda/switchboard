package inbox

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/marker"
)

// running adds an issue of o/r that runs on desk since 20 minutes ago, with a progress comment of two lines.
func (w *world) running(withProgress bool) {
	w.f.PostAs = "bot"
	ref := w.f.AddIssue("o", "r", 30, "Speed up", "body", w.me)
	w.f.PostComment(ctx, ref, "Started on host desk.\n\n"+marker.Marker{Host: "desk", Run: "r7", State: "running", At: w.now.Add(-20 * time.Minute)}.Format())
	if withProgress {
		w.f.PostComment(ctx, ref, "### Progress of the run\n\n- 11:41 1m Read 2 | 読む\n- 11:59 19m Bash 3, Read 2 | テストを直す\n\n"+
			marker.Marker{Host: "desk", Run: "r7", State: "progress"}.Format())
	}
	w.f.AddLabel(ref, "agent:running", w.bot)
	w.f.PostAs = "me"
}

func TestListShowsRunningIssuesApart(t *testing.T) {
	w := newWorld(t)
	w.running(true)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].State != "waiting" || items[1].State != "waiting" {
		t.Fatalf("items = %+v", items)
	}
	r := items[2]
	if r.State != "running" || r.Ref != "gh:o/r#30" || r.N != 0 || r.Host != "desk" || r.Progress != "11:59 19m Bash 3, Read 2 | テストを直す" {
		t.Fatalf("running item = %+v", r)
	}
	if r.RunningSince == nil || !r.RunningSince.Equal(w.now.Add(-20*time.Minute)) {
		t.Fatalf("running since = %v", r.RunningSince)
	}
	// Row numbers name the waiting rows only.
	data, _ := os.ReadFile(w.b.Cache)
	if strings.Contains(string(data), "#30") {
		t.Fatalf("rows = %s", data)
	}
	if _, err := w.b.Resolve("3"); err == nil {
		t.Fatal("row 3 resolved to the running issue")
	}

	var out bytes.Buffer
	Print(&out, items, w.now)
	s := out.String()
	run := strings.Index(s, "Running")
	if run < 0 || strings.Index(s, "gh:o/r#30") < run || !strings.Contains(s[run:], "20m") || !strings.Contains(s[run:], "テストを直す") {
		t.Fatalf("print:\n%s", s)
	}

	out.Reset()
	if err := PrintJSON(&out, items); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != 3 || got[0]["state"] != "waiting" || got[2]["state"] != "running" || got[2]["progress"] == nil {
		t.Fatalf("json = %s (%v)", out.String(), err)
	}
}

func TestRunningIssueWithoutAProgressComment(t *testing.T) {
	w := newWorld(t)
	w.running(false)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r := items[len(items)-1]; r.State != "running" || r.Progress != "?" || r.Host != "desk" {
		t.Fatalf("running item = %+v", r)
	}
	var out bytes.Buffer
	Print(&out, nil, w.now)
	if !strings.Contains(out.String(), "No issue is waiting.") {
		t.Fatal(out.String())
	}
}

func TestDetailOfARunningIssue(t *testing.T) {
	w := newWorld(t)
	w.running(true)
	ref, _ := w.b.ParseRef("gh:o/r#30")
	d, err := w.b.Detail(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if d.State != "running" || d.Waiting || d.Host != "desk" || !strings.Contains(d.Post, "テストを直す") || strings.Contains(d.Post, "<!-- switchboard") {
		t.Fatalf("detail = %+v", d)
	}
	if err := w.b.Reply(ctx, ref, "hi", false); err == nil {
		t.Fatal("a reply to a running issue was posted")
	}

	w2 := newWorld(t)
	w2.running(false)
	d, err = w2.b.Detail(ctx, ref)
	if err != nil || d.State != "running" || !strings.Contains(d.Post, "Started on host desk.") {
		t.Fatalf("detail without progress = %+v, %v", d, err)
	}
}
