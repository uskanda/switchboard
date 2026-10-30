package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// server is a scripted GitHub: handlers keyed by "METHOD /path", and a log of the requests it received.
type server struct {
	t        *testing.T
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
	log      []*http.Request
	bodies   []string
}

func newServer(t *testing.T) (*server, *Client) {
	s := &server{t: t, handlers: map[string]http.HandlerFunc{}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.log = append(s.log, r)
		s.bodies = append(s.bodies, string(b))
		h := s.handlers[r.Method+" "+r.URL.Path]
		s.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("%s %s without the token", r.Method, r.URL.Path)
		}
		if h == nil {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		h(w, r)
	}))
	t.Cleanup(ts.Close)
	return s, New("tok", WithBaseURL(ts.URL), WithGraphQLURL(ts.URL+"/graphql"))
}

func (s *server) on(route string, h http.HandlerFunc) { s.handlers[route] = h }

func jsonReply(v any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
}

var ctx = context.Background()

func TestUpdatedIssuesSkipsPullRequestsAndFollowsPages(t *testing.T) {
	s, c := newServer(t)
	var base string
	s.on("GET /repos/o/r/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "all" || r.URL.Query().Get("since") != "2026-10-29T00:00:00Z" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("page") == "2" {
			jsonReply([]map[string]any{{"number": 3, "title": "c", "state": "closed", "updated_at": "2026-10-29T01:00:00Z"}})(w, r)
			return
		}
		base = "http://" + r.Host
		w.Header().Set("Link", fmt.Sprintf(`<%s/repos/o/r/issues?state=all&since=2026-10-29T00:00:00Z&page=2>; rel="next"`, base))
		jsonReply([]map[string]any{
			{"number": 1, "title": "a", "state": "open", "updated_at": "2026-10-29T01:00:00Z", "labels": []map[string]any{{"name": "agent:ready"}}},
			{"number": 2, "title": "pr", "state": "open", "updated_at": "2026-10-29T01:00:00Z", "pull_request": map[string]any{}},
		})(w, r)
	})
	got, err := c.UpdatedIssues(ctx, "o", "r", time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Ref.Number != 1 || got[1].Ref.Number != 3 || got[0].Labels[0] != "agent:ready" {
		t.Fatalf("got %+v", got)
	}
}

func TestUpdatedIssuesUsesETag(t *testing.T) {
	s, c := newServer(t)
	calls := 0
	s.on("GET /repos/o/r/issues", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			if r.Header.Get("If-None-Match") != `"v1"` {
				t.Errorf("second request without If-None-Match")
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		jsonReply([]map[string]any{{"number": 1, "title": "a", "state": "open", "updated_at": "2026-10-29T01:00:00Z"}})(w, r)
	})
	since := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	first, err := c.UpdatedIssues(ctx, "o", "r", since)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.UpdatedIssues(ctx, "o", "r", since)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || second[0].Ref.Number != 1 {
		t.Fatalf("304 did not return the cached list: %+v", second)
	}
}

