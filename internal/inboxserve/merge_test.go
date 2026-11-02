package inboxserve

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
)

// withMerges is withIssues with an open done issue gh:o/r#50 whose done post names pull request 9 at "abc".
func withMerges(t *testing.T) *issuesWorld {
	t.Helper()
	w := withIssues(t)
	bot := w.gh.AddUser("bot", 2)
	w.cfg.Forges.GitHub.AgentIdentity = "bot"
	ref := w.gh.AddIssue("o", "r", 50, "Finished work", "body", w.me)
	w.gh.PostAs = "bot"
	w.gh.PostComment(ctx, ref, "### The work is done\n\nPull request: https://forge.test/o/r/pull/9\n\n"+
		marker.Marker{Host: "here", Run: "d", State: marker.Done, SHA: "abc", CR: 9}.Format())
	w.gh.PostAs = "me"
	w.gh.AddLabel(ref, "agent:done", bot)
	w.gh.SetChange("o", "r", 9, fake.Change{Draft: true, Head: "abc"})
	w.srv.Merges = map[string]forge.Merges{forge.GitHub: w.gh, forge.GitLab: w.gl}
	return w
}

func (w *issuesWorld) merge(ref, sha string) (int, inboxapi.Error) {
	rec := w.do(http.MethodPost, "/v1/merge", inboxapi.MergeRequest{Ref: ref, SHA: sha})
	var e inboxapi.Error
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	return rec.Code, e
}

func TestMergeIsServedOnlyWithMerges(t *testing.T) {
	w := withIssues(t)
	if st, e := w.merge("gh:o/r#50", "abc"); st != http.StatusNotFound || e.Code != inboxapi.CodeNotFound {
		t.Fatalf("without Merges: %d %+v", st, e)
	}
	w = withMerges(t)
	if rec := w.do(http.MethodGet, "/v1/merge", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d", rec.Code)
	}
}

func TestMergeMergesTheDoneChange(t *testing.T) {
	w := withMerges(t)
	if st, e := w.merge("gh:o/r#50", "abc"); st != http.StatusNoContent {
		t.Fatalf("merge: %d %+v", st, e)
	}
	if c, _ := w.gh.ChangeState("o", "r", 9); !c.Merged {
		t.Fatalf("change = %+v", c)
	}
}

func TestMergeRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  func(w *issuesWorld)
		sha    string
		status int
		code   string
		says   string
	}{
		{"an older sha", nil, "old", http.StatusConflict, inboxapi.CodeStale, ""},
		{"a moved head", func(w *issuesWorld) { w.gh.SetChange("o", "r", 9, fake.Change{Head: "def"}) }, "abc",
			http.StatusConflict, inboxapi.CodeStale, ""},
		{"a refusal", func(w *issuesWorld) { w.gh.SetChange("o", "r", 9, fake.Change{Head: "abc", Refuse: "checks failed"}) }, "abc",
			http.StatusConflict, inboxapi.CodeNotMergeable, "checks failed"},
		{"a token without the permission", func(w *issuesWorld) { w.gh.FailMerge = forge.ErrForbidden }, "abc",
			http.StatusForbidden, inboxapi.CodeForbidden, "Pull requests"},
		{"an empty sha", nil, "", http.StatusBadRequest, inboxapi.CodeInvalid, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := withMerges(t)
			if tc.setup != nil {
				tc.setup(w)
			}
			st, e := w.merge("gh:o/r#50", tc.sha)
			if st != tc.status || e.Code != tc.code || !strings.Contains(e.Message, tc.says) {
				t.Fatalf("%d %+v, want %d %s saying %q", st, e, tc.status, tc.code, tc.says)
			}
		})
	}
}

func TestMergeRefusesABodyThatIsNotAMerge(t *testing.T) {
	w := withMerges(t)
	if rec := w.do(http.MethodPost, "/v1/merge", "not json"); rec.Code != http.StatusBadRequest || code(t, rec) != inboxapi.CodeInvalid {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
