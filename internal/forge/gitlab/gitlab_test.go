package gitlab

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

// server is a scripted GitLab: handlers keyed by "METHOD /escaped/path", and a log of the requests it received.
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
		h := s.handlers[r.Method+" "+r.URL.EscapedPath()]
		s.mu.Unlock()
		if r.Header.Get("PRIVATE-TOKEN") != "tok" {
			t.Errorf("%s %s without the token", r.Method, r.URL.Path)
		}
		if h == nil {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"404 Not found"}`)
			return
		}
		h(w, r)
	}))
	t.Cleanup(ts.Close)
	return s, New("tok", ts.URL)
}

func (s *server) on(route string, h http.HandlerFunc) { s.handlers[route] = h }

func (s *server) body(i int) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	var m map[string]any
	if err := json.Unmarshal([]byte(s.bodies[i]), &m); err != nil {
		s.t.Fatalf("request %d body %q: %v", i, s.bodies[i], err)
	}
	return m
}

func jsonReply(v any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
}

var ctx = context.Background()

// The project of the tests sits in a subgroup, so its escaped path shows that subgroups survive.
const proj = "/api/v4/projects/nas%2Ftools%2Fsb"

var ref = forge.IssueRef{Forge: forge.GitLab, Owner: "nas/tools", Repo: "sb", Number: 3}

func user(id int64, name string) map[string]any { return map[string]any{"id": id, "username": name} }

func TestUpdatedIssuesFollowsPagesAndUsesUpdatedAfter(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != "all" || q.Get("order_by") != "updated_at" || q.Get("updated_after") != "2026-10-30T00:00:00Z" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if q.Get("page") == "2" {
			jsonReply([]map[string]any{{"iid": 2, "title": "b", "state": "closed", "labels": []string{}, "updated_at": "2026-10-30T09:00:00+09:00"}})(w, r)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<http://%s%s/issues?page=2&state=all&order_by=updated_at&updated_after=2026-10-30T00%%3A00%%3A00Z>; rel="next"`, r.Host, proj))
		jsonReply([]map[string]any{{"iid": 3, "title": "a", "state": "opened", "labels": []string{"agent:ready"}, "updated_at": "2026-10-30T10:00:00+09:00", "user_notes_count": 4}})(w, r)
	})
	got, err := c.UpdatedIssues(ctx, "nas/tools", "sb", time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Ref != ref || got[0].State != "open" || got[1].State != "closed" || got[0].Labels[0] != "agent:ready" {
		t.Fatalf("got %+v", got)
	}
}

func TestUpdatedIssuesOmitsUpdatedAfterForTheZeroTime(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("updated_after") {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		jsonReply([]any{})(w, r)
	})
	if _, err := c.UpdatedIssues(ctx, "nas/tools", "sb", time.Time{}); err != nil {
		t.Fatal(err)
	}
}

func TestProjectUserAndVisibility(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj, jsonReply(map[string]any{"id": 7, "path_with_namespace": "nas/tools/sb", "visibility": "internal"}))
	s.on("GET /api/v4/users", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") == "uskanda" {
			jsonReply([]any{user(1, "uskanda")})(w, r)
			return
		}
		jsonReply([]any{})(w, r)
	})
	s.on("GET /api/v4/version", jsonReply(map[string]any{"version": "19.5.1", "enterprise": false}))
	// internal is readable by every user of the instance, so it does not count as private.
	if p, err := c.RepoPrivate(ctx, "nas/tools", "sb"); err != nil || p {
		t.Fatalf("RepoPrivate = %v, %v", p, err)
	}
	if id, err := c.ProjectID(ctx, "nas/tools", "sb"); err != nil || id != 7 {
		t.Fatalf("ProjectID = %d, %v", id, err)
	}
	if u, err := c.User(ctx, "uskanda"); err != nil || u.ID != 1 || u.Login != "uskanda" {
		t.Fatalf("User = %+v, %v", u, err)
	}
	if _, err := c.User(ctx, "nobody"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("User(nobody) err = %v", err)
	}
	if v, err := c.Version(ctx); err != nil || v != "19.5.1" {
		t.Fatalf("Version = %q, %v", v, err)
	}
}

