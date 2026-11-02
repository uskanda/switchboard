package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func pull(draft bool, sha string) http.HandlerFunc {
	return jsonReply(map[string]any{"number": 5, "state": "open", "merged": false, "draft": draft, "node_id": "PR_n5",
		"head": map[string]any{"sha": sha}})
}

func TestMergeChangeRequestMarksADraftReadyThenMergesWithTheSHA(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/pulls/5", pull(true, "abc"))
	s.on("POST /graphql", jsonReply(map[string]any{"data": map[string]any{"markPullRequestReadyForReview": map[string]any{
		"pullRequest": map[string]any{"isDraft": false}}}}))
	s.on("PUT /repos/o/r/pulls/5/merge", jsonReply(map[string]any{"merged": true, "sha": "m1"}))
	if err := c.MergeChangeRequest(ctx, "o", "r", 5, "abc"); err != nil {
		t.Fatal(err)
	}
	var routes []string
	for _, r := range s.log {
		routes = append(routes, r.Method+" "+r.URL.Path)
	}
	if got := strings.Join(routes, ","); got != "GET /repos/o/r/pulls/5,POST /graphql,PUT /repos/o/r/pulls/5/merge" {
		t.Fatalf("requests = %s", got)
	}
	var gql struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal([]byte(s.bodies[1]), &gql); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gql.Query, "markPullRequestReadyForReview") || gql.Variables["id"] != "PR_n5" {
		t.Errorf("graphql = %s", s.bodies[1])
	}
	var put map[string]any
	if err := json.Unmarshal([]byte(s.bodies[2]), &put); err != nil {
		t.Fatal(err)
	}
	if put["sha"] != "abc" || put["merge_method"] != nil {
		t.Errorf("merge body = %s", s.bodies[2])
	}
}

func TestMergeChangeRequestSkipsTheReadyStepForAReadyPull(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/pulls/5", pull(false, "abc"))
	s.on("PUT /repos/o/r/pulls/5/merge", jsonReply(map[string]any{"merged": true}))
	if err := c.MergeChangeRequest(ctx, "o", "r", 5, "abc"); err != nil {
		t.Fatal(err)
	}
	if len(s.log) != 2 {
		t.Fatalf("%d requests, want 2", len(s.log))
	}
}

func TestMergeChangeRequestRefusesAMovedHeadBeforeTouchingTheDraft(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/pulls/5", pull(true, "def"))
	if err := c.MergeChangeRequest(ctx, "o", "r", 5, "abc"); !errors.Is(err, forge.ErrHeadMoved) {
		t.Fatalf("err = %v, want ErrHeadMoved", err)
	}
	if len(s.log) != 1 {
		t.Fatalf("%d requests, want only the read", len(s.log))
	}
}

func TestMergeChangeRequestRefusesAMergedPull(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/pulls/5", jsonReply(map[string]any{"state": "closed", "merged": true, "head": map[string]any{"sha": "abc"}}))
	var nm *forge.NotMergeableError
	if err := c.MergeChangeRequest(ctx, "o", "r", 5, "abc"); !errors.As(err, &nm) || !strings.Contains(nm.Reason, "already merged") {
		t.Fatalf("err = %v, want a NotMergeableError saying it is merged", err)
	}
}

func TestMergeChangeRequestMapsTheMergeResponses(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
	}{
		{http.StatusConflict, forge.ErrHeadMoved},
		{http.StatusMethodNotAllowed, forge.ErrNotMergeable},
		{http.StatusUnprocessableEntity, forge.ErrNotMergeable},
		{http.StatusForbidden, forge.ErrForbidden},
	} {
		s, c := newServer(t)
		s.on("GET /repos/o/r/pulls/5", pull(false, "abc"))
		s.on("PUT /repos/o/r/pulls/5/merge", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"message":"Required status check \"test\" is expected."}`))
		})
		err := c.MergeChangeRequest(ctx, "o", "r", 5, "abc")
		if !errors.Is(err, tc.want) {
			t.Errorf("%d: err = %v, want %v", tc.status, err, tc.want)
		}
		var nm *forge.NotMergeableError
		if tc.want == forge.ErrNotMergeable && (!errors.As(err, &nm) || !strings.Contains(nm.Reason, "Required status check")) {
			t.Errorf("%d: reason missing from %v", tc.status, err)
		}
	}
}

func TestMergeChangeRequestReportsAGraphQLError(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/pulls/5", pull(true, "abc"))
	s.on("POST /graphql", jsonReply(map[string]any{"errors": []map[string]any{{"type": "FORBIDDEN", "message": "Resource not accessible by personal access token"}}}))
	err := c.MergeChangeRequest(ctx, "o", "r", 5, "abc")
	if !errors.Is(err, forge.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if len(s.log) != 2 {
		t.Fatalf("%d requests, want no merge after the failed ready step", len(s.log))
	}
}
