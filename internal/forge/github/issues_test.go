package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestCreateIssueSendsTheTitleBodyAndLabels(t *testing.T) {
	s, c := newServer(t)
	s.on("POST /repos/o/r/issues", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"number":41,"title":"Add a flag","state":"open","html_url":"https://github.test/o/r/issues/41",
			"labels":[{"name":"agent:ready"}]}`))
	})
	got, err := c.CreateIssue(ctx, "o", "r", "Add a flag", "Use --x.", []string{"agent:ready"})
	if err != nil {
		t.Fatal(err)
	}
	want := forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: 41}
	if got.Ref != want || got.URL != "https://github.test/o/r/issues/41" || !slices.Equal(got.Labels, []string{"agent:ready"}) {
		t.Fatalf("CreateIssue = %+v", got)
	}
	var sent struct {
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(s.bodies[0]), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Title != "Add a flag" || sent.Body != "Use --x." || !slices.Equal(sent.Labels, []string{"agent:ready"}) {
		t.Errorf("sent %s", s.bodies[0])
	}
}

func TestCreateIssueInAMissingRepository(t *testing.T) {
	_, c := newServer(t)
	if _, err := c.CreateIssue(ctx, "o", "nope", "t", "", nil); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