// threadServer scripts an issue whose body bob edited (a system note names him), with a comment alice edited, one
// she did not, and a system note that is not a comment.
func threadServer(t *testing.T, edits []any) (*server, *Client) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues/3", jsonReply(map[string]any{
		"iid": 3, "title": "t", "description": "the body", "state": "opened", "labels": []string{"agent:waiting"},
		"author": user(1, "alice"), "created_at": "2026-10-30T09:00:00+09:00", "updated_at": "2026-10-30T09:05:00+09:00",
	}))
	s.on("GET "+proj+"/issues/3/notes", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sort") != "asc" {
			t.Errorf("notes query = %s", r.URL.RawQuery)
		}
		jsonReply([]map[string]any{
			{"id": 80, "system": true, "body": "changed the description", "author": user(2, "bob"), "created_at": "2026-10-30T09:01:00+09:00"},
			{"id": 81, "system": false, "body": "edited reply", "author": user(1, "alice"), "created_at": "2026-10-30T09:02:00+09:00"},
			{"id": 82, "system": false, "body": "plain reply", "author": user(1, "alice"), "created_at": "2026-10-30T09:03:00+09:00"},
			{"id": 83, "system": true, "body": "added ~5 label", "author": user(1, "alice"), "created_at": "2026-10-30T09:04:00+09:00"},
		})(w, r)
	})
	s.on("POST /api/graphql", func(w http.ResponseWriter, r *http.Request) {
		jsonReply(map[string]any{"data": map[string]any{"project": map[string]any{"issue": map[string]any{"notes": map[string]any{"nodes": edits}}}}})(w, r)
	})
	return s, c
}

func TestThreadTakesEditorsFromSystemNotesAndGraphQL(t *testing.T) {
	_, c := threadServer(t, []any{
		map[string]any{"id": "gid://gitlab/Note/81", "lastEditedBy": map[string]any{"id": "gid://gitlab/User/1", "username": "alice"}},
		map[string]any{"id": "gid://gitlab/Note/82", "lastEditedBy": nil},
	})
	is, cs, err := c.Thread(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if is.Ref != ref || is.State != "open" || is.Body != "the body" || is.Author.ID != 1 {
		t.Fatalf("issue = %+v", is)
	}
	if !is.BodyEdit.Edited || is.BodyEdit.Unknown || is.BodyEdit.Editor.ID != 2 {
		t.Fatalf("BodyEdit = %+v", is.BodyEdit)
	}
	// System notes are not comments.
	if len(cs) != 2 || cs[0].ID != 81 || cs[1].ID != 82 {
		t.Fatalf("comments = %+v", cs)
	}
	if !cs[0].Edit.Edited || cs[0].Edit.Editor.ID != 1 || cs[1].Edit.Edited {
		t.Fatalf("edits = %+v / %+v", cs[0].Edit, cs[1].Edit)
	}
}

// A comment GraphQL did not report on (beyond its page) has an editor nobody knows.
func TestThreadMarksCommentsGraphQLMissedUnknown(t *testing.T) {
	_, c := threadServer(t, []any{map[string]any{"id": "gid://gitlab/Note/82", "lastEditedBy": nil}})
	_, cs, err := c.Thread(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if !cs[0].Edit.Unknown || cs[1].Edit.Unknown {
		t.Fatalf("edits = %+v / %+v", cs[0].Edit, cs[1].Edit)
	}
}

func TestThreadWithoutDescriptionNoteIsUnedited(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues/3", jsonReply(map[string]any{"iid": 3, "title": "t", "description": "b", "state": "opened", "author": user(1, "alice")}))
	s.on("GET "+proj+"/issues/3/notes", jsonReply([]any{}))
	s.on("POST /api/graphql", jsonReply(map[string]any{"data": map[string]any{"project": map[string]any{"issue": map[string]any{"notes": map[string]any{"nodes": []any{}}}}}}))
	is, _, err := c.Thread(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if is.BodyEdit.Edited {
		t.Fatalf("BodyEdit = %+v", is.BodyEdit)
	}
}

// Past maxNotePages pages, a later description change may hide, so the body's editor is unknown.
func TestThreadBeyondTheNotePagesLosesTheBodyEditor(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues/3", jsonReply(map[string]any{"iid": 3, "title": "t", "description": "b", "state": "opened", "author": user(1, "alice")}))
	s.on("GET "+proj+"/issues/3/notes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", fmt.Sprintf(`<http://%s%s/issues/3/notes?sort=asc&page=99>; rel="next"`, r.Host, proj))
		jsonReply([]any{})(w, r)
	})
	s.on("POST /api/graphql", jsonReply(map[string]any{"data": map[string]any{"project": map[string]any{"issue": map[string]any{"notes": map[string]any{"nodes": []any{}}}}}}))
	is, _, err := c.Thread(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if !is.BodyEdit.Edited || !is.BodyEdit.Unknown {
		t.Fatalf("BodyEdit = %+v", is.BodyEdit)
	}
}

func TestLabelActorTakesTheLastAdd(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues/3/resource_label_events", jsonReply([]map[string]any{
		{"action": "add", "label": map[string]any{"name": "agent:ready"}, "user": user(2, "bob")},
		{"action": "remove", "label": map[string]any{"name": "agent:ready"}, "user": user(1, "alice")},
		{"action": "add", "label": map[string]any{"name": "agent:ready"}, "user": user(1, "alice")},
		{"action": "add", "label": nil, "user": user(3, "carol")},
	}))
	u, err := c.LabelActor(ctx, ref, "agent:ready")
	if err != nil || u.ID != 1 {
		t.Fatalf("LabelActor = %+v, %v", u, err)
	}
	if _, err := c.LabelActor(ctx, ref, "agent:done"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("LabelActor(done) err = %v", err)
	}
}

