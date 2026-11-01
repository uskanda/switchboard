package github

import (
	"net/http"
	"testing"
)

func TestFindOpenChangeRequestAsksForTheOpenPullOfTheBranch(t *testing.T) {
	s, c := newServer(t)
	open := true
	s.on("GET /repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("head") != "o:switchboard/issue-3" || q.Get("state") != "open" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if !open {
			jsonReply([]map[string]any{})(w, r)
			return
		}
		jsonReply([]map[string]any{{"html_url": "https://github.test/o/r/pull/5", "number": 5}})(w, r)
	})
	u, found, err := c.FindOpenChangeRequest(ctx, "o", "r", "switchboard/issue-3")
	if err != nil || !found || u != "https://github.test/o/r/pull/5" {
		t.Fatalf("FindOpenChangeRequest = %q, %v, %v", u, found, err)
	}
	open = false
	u, found, err = c.FindOpenChangeRequest(ctx, "o", "r", "switchboard/issue-3")
	if err != nil || found || u != "" {
		t.Fatalf("FindOpenChangeRequest without a pull = %q, %v, %v", u, found, err)
	}
	s.on("GET /repos/o/r/pulls", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	})
	if _, _, err := c.FindOpenChangeRequest(ctx, "o", "r", "switchboard/issue-3"); err == nil {
		t.Fatal("FindOpenChangeRequest on a failing forge: no error")
	}
}
