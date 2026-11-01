package gitlab

import (
	"net/http"
	"testing"
)

func TestFindOpenChangeRequestAsksForTheOpenMergeRequestOfTheBranch(t *testing.T) {
	s, c := newServer(t)
	open := true
	s.on("GET "+proj+"/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("source_branch") != "switchboard/issue-3" || q.Get("state") != "opened" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if !open {
			jsonReply([]map[string]any{})(w, r)
			return
		}
		jsonReply([]map[string]any{{"web_url": "https://gl.test/nas/tools/sb/-/merge_requests/4", "iid": 4}})(w, r)
	})
	u, found, err := c.FindOpenChangeRequest(ctx, "nas/tools", "sb", "switchboard/issue-3")
	if err != nil || !found || u != "https://gl.test/nas/tools/sb/-/merge_requests/4" {
		t.Fatalf("FindOpenChangeRequest = %q, %v, %v", u, found, err)
	}
	open = false
	u, found, err = c.FindOpenChangeRequest(ctx, "nas/tools", "sb", "switchboard/issue-3")
	if err != nil || found || u != "" {
		t.Fatalf("FindOpenChangeRequest without a merge request = %q, %v, %v", u, found, err)
	}
}
