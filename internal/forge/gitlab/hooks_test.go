package gitlab

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestCreateHookWithASigningToken(t *testing.T) {
	s, c := newServer(t)
	var got map[string]any
	s.on("POST "+proj+"/hooks", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		_ = json.Unmarshal([]byte(s.bodies[len(s.bodies)-1]), &got)
		s.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":5,"signing_token_present":true}`))
	})
	id, err := c.CreateHook(ctx, "nas/tools", "sb", forge.Hook{URL: "http://127.0.0.1:8781/webhook/gitlab/k", Secret: "whsec_abc"})
	if err != nil || id != 5 {
		t.Fatalf("CreateHook = %d, %v", id, err)
	}
	for k, want := range map[string]any{
		"url": "http://127.0.0.1:8781/webhook/gitlab/k", "signing_token": "whsec_abc",
		"issues_events": true, "confidential_issues_events": true, "note_events": true, "confidential_note_events": true,
		"push_events": false, "merge_requests_events": false, "enable_ssl_verification": true,
	} {
		if got[k] != want {
			t.Errorf("%s = %v, want %v", k, got[k], want)
		}
	}
	if _, ok := got["token"]; ok {
		t.Error("a secret token was sent with a signing token")
	}
}

func TestCreateHookWithASecretToken(t *testing.T) {
	s, c := newServer(t)
	var got map[string]any
	s.on("POST "+proj+"/hooks", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		_ = json.Unmarshal([]byte(s.bodies[len(s.bodies)-1]), &got)
		s.mu.Unlock()
		_, _ = w.Write([]byte(`{"id":6}`))
	})
	if _, err := c.CreateHook(ctx, "nas/tools", "sb", forge.Hook{URL: "u", Secret: "plain", SecretToken: true}); err != nil {
		t.Fatal(err)
	}
	if got["token"] != "plain" || got["signing_token"] != nil {
		t.Fatalf("hook = %v", got)
	}
}

func TestCreateHookWithoutTheRightIsForbidden(t *testing.T) {
	s, c := newServer(t)
	s.on("POST "+proj+"/hooks", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"403 Forbidden"}`, http.StatusForbidden)
	})
	if _, err := c.CreateHook(ctx, "nas/tools", "sb", forge.Hook{URL: "u", Secret: "s"}); !errors.Is(err, forge.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteHookFindsItByURL(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/hooks", jsonReply([]map[string]any{{"id": 1, "url": "http://other"}, {"id": 2, "url": "http://127.0.0.1:8781/webhook/gitlab/k"}}))
	deleted := false
	s.on("DELETE "+proj+"/hooks/2", func(w http.ResponseWriter, _ *http.Request) { deleted = true; w.WriteHeader(http.StatusNoContent) })
	found, err := c.DeleteHook(ctx, "nas/tools", "sb", "http://127.0.0.1:8781/webhook/gitlab/k")
	if err != nil || !found || !deleted {
		t.Fatalf("DeleteHook = %v, %v, deleted %v", found, err, deleted)
	}
}

var _ forge.Hooks = (*Client)(nil)
