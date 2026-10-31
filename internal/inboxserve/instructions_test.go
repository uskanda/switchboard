package inboxserve

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/instructions"
)

type events struct{ names []string }

func (e *events) Log(event string, _ map[string]any) { e.names = append(e.names, event) }

func withInstructions(t *testing.T) (*world, instructions.Store, *events) {
	t.Helper()
	w := newWorld(t)
	ev := &events{}
	s := instructions.Store{Dir: filepath.Join(t.TempDir(), "instructions"), Audit: ev}
	w.srv.Instructions = &Instructions{Store: s, Projects: []string{"gh:o/r", "gl:nas/sb"}, Host: "here"}
	return w, s, ev
}

func TestInstructionsListsGlobalAndEveryProject(t *testing.T) {
	w, s, _ := withInstructions(t)
	if err := s.Save("gh:o/r", "Mind the API names."); err != nil {
		t.Fatal(err)
	}
	got, err := w.c.Instructions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "here" || len(got.Items) != 3 {
		t.Fatalf("instructions = %+v", got)
	}
	want := []inboxapi.Instruction{{Target: "", Content: ""}, {Target: "gh:o/r", Content: "Mind the API names."}, {Target: "gl:nas/sb", Content: ""}}
	for i, it := range got.Items {
		if it != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, it, want[i])
		}
	}
}

func TestSaveInstructions(t *testing.T) {
	w, s, ev := withInstructions(t)
	if err := w.c.SaveInstructions(ctx, "gl:nas/sb", "Ask in Japanese."); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Read("gl:nas/sb"); got != "Ask in Japanese." {
		t.Fatalf("file = %q", got)
	}
	if err := w.c.SaveInstructions(ctx, "", "Short questions."); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Read(instructions.Global); got != "Short questions." {
		t.Fatalf("global file = %q", got)
	}
	if len(ev.names) != 2 || ev.names[0] != "instructions.saved" {
		t.Fatalf("audit = %v", ev.names)
	}
}

func TestSaveInstructionsRefusals(t *testing.T) {
	w, s, _ := withInstructions(t)
	for _, c := range []struct {
		target, content string
		want            error
	}{
		{"gh:o/r", strings.Repeat("x", 40<<10), inboxapi.ErrTooLarge},
		{"gh:o/r", "use ghp_" + strings.Repeat("Zx9", 12), inboxapi.ErrSecret},
		{"gh:someone/other", "x", inboxapi.ErrUnknownProject},
	} {
		if err := w.c.SaveInstructions(ctx, c.target, c.content); !errors.Is(err, c.want) {
			t.Errorf("SaveInstructions(%s, %d bytes) = %v, want %v", c.target, len(c.content), err, c.want)
		}
	}
	if got, _ := s.Read("gh:o/r"); got != "" {
		t.Fatalf("a refused save wrote %q", got)
	}
}

func TestInstructionsWithoutAStoreAreNotServed(t *testing.T) {
	w := newWorld(t)
	if _, err := w.c.Instructions(ctx); !errors.Is(err, inboxapi.ErrNotFound) {
		t.Fatalf("Instructions = %v", err)
	}
}
