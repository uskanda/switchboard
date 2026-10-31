package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestCreateHook(t *testing.T) {
	s, c := newServer(t)
	var got map[string]any
	s.on("POST /repos/o/r/hooks", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		_ = json.Unmarshal([]byte(s.bodies[len(s.bodies)-1]), &got)
		s.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":77}`))
	})
	id, err := c.CreateHook(ctx, "o", "r", forge.Hook{URL: "https://relay.example/webhook/github/k", Secret: "s3cret"})
	if err != nil || id != 77 {
		t.Fatalf("CreateHook = %d, %v", id, err)
	}
	cfg := got["config"].(map[string]any)
	if cfg["url"] != "https://relay.example/webhook/github/k" || cfg["secret"] != "s3cret" || cfg["content_type"] != "json" || cfg["insecure_ssl"] != "0" {
		t.Fatalf("config = %v", cfg)
	}
	ev := got["events"].([]any)
	if len(ev) != 2 || ev[0] != "issues" || ev[1] != "issue_comment" || got["active"] != true || got["name"] != "web" {
		t.Fatalf("hook = %v", got)
	}
}

func TestCreateHookWithoutTheRightIsForbidden(t *testing.T) {
	s, c := newServer(t)
	s.on("POST /repos/o/r/hooks", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Resource not accessible by personal access token"}`, http.StatusForbidden)
	})
	if _, err := c.CreateHook(ctx, "o", "r", forge.Hook{URL: "u", Secret: "s"}); !errors.Is(err, forge.ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
	// GitHub answers 404 to a token that cannot see the hooks at all.
	if _, err := c.CreateHook(ctx, "o", "other", forge.Hook{URL: "u", Secret: "s"}); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteHookFindsItByURL(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/hooks", jsonReply([]map[string]any{
		{"id": 1, "config": map[string]any{"url": "https://elsewhere/hook"}},
		{"id": 2, "config": map[string]any{"url": "https://relay.example/webhook/github/k"}},
	}))
	deleted := false
	s.on("DELETE /repos/o/r/hooks/2", func(w http.ResponseWriter, _ *http.Request) { deleted = true; w.WriteHeader(http.StatusNoContent) })
	found, err := c.DeleteHook(ctx, "o", "r", "https://relay.example/webhook/github/k")
	if err != nil || !found || !deleted {
		t.Fatalf("DeleteHook = %v, %v, deleted %v", found, err, deleted)
	}
	if found, err := c.DeleteHook(ctx, "o", "r", "https://nowhere"); err != nil || found {
		t.Fatalf("an unknown URL: %v, %v", found, err)
	}
}

var _ forge.Hooks = (*Client)(nil)
