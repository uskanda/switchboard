package gitlab

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestDefaultBranch(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":9,"visibility":"private","default_branch":"trunk"}`))
	})
	if b, err := c.DefaultBranch(ctx, "nas/tools", "sb"); err != nil || b != "trunk" {
		t.Fatalf("DefaultBranch = %q, %v; want trunk", b, err)
	}
}

func TestDefaultBranchOfAMissingProject(t *testing.T) {
	_, c := newServer(t)
	if _, err := c.DefaultBranch(ctx, "nas/tools", "sb"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestLabelNamesReadsEveryPage(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/labels", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			_, _ = w.Write([]byte(`[{"name":"agent:done"}]`))
			return
		}
		w.Header().Set("Link", `<http://`+r.Host+proj+`/labels?per_page=100&page=2>; rel="next"`)
		_, _ = w.Write([]byte(`[{"name":"bug"},{"name":"agent:ready"}]`))
	})
	got, err := c.LabelNames(ctx, "nas/tools", "sb")
	if err != nil || !slices.Equal(got, []string{"bug", "agent:ready", "agent:done"}) {
		t.Fatalf("LabelNames = %v, %v", got, err)
	}
}
