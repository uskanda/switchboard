package gitlab

import (
	"net/http"
	"reflect"
	"testing"
)

func TestTokenScopes(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /api/v4/personal_access_tokens/self", jsonReply(map[string]any{"id": 4, "scopes": []string{"api", "read_user"}}))
	ts, err := c.TokenScopes(ctx)
	if err != nil || !ts.Known || !reflect.DeepEqual(ts.Scopes, []string{"api", "read_user"}) {
		t.Fatalf("%+v, %v", ts, err)
	}
}

func TestProbeReads(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues", jsonReply([]any{}))
	s.on("GET "+proj+"/repository/branches/main", jsonReply(map[string]any{"name": "main"}))
	got := c.ProbeReads(ctx, "nas/tools", "sb", "main")
	if len(got) != 3 || got[0].Err != nil || got[1].Err == nil || got[2].Err != nil {
		t.Fatalf("probes = %+v", got)
	}
	for _, r := range s.log {
		if r.Method != http.MethodGet {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
	}
}
