package github

import (
	"net/http"
	"reflect"
	"testing"
)

func TestTokenScopesOfAClassicAndAFineGrainedToken(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-OAuth-Scopes", "repo, read:org")
		jsonReply(map[string]any{"login": "me"})(w, r)
	})
	ts, err := c.TokenScopes(ctx)
	if err != nil || !ts.Known || !reflect.DeepEqual(ts.Scopes, []string{"repo", "read:org"}) {
		t.Fatalf("classic: %+v, %v", ts, err)
	}
	s.on("GET /user", jsonReply(map[string]any{"login": "me"}))
	if ts, err := c.TokenScopes(ctx); err != nil || ts.Known {
		t.Fatalf("fine-grained: %+v, %v", ts, err)
	}
}

func TestProbeReadsOnlyReads(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/issues", jsonReply([]any{}))
	s.on("GET /repos/o/r/pulls", jsonReply([]any{}))
	got := c.ProbeReads(ctx, "o", "r", "main")
	if len(got) != 3 || got[0].Err != nil || got[1].Err != nil || got[2].Err == nil || got[2].What != "contents" {
		t.Fatalf("probes = %+v", got)
	}
	for _, r := range s.log {
		if r.Method != http.MethodGet {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
	}
}
