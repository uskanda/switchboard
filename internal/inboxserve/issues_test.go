package inboxserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/marker"
)

type issuesWorld struct {
	gh, gl *fake.Forge
	cfg    *config.Config
	srv    *Server
	me     forge.User
}

// failingIssues is a forge.Issues that cannot be reached.
type failingIssues struct{}

func (failingIssues) CreateIssue(context.Context, string, string, string, string, []string) (*forge.IssueSummary, error) {
	return nil, errors.New("connection refused")
}

// withIssues serves new issues over a GitHub and a GitLab fake, for projects run by two machines. The handler is
// called directly, so the tests need no socket.
func withIssues(t *testing.T) *issuesWorld {
	t.Helper()
	w := &issuesWorld{gh: fake.New(nil), gl: fake.NewGitLab(nil)}
	w.me = w.gh.AddUser("me", 1)
	w.gl.AddUser("me", 1)
	w.gh.PostAs, w.gl.PostAs = "me", "me"
	w.gh.AddRepo("o", "r", true)
	w.gh.AddRepo("o", "far", true)
	w.gl.AddProject("nas", "sb", "private")
	w.cfg = &config.Config{
		Host:   config.Host{Name: "here"},
		Labels: config.Labels{Ready: "agent:ready", Plan: "agent:plan", Go: "agent:go", Running: "agent:running", Waiting: "agent:waiting", Done: "agent:done"},
		Projects: []config.Project{
			{Ref: "gh:o/r", Forge: "gh", Owner: "o", Repo: "r", Host: "here"},
			{Ref: "gl:nas/sb", Forge: "gl", Owner: "nas", Repo: "sb", Host: "nas"},
			{Ref: "gh:o/far", Forge: "gh", Owner: "o", Repo: "far", Host: "elsewhere"},
		},
	}
	b := &inbox.Inbox{Config: w.cfg, Forges: map[string]forge.Forge{forge.GitHub: w.gh, forge.GitLab: w.gl}}
	w.srv = &Server{Inbox: b, Now: func() time.Time { return time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC) },
		NewIssues: &NewIssues{Config: w.cfg, Issues: map[string]forge.Issues{forge.GitHub: w.gh, forge.GitLab: w.gl}}}
	return w
}

func (w *issuesWorld) do(method, path string, body any) *httptest.ResponseRecorder {
	var data []byte
	switch b := body.(type) {
	case nil:
	case string:
		data = []byte(b)
	default:
		data, _ = json.Marshal(b)
	}
	rec := httptest.NewRecorder()
	w.srv.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(data)))
	return rec
}

func (w *issuesWorld) create(req inboxapi.CreateIssueRequest) *httptest.ResponseRecorder {
	return w.do(http.MethodPost, "/v1/issues", req)
}

func code(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var e inboxapi.Error
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	return e.Code
}

func TestIssueTargetsListsEveryProjectWithItsMachine(t *testing.T) {
	w := withIssues(t)
	rec := w.do(http.MethodGet, "/v1/issue-targets", nil)
	var got inboxapi.IssueTargets
	if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	wantItems := []inboxapi.IssueTarget{{Ref: "gh:o/r", Host: "here"}, {Ref: "gl:nas/sb", Host: "nas"}, {Ref: "gh:o/far", Host: "elsewhere"}}
	if !slices.Equal(got.Items, wantItems) {
		t.Errorf("items = %+v, want %+v", got.Items, wantItems)
	}
	wantStarts := []inboxapi.IssueStart{{Kind: "ready", Label: "agent:ready"}, {Kind: "plan", Label: "agent:plan"}, {Kind: "go", Label: "agent:go"}}
	if !slices.Equal(got.Starts, wantStarts) {
		t.Errorf("starts = %+v, want %+v", got.Starts, wantStarts)
	}
}

func TestIssueTargetsLeavesOutAStartWithoutALabel(t *testing.T) {
	w := withIssues(t)
	w.cfg.Labels.Plan = ""
	var got inboxapi.IssueTargets
	_ = json.Unmarshal(w.do(http.MethodGet, "/v1/issue-targets", nil).Body.Bytes(), &got)
	if len(got.Starts) != 2 || got.Starts[0].Kind != "ready" || got.Starts[1].Kind != "go" {
		t.Fatalf("starts = %+v, want ready and go", got.Starts)
	}
	rec := w.create(inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: "t", Start: "plan"})
	if rec.Code != http.StatusBadRequest || code(t, rec) != inboxapi.CodeInvalid {
		t.Errorf("a start without a label = %d %s, want 400 invalid", rec.Code, rec.Body)
	}
}

