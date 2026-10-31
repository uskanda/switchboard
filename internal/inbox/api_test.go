package inbox

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestCollectReportsEachUnreachableForgeAndKeepsNoRows(t *testing.T) {
	w := newMixed(t)
	w.b.Forges[forge.GitLab] = down{w.g}
	items, errs := w.b.Collect(ctx)
	if refs(items) != "gh:o/s#3 gh:o/r#12" {
		t.Fatalf("items = %s", refs(items))
	}
	if len(errs) != 1 || errs[0].Forge != "gitlab" || !errors.Is(errs[0].Err, errDown) {
		t.Fatalf("errs = %+v", errs)
	}
	if items[0].URL != "https://github.test/o/s/issues/3" {
		t.Fatalf("URL = %q", items[0].URL)
	}
	if _, err := os.Stat(w.b.Cache); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Collect wrote the row cache: %v", err)
	}
}

func TestDetail(t *testing.T) {
	w := newWorld(t)
	d, err := w.b.Detail(ctx, w.a)
	if err != nil {
		t.Fatal(err)
	}
	if d.Ref != "gh:o/r#12" || d.Title != "Add a flag" || d.URL != "https://github.test/o/r/issues/12" || !d.Waiting {
		t.Fatalf("detail = %+v", d)
	}
	if d.Host != "laptop" || d.Reason != "question" || d.Since == nil || !d.Since.Equal(w.now.Add(-3*time.Hour)) {
		t.Fatalf("marker fields = %+v", d)
	}
	if !strings.Contains(d.Post, "Which default should --dry-run have?") || strings.Contains(d.Post, "<!-- switchboard") {
		t.Fatalf("post:\n%s", d.Post)
	}
	w.f.RemoveLabel(w.a, "agent:waiting")
	if d, err := w.b.Detail(ctx, w.a); err != nil || d.Waiting {
		t.Fatalf("after the label is gone: %+v, %v", d, err)
	}
}

func TestReplyErrorsCanBeTold(t *testing.T) {
	w := newWorld(t)
	if err := w.b.Reply(ctx, w.a, "  ", false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty reply: %v", err)
	}
	w.f.RemoveLabel(w.a, "agent:waiting")
	if err := w.b.Reply(ctx, w.a, "Go with B.", false); !errors.Is(err, ErrNotWaiting) {
		t.Fatalf("not waiting: %v", err)
	}
	if _, err := w.b.ParseRef("gh:o/unlisted#1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unconfigured project: %v", err)
	}
	if _, err := w.b.ParseRef("1"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a row number: %v", err)
	}
}
