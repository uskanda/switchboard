package gitlab

import (
	"errors"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestCommitChecks(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/projects/nas%2Fx/pipelines", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sha") != "abc" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		jsonReply([]map[string]any{{"id": 5, "status": "failed"}, {"id": 6, "status": "created", "web_url": "https://gl.test/p/6"}})(w, r)
	})
	s.on("GET /api/v4/projects/nas%2Fx/pipelines/5/jobs", jsonReply([]map[string]any{
		{"id": 51, "name": "test", "stage": "test", "status": "failed", "web_url": "https://gl.test/j/51"},
		{"id": 52, "name": "slow", "stage": "test", "status": "failed", "failure_reason": "job_execution_timeout"},
		{"id": 53, "name": "lint", "stage": "test", "status": "failed", "allow_failure": true},
		{"id": 54, "name": "deploy", "stage": "deploy", "status": "manual"},
		{"id": 55, "name": "build", "stage": "build", "status": "running"},
		{"id": 56, "name": "ok", "stage": "build", "status": "success"},
		{"id": 57, "name": "gone", "stage": "build", "status": "canceled"},
	}))
	s.on("GET /api/v4/projects/nas%2Fx/pipelines/6/jobs", jsonReply([]any{}))
	checks, err := c.CommitChecks(ctx, "nas", "x", "abc")
	if err != nil {
		t.Fatal(err)
	}
	want := []forge.Check{
		{Name: "test", Done: true, Conclusion: forge.Failure, URL: "https://gl.test/j/51", Job: 51},
		{Name: "slow", Done: true, Conclusion: forge.TimedOut, Job: 52},
		{Name: "lint", Done: true, Conclusion: forge.Neutral, Job: 53},
		{Name: "deploy", Done: true, Conclusion: forge.Skipped, Job: 54},
		{Name: "build", Job: 55},
		{Name: "ok", Done: true, Conclusion: forge.Success, Job: 56},
		{Name: "gone", Done: true, Conclusion: forge.Cancelled, Job: 57},
		{Name: "pipeline 6", URL: "https://gl.test/p/6"},
	}
	if len(checks) != len(want) {
		t.Fatalf("checks = %+v", checks)
	}
	for i := range want {
		g, w := checks[i], want[i]
		if g.Name != w.Name || g.Done != w.Done || g.Conclusion != w.Conclusion || g.URL != w.URL || g.Job != w.Job {
			t.Errorf("checks[%d] = %+v, want %+v", i, g, w)
		}
	}
}

func TestCommitChecksForbidden(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/projects/nas%2Fx/pipelines", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
	})
	if _, err := c.CommitChecks(ctx, "nas", "x", "abc"); !errors.Is(err, forge.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestCheckLog(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/projects/nas%2Fx/jobs/51/trace", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("\x1b[31mFAIL\x1b[0m\n"))
	})
	log, err := c.CheckLog(ctx, "nas", "x", forge.Check{Job: 51})
	if err != nil || log != "\x1b[31mFAIL\x1b[0m\n" {
		t.Fatalf("CheckLog = %q, %v", log, err)
	}
}

func TestChangeRequestHead(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/projects/nas%2Fx/merge_requests/7", jsonReply(map[string]any{"state": "merged", "sha": "def"}))
	s.on("GET /api/v4/projects/nas%2Fx/merge_requests/8", jsonReply(map[string]any{"state": "opened", "sha": "abc"}))
	s.on("GET /api/v4/projects/nas%2Fx/merge_requests/9", jsonReply(map[string]any{"state": "closed", "sha": "abc"}))
	for n, want := range map[int]forge.ChangeRequestState{7: {State: forge.CRMerged, Head: "def"}, 8: {State: forge.CROpen, Head: "abc"}, 9: {State: forge.CRClosed, Head: "abc"}} {
		st, err := c.ChangeRequestHead(ctx, "nas", "x", n)
		if err != nil || *st != want {
			t.Errorf("ChangeRequestHead(%d) = %+v, %v, want %+v", n, st, err, want)
		}
	}
}

func TestChangeRequestConflict(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/projects/nas%2Fx/merge_requests/10", jsonReply(map[string]any{"state": "opened", "sha": "abc", "has_conflicts": true, "detailed_merge_status": "conflict"}))
	s.on("GET /api/v4/projects/nas%2Fx/merge_requests/11", jsonReply(map[string]any{"state": "opened", "sha": "abc", "has_conflicts": true, "detailed_merge_status": "checking"}))
	s.on("GET /api/v4/projects/nas%2Fx/merge_requests/12", jsonReply(map[string]any{"state": "opened", "sha": "abc", "has_conflicts": false, "detailed_merge_status": "mergeable"}))
	for n, want := range map[int]bool{10: true, 11: false, 12: false} {
		st, err := c.ChangeRequestHead(ctx, "nas", "x", n)
		if err != nil || st.Conflict != want {
			t.Errorf("ChangeRequestHead(%d) = %+v, %v, want Conflict %v", n, st, err, want)
		}
	}
}
