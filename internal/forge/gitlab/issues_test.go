package gitlab

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestCreateIssueSendsTheLabelsCommaSeparated(t *testing.T) {
	s, c := newServer(t)
	s.on("POST "+proj+"/issues", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"iid":7,"title":"Add a flag","state":"opened","web_url":"https://gl.test/nas/tools/sb/-/issues/7",
			"labels":["agent:plan","bug"]}`))
	})
	got, err := c.CreateIssue(ctx, "nas/tools", "sb", "Add a flag", "Use --x.", []string{"agent:plan", "bug"})
	if err != nil {
		t.Fatal(err)
	}
	want := forge.IssueRef{Forge: forge.GitLab, Owner: "nas/tools", Repo: "sb", Number: 7}
	if got.Ref != want || got.State != "open" || got.URL != "https://gl.test/nas/tools/sb/-/issues/7" ||
		!slices.Equal(got.Labels, []string{"agent:plan", "bug"}) {
		t.Fatalf("CreateIssue = %+v", got)
	}
	b := s.body(0)
	if b["title"] != "Add a flag" || b["description"] != "Use --x." || b["labels"] != "agent:plan,bug" {
		t.Errorf("sent %v", b)
	}
}

func TestCreateIssueInAMissingProject(t *testing.T) {
	_, c := newServer(t)
	if _, err := c.CreateIssue(ctx, "nas/tools", "sb", "t", "", nil); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
