package github

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// Comments reads the comments alone, every page, and nothing of the issue or its edits.
func TestCommentsReadsEveryPageAndNothingElse(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/issues/7/comments", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			jsonReply([]map[string]any{{"id": 12, "body": "second", "user": map[string]any{"login": "bob", "id": 2}, "created_at": "2026-10-29T00:20:00Z"}})(w, r)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, "http://"+r.Host+r.URL.Path))
		jsonReply([]map[string]any{{"id": 11, "body": "first", "user": map[string]any{"login": "alice", "id": 1}, "created_at": "2026-10-29T00:10:00Z"}})(w, r)
	})
	got, err := c.Comments(ctx, forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 11 || got[1].ID != 12 || got[1].Author.ID != 2 || got[0].Body != "first" {
		t.Fatalf("comments = %+v", got)
	}
	for _, r := range s.log {
		if r.URL.Path != "/repos/o/r/issues/7/comments" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}
}

// A second read sends the ETag of the first page; on 304 the comments of the last read come back, and no further
// page is asked for.
func TestCommentsUsesETag(t *testing.T) {
	s, c := newServer(t)
	calls := 0
	s.on("GET /repos/o/r/issues/7/comments", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			if r.Header.Get("If-None-Match") != `"c1"` {
				t.Errorf("second request without If-None-Match")
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"c1"`)
		jsonReply([]map[string]any{{"id": 11, "body": "first", "user": map[string]any{"login": "alice", "id": 1}}})(w, r)
	})
	ref := forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: 7}
	if _, err := c.Comments(ctx, ref); err != nil {
		t.Fatal(err)
	}
	got, err := c.Comments(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(got) != 1 || got[0].ID != 11 {
		t.Fatalf("calls = %d, comments = %+v, want the cached comment after 304", calls, got)
	}
}

// The search's comment count goes into Notes, so a reader can tell a new comment from the search alone.
func TestSearchFillsNotesFromTheCommentCount(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /search/issues", jsonReply(map[string]any{"items": []map[string]any{{
		"number": 3, "title": "t", "state": "open", "comments": 5, "repository_url": "https://api.github.com/repos/o/r",
	}}}))
	got, err := c.SearchOpenIssuesWithLabel(ctx, forge.Projects("o/r"), "agent:waiting")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Notes != 5 {
		t.Fatalf("summaries = %+v, want Notes 5", got)
	}
}

func rateLimited(status int, h map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range h {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded for user ID 1."}`))
	}
}

// A rate-limit answer is a *forge.RateLimitError with the time the limit resets; a 403 still reads as forbidden.
func TestRateLimitAnswers(t *testing.T) {
	reset := time.Date(2026, 11, 14, 10, 21, 5, 0, time.UTC)
	ref := forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: 7}
	for _, tc := range []struct {
		name      string
		status    int
		header    map[string]string
		want      func(now time.Time) (lo, hi time.Time)
		forbidden bool
	}{
		{"primary", 403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(reset.Unix(), 10)},
			func(time.Time) (time.Time, time.Time) { return reset, reset }, true},
		{"secondary", 403, map[string]string{"Retry-After": "120"},
			func(now time.Time) (time.Time, time.Time) {
				return now.Add(119 * time.Second), now.Add(122 * time.Second)
			}, true},
		{"no header 429", 429, nil,
			func(now time.Time) (time.Time, time.Time) {
				return now.Add(59 * time.Second), now.Add(62 * time.Second)
			}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, c := newServer(t)
			s.on("GET /repos/o/r/issues/7/comments", rateLimited(tc.status, tc.header))
			now := time.Now()
			_, err := c.Comments(ctx, ref)
			var rl *forge.RateLimitError
			if !errors.As(err, &rl) {
				t.Fatalf("err = %v, want a *forge.RateLimitError", err)
			}
			lo, hi := tc.want(now)
			if rl.Reset.Before(lo) || rl.Reset.After(hi) {
				t.Errorf("reset = %v, want within [%v, %v]", rl.Reset, lo, hi)
			}
			if got := errors.Is(err, forge.ErrForbidden); got != tc.forbidden {
				t.Errorf("errors.Is(err, ErrForbidden) = %v, want %v", got, tc.forbidden)
			}
		})
	}
}

// A 403 that is no rate limit stays a plain forbidden error.
func TestForbiddenIsNoRateLimit(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/issues/7/comments", rateLimited(403, map[string]string{"X-RateLimit-Remaining": "4000"}))
	_, err := c.Comments(ctx, forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: 7})
	var rl *forge.RateLimitError
	if errors.As(err, &rl) || !errors.Is(err, forge.ErrForbidden) {
		t.Fatalf("err = %v, want forbidden and no rate limit", err)
	}
}
