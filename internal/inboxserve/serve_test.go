package inboxserve

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/marker"
)

var ctx = context.Background()

// counting counts the searches a forge is asked for.
type counting struct {
	forge.Forge
	searches atomic.Int32
	err      error
}

func (c *counting) SearchOpenIssuesWithLabel(ctx context.Context, scope string, labels ...string) ([]forge.IssueSummary, error) {
	c.searches.Add(1)
	if c.err != nil {
		return nil, c.err
	}
	return c.Forge.SearchOpenIssuesWithLabel(ctx, scope, labels...)
}

type world struct {
	f      *fake.Forge
	gh, gl *counting
	srv    *Server
	c      *inboxapi.Client
	now    time.Time
	ref    forge.IssueRef
	bot    forge.User
}

func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{f: fake.New(nil), now: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}
	me := w.f.AddUser("me", 1)
	w.bot = w.f.AddUser("bot", 2)
	w.f.AddRepo("o", "r", true)
	g := fake.NewGitLab(nil)
	g.AddUser("bot", 2)
	g.AddProject("nas", "sb", "private")
	cfg := &config.Config{
		Host: config.Host{Name: "here"},
		Forges: config.Forges{
			GitHub: config.GitHub{AgentIdentity: "bot", SearchScope: "user:me"},
			GitLab: config.GitLab{AgentIdentity: "bot"},
		},
		Labels: config.Labels{Ready: "agent:ready", Running: "agent:running", Waiting: "agent:waiting", Done: "agent:done"},
		Projects: []config.Project{
			{Ref: "gh:o/r", Forge: "gh", Owner: "o", Repo: "r", Host: "here"},
			{Ref: "gl:nas/sb", Forge: "gl", Owner: "nas", Repo: "sb", Host: "here"},
		},
	}
	w.gh, w.gl = &counting{Forge: w.f}, &counting{Forge: g}
	b := &inbox.Inbox{Config: cfg, Forges: map[string]forge.Forge{forge.GitHub: w.gh, forge.GitLab: w.gl}, Now: func() time.Time { return w.now }}
	w.f.PostAs = "bot"
	w.ref = w.f.AddIssue("o", "r", 12, "Add a flag", "body", me)
	w.f.PostComment(ctx, w.ref, "### The agent has a question\n\n**Question**\n\nWhich default?\n\n"+
		marker.Marker{Host: "laptop", Run: "r1", State: "waiting", Reason: "question", At: w.now.Add(-time.Hour)}.Format()+"\n")
	w.f.AddLabel(w.ref, "agent:waiting", w.bot)
	w.f.PostAs = "me"
	w.srv = &Server{Inbox: b, Now: func() time.Time { return w.now }}

	dir, err := os.MkdirTemp("", "is")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "inbox.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	hs := &http.Server{Handler: w.srv}
	go hs.Serve(ln)
	t.Cleanup(func() { hs.Close() })
	w.c = &inboxapi.Client{Socket: sock}
	return w
}

func TestListReportsTheUnreachableForge(t *testing.T) {
	w := newWorld(t)
	w.gl.err = errors.New("connection refused")
	got, err := w.c.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].Ref != "gh:o/r#12" || got.Items[0].Question != "Which default?" {
		t.Fatalf("items = %+v", got.Items)
	}
	if len(got.Errors) != 1 || got.Errors[0].Forge != "gitlab" || got.Errors[0].Message == "" {
		t.Fatalf("errors = %+v", got.Errors)
	}
	if !got.FetchedAt.Equal(w.now) {
		t.Fatalf("fetched_at = %v", got.FetchedAt)
	}
}

func TestListIsCachedForAMinuteAndDroppedByAReply(t *testing.T) {
	w := newWorld(t)
	w.c.List(ctx)
	w.now = w.now.Add(59 * time.Second)
	w.c.List(ctx)
	if n := w.gh.searches.Load(); n != 1 {
		t.Fatalf("searches within a minute = %d", n)
	}
	w.now = w.now.Add(2 * time.Second)
	w.c.List(ctx)
	if n := w.gh.searches.Load(); n != 2 {
		t.Fatalf("searches after a minute = %d", n)
	}
	if err := w.c.Reply(ctx, "gh:o/r#12", "Go with B.", false); err != nil {
		t.Fatal(err)
	}
	w.c.List(ctx)
	if n := w.gh.searches.Load(); n != 3 {
		t.Fatalf("searches after a reply = %d", n)
	}
}

func TestIssue(t *testing.T) {
	w := newWorld(t)
	d, err := w.c.Issue(ctx, "gh:o/r#12")
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "Add a flag" || d.URL != "https://github.test/o/r/issues/12" || !d.Waiting || d.Reason != "question" {
		t.Fatalf("detail = %+v", d)
	}
	if _, err := w.c.Issue(ctx, "1"); !errors.Is(err, inboxapi.ErrInvalid) {
		t.Fatalf("a row number: %v", err)
	}
	if _, err := w.c.Issue(ctx, "gh:o/elsewhere#1"); !errors.Is(err, inboxapi.ErrNotFound) {
		t.Fatalf("an unconfigured project: %v", err)
	}
}

func TestReplyToAnIssueThatIsNotWaiting(t *testing.T) {
	w := newWorld(t)
	w.f.RemoveLabel(w.ref, "agent:waiting")
	before := w.f.Comments
	err := w.c.Reply(ctx, "gh:o/r#12", "Go with B.", false)
	if !errors.Is(err, inboxapi.ErrNotWaiting) || w.f.Comments != before {
		t.Fatalf("err = %v, comments %d -> %d", err, before, w.f.Comments)
	}
	if err := w.c.Reply(ctx, "gh:o/r#12", "  ", true); !errors.Is(err, inboxapi.ErrInvalid) {
		t.Fatalf("empty reply: %v", err)
	}
}

func TestWrongMethodsAndBodies(t *testing.T) {
	w := newWorld(t)
	hc := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", w.c.Socket)
	}}}
	for _, c := range []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/v1/inbox", "", http.StatusMethodNotAllowed},
		{"GET", "/v1/reply", "", http.StatusMethodNotAllowed},
		{"POST", "/v1/reply", "{not json", http.StatusBadRequest},
		{"GET", "/v2/inbox", "", http.StatusNotFound},
	} {
		req, _ := http.NewRequest(c.method, "http://inbox"+c.path, stringsReader(c.body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != c.want {
			t.Errorf("%s %s = %d, want %d", c.method, c.path, resp.StatusCode, c.want)
		}
	}
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }
