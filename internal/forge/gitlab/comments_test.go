package gitlab

import (
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

var ref3 = forge.IssueRef{Forge: forge.GitLab, Owner: "nas/tools", Repo: "sb", Number: 3}

// Comments returns the notes people and bots wrote, without the system notes, and reads nothing else.
func TestCommentsSkipsSystemNotes(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues/3/notes", jsonReply([]map[string]any{
		{"id": 1, "body": "changed the description", "system": true, "author": map[string]any{"id": 9, "username": "alice"}},
		{"id": 2, "body": "hello", "author": map[string]any{"id": 9, "username": "alice"}, "created_at": "2026-10-29T00:10:00Z"},
	}))
	got, err := c.Comments(ctx, ref3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 2 || got[0].Body != "hello" || got[0].Author.Login != "alice" {
		t.Fatalf("comments = %+v", got)
	}
	if len(s.log) != 1 {
		t.Errorf("requests = %d, want the notes alone", len(s.log))
	}
}

// A 429 is a *forge.RateLimitError with the time GitLab names in RateLimit-Reset, else in Retry-After.
func TestCommentsRateLimited(t *testing.T) {
	reset := time.Date(2026, 11, 14, 10, 21, 5, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		header map[string]string
		lo, hi time.Duration // from now, when no fixed reset is expected
	}{
		{"RateLimit-Reset", map[string]string{"RateLimit-Reset": strconv.FormatInt(reset.Unix(), 10)}, 0, 0},
		{"Retry-After", map[string]string{"Retry-After": "90"}, 89 * time.Second, 92 * time.Second},
		{"no header", nil, 59 * time.Second, 62 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, c := newServer(t)
			s.on("GET "+proj+"/issues/3/notes", func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tc.header {
					w.Header().Set(k, v)
				}
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"message":"Retry later"}`))
			})
			now := time.Now()
			_, err := c.Comments(ctx, ref3)
			var rl *forge.RateLimitError
			if !errors.As(err, &rl) {
				t.Fatalf("err = %v, want a *forge.RateLimitError", err)
			}
			if tc.hi == 0 {
				if !rl.Reset.Equal(reset) {
					t.Errorf("reset = %v, want %v", rl.Reset, reset)
				}
				return
			}
			if rl.Reset.Before(now.Add(tc.lo)) || rl.Reset.After(now.Add(tc.hi)) {
				t.Errorf("reset = %v, want %v to %v from now", rl.Reset, tc.lo, tc.hi)
			}
		})
	}
}