func TestWrites(t *testing.T) {
	s, c := newServer(t)
	s.on("PUT "+proj+"/issues/3", jsonReply(map[string]any{"iid": 3}))
	s.on("POST "+proj+"/issues/3/notes", jsonReply(map[string]any{"id": 90, "body": "hi", "author": user(9, "bot"), "created_at": "2026-10-30T09:00:00+09:00"}))
	s.on("GET "+proj+"/labels/agent:ready", jsonReply(map[string]any{"name": "agent:ready"}))
	s.on("POST "+proj+"/labels", jsonReply(map[string]any{"name": "agent:done"}))
	s.on("POST "+proj+"/merge_requests", jsonReply(map[string]any{"iid": 4, "web_url": "https://gl.test/nas/tools/sb/-/merge_requests/4"}))

	if err := c.SetLabels(ctx, ref, []string{"agent:waiting", "bug"}); err != nil {
		t.Fatal(err)
	}
	if got := s.body(0)["labels"]; got != "agent:waiting,bug" {
		t.Errorf("labels = %v", got)
	}
	if err := c.SetLabels(ctx, ref, nil); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.body(1)["labels"]; !ok || got != "" {
		t.Errorf("clearing labels sent %v", s.body(1))
	}
	cm, err := c.PostComment(ctx, ref, "hi")
	if err != nil || cm.ID != 90 || cm.Author.ID != 9 {
		t.Fatalf("PostComment = %+v, %v", cm, err)
	}
	if created, err := c.EnsureLabel(ctx, "nas/tools", "sb", "agent:ready", "0e8a16", "d"); err != nil || created {
		t.Fatalf("EnsureLabel(existing) = %v, %v", created, err)
	}
	if created, err := c.EnsureLabel(ctx, "nas/tools", "sb", "agent:done", "0e8a16", "d"); err != nil || !created {
		t.Fatalf("EnsureLabel(new) = %v, %v", created, err)
	}
	if got := s.body(5)["color"]; got != "#0e8a16" {
		t.Errorf("label color = %v", got)
	}
	u, err := c.CreateChangeRequest(ctx, forge.ChangeRequest{Owner: "nas/tools", Repo: "sb", Title: "Add a file", Body: "Closes #3", Head: "switchboard/issue-3", Base: "main", Draft: true})
	if err != nil || u != "https://gl.test/nas/tools/sb/-/merge_requests/4" {
		t.Fatalf("CreateChangeRequest = %q, %v", u, err)
	}
	mr := s.body(6)
	if mr["title"] != "Draft: Add a file" || mr["source_branch"] != "switchboard/issue-3" || mr["target_branch"] != "main" || mr["description"] != "Closes #3" {
		t.Errorf("merge request = %v", mr)
	}
}

func TestSearchOpenIssuesWithLabelReadsOneProject(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj+"/issues", func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query(); q.Get("labels") != "agent:waiting" || q.Get("state") != "opened" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		jsonReply([]map[string]any{{"iid": 3, "title": "t", "state": "opened", "labels": []string{"agent:waiting"}}})(w, r)
	})
	got, err := c.SearchOpenIssuesWithLabel(ctx, "nas/tools/sb", "agent:waiting")
	if err != nil || len(got) != 1 || got[0].Ref != ref {
		t.Fatalf("Search = %+v, %v", got, err)
	}
}

func TestErrorsDoNotLeakTheToken(t *testing.T) {
	s, c := newServer(t)
	s.on("GET "+proj, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"message":"403 Forbidden"}`)
	})
	_, err := c.RepoPrivate(ctx, "nas/tools", "sb")
	if err == nil || !strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "tok") {
		t.Fatalf("err = %v", err)
	}
	if _, err := c.RepoPrivate(ctx, "nas/tools", "missing"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("a 404 is not ErrNotFound: %v", err)
	}
}
