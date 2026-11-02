package gitlab

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func mr(draft bool, title, sha string) http.HandlerFunc {
	return jsonReply(map[string]any{"iid": 9, "state": "opened", "draft": draft, "title": title, "sha": sha})
}

func TestMergeChangeRequestTakesDraftOffTheTitleThenMergesWithTheSHA(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/merge_requests/9", mr(true, "Draft: Add a flag", "abc"))
	s.on("PUT "+proj+"/merge_requests/9", jsonReply(map[string]any{"iid": 9}))
	s.on("PUT "+proj+"/merge_requests/9/merge", jsonReply(map[string]any{"state": "merged"}))
	if err := c.MergeChangeRequest(ctx, "nas/tools", "sb", 9, "abc"); err != nil {
		t.Fatal(err)
	}
	var routes []string
	for _, r := range s.log {
		routes = append(routes, r.Method+" "+r.URL.EscapedPath())
	}
	want := "GET " + proj + "/merge_requests/9,PUT " + proj + "/merge_requests/9,PUT " + proj + "/merge_requests/9/merge"
	if got := strings.Join(routes, ","); got != want {
		t.Fatalf("requests = %s", got)
	}
	if b := s.body(1); b["title"] != "Add a flag" {
		t.Errorf("title update = %v", b)
	}
	if b := s.body(2); b["sha"] != "abc" || b["squash"] != nil || b["should_remove_source_branch"] != nil {
		t.Errorf("merge body = %v", b)
	}
}

func TestUndraftedTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Draft: Add a flag":   "Add a flag",
		"draft:Add a flag":    "Add a flag",
		"[Draft] Add a flag":  "Add a flag",
		"(Draft) Add a flag":  "Add a flag",
		"Add a flag":          "Add a flag",
		"Drafting: the guide": "Drafting: the guide",
	} {
		if got := undrafted(in); got != want {
			t.Errorf("undrafted(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMergeChangeRequestSkipsTheTitleForAReadyRequest(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/merge_requests/9", mr(false, "Add a flag", "abc"))
	s.on("PUT "+proj+"/merge_requests/9/merge", jsonReply(map[string]any{"state": "merged"}))
	if err := c.MergeChangeRequest(ctx, "nas/tools", "sb", 9, "abc"); err != nil {
		t.Fatal(err)
	}
	if len(s.log) != 2 {
		t.Fatalf("%d requests, want 2", len(s.log))
	}
}

func TestMergeChangeRequestRefusesAMovedHeadBeforeTouchingTheTitle(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/merge_requests/9", mr(true, "Draft: Add a flag", "def"))
	if err := c.MergeChangeRequest(ctx, "nas/tools", "sb", 9, "abc"); !errors.Is(err, forge.ErrHeadMoved) {
		t.Fatalf("err = %v, want ErrHeadMoved", err)
	}
	if len(s.log) != 1 {
		t.Fatalf("%d requests, want only the read", len(s.log))
	}
}

func TestMergeChangeRequestRefusesAMergedRequest(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/merge_requests/9", jsonReply(map[string]any{"iid": 9, "state": "merged", "sha": "abc"}))
	var nm *forge.NotMergeableError
	if err := c.MergeChangeRequest(ctx, "nas/tools", "sb", 9, "abc"); !errors.As(err, &nm) || !strings.Contains(nm.Reason, "already merged") {
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
		{http.StatusNotAcceptable, forge.ErrNotMergeable},
		{http.StatusUnprocessableEntity, forge.ErrNotMergeable},
		{http.StatusForbidden, forge.ErrForbidden},
	} {
		s, c := newServer(t)
		s.on("GET "+proj+"/merge_requests/9", mr(false, "Add a flag", "abc"))
		s.on("PUT "+proj+"/merge_requests/9/merge", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"message":"Pipeline must succeed"}`))
		})
		err := c.MergeChangeRequest(ctx, "nas/tools", "sb", 9, "abc")
		if !errors.Is(err, tc.want) {
			t.Errorf("%d: err = %v, want %v", tc.status, err, tc.want)
		}
		var nm *forge.NotMergeableError
		if tc.want == forge.ErrNotMergeable && (!errors.As(err, &nm) || nm.Reason != "Pipeline must succeed") {
			t.Errorf("%d: reason missing from %v", tc.status, err)
		}
	}
}
