package github

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestCommitChecks(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/actions/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("head_sha") != "abc" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		jsonReply(map[string]any{"workflow_runs": []map[string]any{
			{"id": 1, "name": "CI", "status": "completed"},
			{"id": 2, "name": "Lint", "status": "queued", "html_url": "https://github.test/o/r/actions/runs/2"},
		}})(w, r)
	})
	s.on("GET /repos/o/r/actions/runs/1/jobs", jsonReply(map[string]any{"jobs": []map[string]any{
		{"id": 11, "name": "test", "status": "completed", "conclusion": "failure", "html_url": "https://github.test/j/11",
			"steps": []map[string]any{{"name": "checkout", "conclusion": "success"}, {"name": "go test", "conclusion": "failure"}}},
		{"id": 12, "name": "build", "status": "in_progress", "conclusion": nil},
	}}))
	s.on("GET /repos/o/r/actions/runs/2/jobs", jsonReply(map[string]any{"jobs": []any{}}))
	s.on("GET /repos/o/r/commits/abc/status", jsonReply(map[string]any{"statuses": []map[string]any{
		{"context": "ext/ci", "state": "error", "description": "boom", "target_url": "https://ci.test/1"},
		{"context": "ext/lint", "state": "pending"},
	}}))
	checks, err := c.CommitChecks(ctx, "o", "r", "abc")
	if err != nil {
		t.Fatal(err)
	}
	want := []forge.Check{
		{Name: "CI / test", Done: true, Conclusion: "failure", URL: "https://github.test/j/11", FailedSteps: []string{"go test"}, Job: 11},
		{Name: "CI / build", Job: 12},
		{Name: "Lint", URL: "https://github.test/o/r/actions/runs/2"},
		{Name: "ext/ci", Done: true, Conclusion: "failure", URL: "https://ci.test/1", Description: "boom"},
		{Name: "ext/lint"},
	}
	if len(checks) != len(want) {
		t.Fatalf("checks = %+v", checks)
	}
	for i := range want {
		g, w := checks[i], want[i]
		if g.Name != w.Name || g.Done != w.Done || g.Conclusion != w.Conclusion || g.URL != w.URL || g.Description != w.Description ||
			g.Job != w.Job || len(g.FailedSteps) != len(w.FailedSteps) || (len(w.FailedSteps) > 0 && g.FailedSteps[0] != w.FailedSteps[0]) {
			t.Errorf("checks[%d] = %+v, want %+v", i, g, w)
		}
	}
}

func TestCommitChecksForbidden(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/actions/runs", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Resource not accessible by personal access token"}`, http.StatusForbidden)
	})
	if _, err := c.CommitChecks(ctx, "o", "r", "abc"); !errors.Is(err, forge.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestCheckLog(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/actions/jobs/11/logs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("2026-11-02T00:00:00.0000000Z --- FAIL: TestX\n"))
	})
	log, err := c.CheckLog(ctx, "o", "r", forge.Check{Job: 11})
	if err != nil || log != "2026-11-02T00:00:00.0000000Z --- FAIL: TestX\n" {
		t.Fatalf("CheckLog = %q, %v", log, err)
	}
	if log, err := c.CheckLog(ctx, "o", "r", forge.Check{}); err != nil || log != "" {
		t.Fatalf("CheckLog of a status = %q, %v, want nothing", log, err)
	}
}

func TestChangeRequestHead(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/pulls/48", jsonReply(map[string]any{"state": "closed", "merged": true, "head": map[string]any{"sha": "def"}}))
	s.on("GET /repos/o/r/pulls/49", jsonReply(map[string]any{"state": "open", "merged": false, "head": map[string]any{"sha": "abc"}}))
	st, err := c.ChangeRequestHead(ctx, "o", "r", 48)
	if err != nil || st.State != forge.CRMerged || st.Head != "def" {
		t.Fatalf("ChangeRequestHead(48) = %+v, %v", st, err)
	}
	st, err = c.ChangeRequestHead(ctx, "o", "r", 49)
	if err != nil || st.State != forge.CROpen || st.Head != "abc" || st.Conflict {
		t.Fatalf("ChangeRequestHead(49) = %+v, %v", st, err)
	}
}

func TestChangeRequestConflict(t *testing.T) {
	s, c := newServer(t)
	cases := map[int]struct {
		mergeable any
		state     string
		want      bool
	}{
		50: {false, "dirty", true},
		51: {nil, "unknown", false},
		52: {true, "behind", false},
		53: {false, "blocked", false},
		54: {true, "clean", false},
	}
	for n, tc := range cases {
		s.on(fmt.Sprintf("GET /repos/o/r/pulls/%d", n), jsonReply(map[string]any{
			"state": "open", "head": map[string]any{"sha": "abc"}, "mergeable": tc.mergeable, "mergeable_state": tc.state,
		}))
	}
	for n, tc := range cases {
		st, err := c.ChangeRequestHead(ctx, "o", "r", n)
		if err != nil || st.Conflict != tc.want {
			t.Errorf("ChangeRequestHead(%d) with mergeable %v, %s = %+v, %v, want Conflict %v", n, tc.mergeable, tc.state, st, err, tc.want)
		}
	}
}