func TestCreateIssueOpensItWithTheStartLabelAsTheInboxToken(t *testing.T) {
	w := withIssues(t)
	rec := w.create(inboxapi.CreateIssueRequest{Ref: "gl:nas/sb", Title: "  Add a flag  ", Body: "Use --x.", Start: "plan"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	var got inboxapi.CreateIssueResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Ref != "gl:nas/sb#1" || got.URL != "https://gitlab.test/nas/sb/-/issues/1" {
		t.Fatalf("response = %+v", got)
	}
	ref := forge.IssueRef{Forge: forge.GitLab, Owner: "nas", Repo: "sb", Number: 1}
	is, _, _ := w.gl.Thread(ctx, ref)
	if is.Title != "Add a flag" || is.Body != "Use --x." || is.Author != w.me || !slices.Equal(is.Labels, []string{"agent:plan"}) {
		t.Errorf("issue = %+v", is)
	}
}

func TestCreateIssueInAProjectOfAnotherMachine(t *testing.T) {
	w := withIssues(t)
	if rec := w.create(inboxapi.CreateIssueRequest{Ref: "gh:o/far", Title: "t", Start: "ready"}); rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	if got := w.gh.Labels(forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "far", Number: 1}); !slices.Equal(got, []string{"agent:ready"}) {
		t.Errorf("labels = %v", got)
	}
}

func TestCreateIssueRefusals(t *testing.T) {
	end := marker.Marker{Host: "h", Run: "r", State: "done", At: time.Now()}.Format()
	for _, tc := range []struct {
		name   string
		req    inboxapi.CreateIssueRequest
		status int
		code   string
	}{
		{"unknown project", inboxapi.CreateIssueRequest{Ref: "gh:o/nope", Title: "t", Start: "ready"}, 422, inboxapi.CodeUnknownProject},
		{"blank title", inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: " \n ", Start: "ready"}, 400, inboxapi.CodeInvalid},
		{"long title", inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: strings.Repeat("あ", 257), Start: "ready"}, 400, inboxapi.CodeInvalid},
		{"no start", inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: "t"}, 400, inboxapi.CodeInvalid},
		{"a label as the start", inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: "t", Start: "agent:done"}, 400, inboxapi.CodeInvalid},
		{"large body", inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: "t", Body: strings.Repeat("x", inboxapi.MaxIssueBody+1), Start: "ready"}, 413, inboxapi.CodeTooLarge},
		{"marker line", inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: "t", Body: "text\n\n" + end + "\n", Start: "ready"}, 400, inboxapi.CodeInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := withIssues(t)
			rec := w.create(tc.req)
			if rec.Code != tc.status || code(t, rec) != tc.code {
				t.Errorf("POST = %d %s, want %d %s", rec.Code, rec.Body, tc.status, tc.code)
			}
			if is, _ := w.gh.UpdatedIssues(ctx, "o", "r", time.Time{}); len(is) != 0 {
				t.Errorf("an issue was opened: %+v", is)
			}
		})
	}
}

func TestCreateIssueTakesAFullTitleOfTheLimit(t *testing.T) {
	w := withIssues(t)
	if rec := w.create(inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: strings.Repeat("あ", 256), Start: "go"}); rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
}

func TestCreateIssueThatIsNotJSON(t *testing.T) {
	w := withIssues(t)
	if rec := w.do(http.MethodPost, "/v1/issues", "title=t"); rec.Code != http.StatusBadRequest || code(t, rec) != inboxapi.CodeInvalid {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
}

func TestCreateIssueReportsTheForge(t *testing.T) {
	w := withIssues(t)
	w.srv.NewIssues.Issues[forge.GitHub] = failingIssues{}
	if rec := w.create(inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: "t", Start: "ready"}); rec.Code != http.StatusBadGateway || code(t, rec) != inboxapi.CodeForge {
		t.Fatalf("POST = %d %s, want 502 forge", rec.Code, rec.Body)
	}
}

func TestCreateIssueReadsTheForgesAgain(t *testing.T) {
	w := withIssues(t)
	w.srv.cached = &inboxapi.Inbox{FetchedAt: w.srv.now()}
	w.srv.refresh = make(chan struct{}, 1)
	if rec := w.create(inboxapi.CreateIssueRequest{Ref: "gh:o/r", Title: "t", Start: "ready"}); rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	if w.srv.cached != nil {
		t.Error("the cached listing was kept")
	}
	select {
	case <-w.srv.refresh:
	default:
		t.Error("the forges were not asked to be read again")
	}
}

func TestIssueTargetsFollowAReload(t *testing.T) {
	w := withIssues(t)
	next := *w.cfg
	next.Projects = append(slices.Clone(w.cfg.Projects), config.Project{Ref: "gh:o/new", Forge: "gh", Owner: "o", Repo: "new", Host: "here"})
	w.gh.AddRepo("o", "new", true)
	w.srv.Reload(&next, &inbox.Inbox{Config: &next, Forges: map[string]forge.Forge{forge.GitHub: w.gh, forge.GitLab: w.gl}})
	var got inboxapi.IssueTargets
	_ = json.Unmarshal(w.do(http.MethodGet, "/v1/issue-targets", nil).Body.Bytes(), &got)
	if len(got.Items) != 4 || got.Items[3].Ref != "gh:o/new" {
		t.Fatalf("items = %+v, want gh:o/new last", got.Items)
	}
	if rec := w.create(inboxapi.CreateIssueRequest{Ref: "gh:o/new", Title: "t", Start: "ready"}); rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
}

func TestIssuesMethods(t *testing.T) {
	w := withIssues(t)
	if rec := w.do(http.MethodGet, "/v1/issues", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /v1/issues = %d", rec.Code)
	}
	if rec := w.do(http.MethodPost, "/v1/issue-targets", "{}"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /v1/issue-targets = %d", rec.Code)
	}
}