func TestThreadMergesEditInformation(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/issues/7", jsonReply(map[string]any{
		"number": 7, "title": "t", "body": "b", "state": "open",
		"user": map[string]any{"login": "alice", "id": 1}, "labels": []map[string]any{{"name": "agent:waiting"}},
		"created_at": "2026-10-29T00:00:00Z", "updated_at": "2026-10-29T01:00:00Z",
	}))
	s.on("GET /repos/o/r/issues/7/comments", jsonReply([]map[string]any{
		{"id": 11, "body": "first", "user": map[string]any{"login": "alice", "id": 1}, "created_at": "2026-10-29T00:10:00Z"},
		{"id": 12, "body": "second", "user": map[string]any{"login": "alice", "id": 1}, "created_at": "2026-10-29T00:20:00Z"},
		{"id": 13, "body": "third", "user": map[string]any{"login": "bob", "id": 2}, "created_at": "2026-10-29T00:30:00Z"},
	}))
	s.on("POST /graphql", jsonReply(map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
		"lastEditedAt": "2026-10-29T00:05:00Z", "editor": map[string]any{"login": "mallory", "databaseId": 9},
		"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": false}, "nodes": []map[string]any{
			{"databaseId": 11, "lastEditedAt": nil, "editor": nil},
			{"databaseId": 12, "lastEditedAt": "2026-10-29T00:25:00Z", "editor": map[string]any{"login": "alice", "databaseId": 1}},
			{"databaseId": 13, "lastEditedAt": "2026-10-29T00:35:00Z", "editor": nil},
		}},
	}}}}))
	is, comments, err := c.Thread(ctx, forge.IssueRef{Owner: "o", Repo: "r", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if is.Author.ID != 1 || !is.BodyEdit.Edited || is.BodyEdit.Editor.ID != 9 {
		t.Errorf("issue edit = %+v", is.BodyEdit)
	}
	if len(comments) != 3 || comments[0].Edit.Edited || comments[1].Edit.Editor.ID != 1 {
		t.Errorf("comments = %+v", comments)
	}
	if !comments[2].Edit.Edited || !comments[2].Edit.Unknown {
		t.Errorf("an edit without an editor must be unknown: %+v", comments[2].Edit)
	}
}

func TestThreadMarksCommentsBeyondTheGraphQLPageUnknown(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/issues/7", jsonReply(map[string]any{"number": 7, "user": map[string]any{"login": "alice", "id": 1}}))
	s.on("GET /repos/o/r/issues/7/comments", jsonReply([]map[string]any{{"id": 11, "user": map[string]any{"login": "alice", "id": 1}}}))
	s.on("POST /graphql", jsonReply(map[string]any{"data": map[string]any{"repository": map[string]any{"issue": map[string]any{
		"comments": map[string]any{"pageInfo": map[string]any{"hasNextPage": true}, "nodes": []map[string]any{}},
	}}}}))
	_, comments, err := c.Thread(ctx, forge.IssueRef{Owner: "o", Repo: "r", Number: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !comments[0].Edit.Unknown {
		t.Errorf("a comment missing from the edit query must be unknown: %+v", comments[0].Edit)
	}
}

func TestLabelActorTakesTheLastLabeledEvent(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r/issues/7/events", jsonReply([]map[string]any{
		{"event": "labeled", "label": map[string]any{"name": "agent:ready"}, "actor": map[string]any{"login": "bob", "id": 2}},
		{"event": "unlabeled", "label": map[string]any{"name": "agent:ready"}, "actor": map[string]any{"login": "bob", "id": 2}},
		{"event": "labeled", "label": map[string]any{"name": "agent:ready"}, "actor": map[string]any{"login": "alice", "id": 1}},
		{"event": "labeled", "label": map[string]any{"name": "bug"}, "actor": map[string]any{"login": "bob", "id": 2}},
	}))
	u, err := c.LabelActor(ctx, forge.IssueRef{Owner: "o", Repo: "r", Number: 7}, "agent:ready")
	if err != nil || u.ID != 1 {
		t.Fatalf("LabelActor = %+v, %v", u, err)
	}
	if _, err := c.LabelActor(ctx, forge.IssueRef{Owner: "o", Repo: "r", Number: 7}, "agent:done"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestWrites(t *testing.T) {
	s, c := newServer(t)
	ref := forge.IssueRef{Owner: "o", Repo: "r", Number: 7}
	s.on("PUT /repos/o/r/issues/7/labels", jsonReply([]any{}))
	s.on("POST /repos/o/r/issues/7/comments", jsonReply(map[string]any{"id": 99, "body": "hi", "user": map[string]any{"login": "alice", "id": 1}}))
	s.on("POST /repos/o/r/pulls", jsonReply(map[string]any{"html_url": "https://github.test/o/r/pull/3"}))
	s.on("POST /repos/o/r/labels", jsonReply(map[string]any{"name": "agent:ready"}))
	s.on("GET /repos/o/r/labels/agent:done", jsonReply(map[string]any{"name": "agent:done"}))

	if err := c.SetLabels(ctx, ref, []string{"agent:waiting", "bug"}); err != nil {
		t.Fatal(err)
	}
	cm, err := c.PostComment(ctx, ref, "hi")
	if err != nil || cm.ID != 99 {
		t.Fatalf("PostComment = %+v, %v", cm, err)
	}
	url, err := c.CreateChangeRequest(ctx, forge.ChangeRequest{Owner: "o", Repo: "r", Title: "x", Head: "h", Base: "main", Draft: true})
	if err != nil || url != "https://github.test/o/r/pull/3" {
		t.Fatalf("CreateChangeRequest = %q, %v", url, err)
	}
	created, err := c.EnsureLabel(ctx, "o", "r", "agent:ready", "0e8a16", "d")
	if err != nil || !created {
		t.Fatalf("EnsureLabel(new) = %v, %v", created, err)
	}
	created, err = c.EnsureLabel(ctx, "o", "r", "agent:done", "0e8a16", "d")
	if err != nil || created {
		t.Fatalf("EnsureLabel(existing) = %v, %v", created, err)
	}
	joined := strings.Join(s.bodies, "\n")
	for _, want := range []string{`"labels":["agent:waiting","bug"]`, `"body":"hi"`, `"draft":true`, `"head":"h"`} {
		if !strings.Contains(joined, want) {
			t.Errorf("request bodies lack %s:\n%s", want, joined)
		}
	}
}

func TestReadsOfRepositoryUserAndSearch(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r", jsonReply(map[string]any{"private": true}))
	s.on("GET /users/alice", jsonReply(map[string]any{"login": "alice", "id": 1}))
	s.on("GET /search/issues", func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("q"); q != `is:issue is:open label:"agent:waiting" user:alice` {
			t.Errorf("q = %q", q)
		}
		jsonReply(map[string]any{"items": []map[string]any{{"number": 7, "title": "t", "state": "open",
			"repository_url": "https://api.github.test/repos/o/r", "labels": []map[string]any{{"name": "agent:waiting"}}}}})(w, r)
	})
	priv, err := c.RepoPrivate(ctx, "o", "r")
	if err != nil || !priv {
		t.Fatalf("RepoPrivate = %v, %v", priv, err)
	}
	u, err := c.User(ctx, "alice")
	if err != nil || u.ID != 1 {
		t.Fatalf("User = %+v, %v", u, err)
	}
	found, err := c.SearchOpenIssuesWithLabel(ctx, "user:alice", "agent:waiting")
	if err != nil || len(found) != 1 || found[0].Ref != (forge.IssueRef{Owner: "o", Repo: "r", Number: 7}) {
		t.Fatalf("Search = %+v, %v", found, err)
	}
}

func TestErrorsDoNotLeakTheToken(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
	})
	_, err := c.RepoPrivate(ctx, "o", "r")
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "tok") {
		t.Fatalf("err = %v", err)
	}
	if _, err := c.User(ctx, "nobody"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
