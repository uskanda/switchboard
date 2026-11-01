package github

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestDefaultBranch(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"default_branch":"trunk","private":true}`))
	})
	if b, err := c.DefaultBranch(ctx, "o", "r"); err != nil || b != "trunk" {
		t.Fatalf("DefaultBranch = %q, %v; want trunk", b, err)
	}
}

func TestDefaultBranchOfAMissingRepository(t *testing.T) {
	_, c := newServer(t)
	if _, err := c.DefaultBranch(ctx, "o", "nope"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestLabelNamesReadsEveryPage(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/labels", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`[{"name":"agent:done"}]`))
			return
		}
		w.Header().Set("Link", `<http://`+r.Host+`/repos/o/r/labels?per_page=100&page=2>; rel="next"`)
		_, _ = w.Write([]byte(`[{"name":"bug"},{"name":"agent:ready"}]`))
	})
	got, err := c.LabelNames(ctx, "o", "r")
	if err != nil || !slices.Equal(got, []string{"bug", "agent:ready", "agent:done"}) {
		t.Fatalf("LabelNames = %v, %v", got, err)
	}
}
