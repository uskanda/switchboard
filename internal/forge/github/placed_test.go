package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

const placedBody = "❓ **Q1** - **Layout**\n\nWhich layout?\n\n<!-- switchboard-image:1 -->\n\nA: one column\nB: two columns\n<!-- switchboard-image:2 -->\n\n<!-- switchboard v=1 state=waiting -->\n"

// placedFiles writes now.png and b.png, the images of placedBody's two marks.
func placedFiles(t *testing.T) []forge.PlacedImage {
	t.Helper()
	dir := t.TempDir()
	var out []forge.PlacedImage
	for i, n := range []string{"now.png", "b.png"} {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("img"), 0o600); err != nil {
			t.Fatal(err)
		}
		out = append(out, forge.PlacedImage{Mark: "<!-- switchboard-image:" + string(rune('1'+i)) + " -->", Path: p})
	}
	return out
}

// attachedAs is the body gh leaves: what it was given, and the images after it.
func attachedAs(names ...string) string {
	body := placedBody
	for i, n := range names {
		body += "\n![" + n + "](https://github.com/user-attachments/assets/0000000" + string(rune('1'+i)) + "-aaaa-bbbb-cccc-dddddddddddd)"
	}
	return body
}

// placedServer answers the read of comment 77 with body, and counts the edits and the deletions, failing the first
// failEdits edits and every deletion when failDelete.
func placedServer(t *testing.T, body string, failEdits int, failDelete bool) (*server, *Client, *string, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	s, c := newServer(t)
	c.gh = fakeGH(t, t.TempDir(), "https://github.com/o/r/issues/12#issuecomment-77", 0)
	s.on("GET /repos/o/r/issues/comments/77", jsonReply(map[string]any{"id": 77, "body": body, "user": map[string]any{"login": "agent", "id": 2}}))
	var edited string
	var edits, deletes atomic.Int32
	s.on("PATCH /repos/o/r/issues/comments/77", func(w http.ResponseWriter, r *http.Request) {
		if int(edits.Add(1)) <= failEdits {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		var in struct{ Body string }
		s.mu.Lock()
		_ = json.Unmarshal([]byte(s.bodies[len(s.bodies)-1]), &in)
		s.mu.Unlock()
		edited = in.Body
		_, _ = w.Write([]byte(`{}`))
	})
	s.on("DELETE /repos/o/r/issues/comments/77", func(w http.ResponseWriter, r *http.Request) {
		deletes.Add(1)
		if failDelete {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return s, c, &edited, &edits, &deletes
}

func TestAttachPlacedMovesTheImagesToTheirMarks(t *testing.T) {
	_, c, edited, _, deletes := placedServer(t, attachedAs("now", "b"), 0, false)
	images := placedFiles(t)
	got, err := c.AttachPlaced(ctx, ref, placedBody, images)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 77 {
		t.Fatalf("comment = %+v", got)
	}
	want := "❓ **Q1** - **Layout**\n\nWhich layout?\n\n![now](https://github.com/user-attachments/assets/00000001-aaaa-bbbb-cccc-dddddddddddd)\n\nA: one column\nB: two columns\n![b](https://github.com/user-attachments/assets/00000002-aaaa-bbbb-cccc-dddddddddddd)\n\n<!-- switchboard v=1 state=waiting -->\n"
	if *edited != want || got.Body != want {
		t.Fatalf("edited =\n%s\nwant\n%s", *edited, want)
	}
	if deletes.Load() != 0 {
		t.Fatal("the comment was deleted")
	}
	args := lines(t, filepath.Join(filepath.Dir(c.gh), "args"))
	if !slices.Equal(args[len(args)-4:], []string{"--attach", images[0].Path, "--attach", images[1].Path}) {
		t.Fatalf("args = %q", args)
	}
}

func TestAttachPlacedTriesTheEditTwice(t *testing.T) {
	_, c, edited, edits, deletes := placedServer(t, attachedAs("now", "b"), 1, false)
	if _, err := c.AttachPlaced(ctx, ref, placedBody, placedFiles(t)); err != nil {
		t.Fatal(err)
	}
	if edits.Load() != 2 || deletes.Load() != 0 || !strings.Contains(*edited, "![now]") {
		t.Fatalf("edits = %d, deletes = %d", edits.Load(), deletes.Load())
	}
}

func TestAttachPlacedDeletesTheCommentWhenTheEditFailsTwice(t *testing.T) {
	_, c, _, edits, deletes := placedServer(t, attachedAs("now", "b"), 2, false)
	_, err := c.AttachPlaced(ctx, ref, placedBody, placedFiles(t))
	var pe *forge.PlacedError
	if !errors.As(err, &pe) || pe.Left {
		t.Fatalf("err = %#v", err)
	}
	if edits.Load() != 2 || deletes.Load() != 1 {
		t.Fatalf("edits = %d, deletes = %d", edits.Load(), deletes.Load())
	}
}

func TestAttachPlacedDeletesTheCommentWhenTheNamesDoNotMatch(t *testing.T) {
	for name, body := range map[string]string{
		"other names": attachedAs("now", "c"),
		"one missing": attachedAs("now"),
	} {
		t.Run(name, func(t *testing.T) {
			_, c, _, edits, deletes := placedServer(t, body, 0, false)
			_, err := c.AttachPlaced(ctx, ref, placedBody, placedFiles(t))
			var pe *forge.PlacedError
			if !errors.As(err, &pe) || pe.Left || edits.Load() != 0 || deletes.Load() != 1 {
				t.Fatalf("err = %v, edits = %d, deletes = %d", err, edits.Load(), deletes.Load())
			}
		})
	}
}

func TestAttachPlacedSaysTheCommentIsLeftWhenItCannotBeDeleted(t *testing.T) {
	_, c, _, _, _ := placedServer(t, attachedAs("now", "b"), 2, true)
	_, err := c.AttachPlaced(ctx, ref, placedBody, placedFiles(t))
	var pe *forge.PlacedError
	if !errors.As(err, &pe) || !pe.Left {
		t.Fatalf("err = %#v", err)
	}
}

func TestAttachPlacedReportsGHFailing(t *testing.T) {
	_, c := newServer(t)
	c.gh = fakeGH(t, t.TempDir(), "unknown flag: --attach", 1)
	_, err := c.AttachPlaced(ctx, ref, placedBody, placedFiles(t))
	var pe *forge.PlacedError
	if !errors.As(err, &pe) || pe.Left || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("err = %#v", err)
	}
}
