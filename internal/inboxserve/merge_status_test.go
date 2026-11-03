package inboxserve

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
)

// withMergeStatus is withMerges with the inbox token's CI clients.
func withMergeStatus(t *testing.T) *issuesWorld {
	t.Helper()
	w := withMerges(t)
	w.srv.CI = map[string]forge.CI{forge.GitHub: w.gh, forge.GitLab: w.gl}
	return w
}

func (w *issuesWorld) mergeStatus(ref, sha string) (int, inboxapi.MergeStatus, inboxapi.Error) {
	rec := w.do(http.MethodGet, "/v1/merge-status?"+url.Values{"ref": {ref}, "sha": {sha}}.Encode(), nil)
	var st inboxapi.MergeStatus
	var e inboxapi.Error
	if rec.Code == http.StatusOK {
		_ = json.Unmarshal(rec.Body.Bytes(), &st)
	} else {
		_ = json.Unmarshal(rec.Body.Bytes(), &e)
	}
	return rec.Code, st, e
}

func TestMergeStatusIsServedOnlyWithCI(t *testing.T) {
	w := withMerges(t)
	if st, _, e := w.mergeStatus("gh:o/r#50", "abc"); st != http.StatusNotFound || e.Code != inboxapi.CodeNotFound {
		t.Fatalf("without CI: %d %+v", st, e)
	}
	w = withMergeStatus(t)
	if rec := w.do(http.MethodPost, "/v1/merge-status", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: %d", rec.Code)
	}
}

func TestMergeStatusAnswersTheStatusOfTheChange(t *testing.T) {
	w := withMergeStatus(t)
	w.gh.SetChange("o", "r", 9, fake.Change{Draft: true, Head: "def", Conflict: true})
	w.gh.SetChecks("o", "r", "abc",
		forge.Check{Name: "test", Done: true, Conclusion: forge.Success, URL: "https://ci/1"},
		forge.Check{Name: "build", URL: "https://ci/3"})
	code, st, e := w.mergeStatus("gh:o/r#50", "abc")
	if code != http.StatusOK {
		t.Fatalf("%d %+v", code, e)
	}
	if st.State != forge.CROpen || !st.HeadMoved || st.Mergeable != inboxapi.MergeConflict || st.CI != inboxapi.CIRead ||
		len(st.Checks) != 2 || st.Checks[1] != (inboxapi.CheckStatus{Name: "build", Status: inboxapi.CheckRunning, URL: "https://ci/3"}) {
		t.Fatalf("status = %+v", st)
	}
}

func TestMergeStatusWithATokenThatCannotReadCI(t *testing.T) {
	w := withMergeStatus(t)
	w.gh.FailCI = forge.ErrForbidden
	code, st, e := w.mergeStatus("gh:o/r#50", "abc")
	if code != http.StatusOK || st.CI != inboxapi.CIForbidden || st.Checks == nil || st.Mergeable != inboxapi.MergeClean {
		t.Fatalf("%d %+v %+v", code, st, e)
	}
}

func TestMergeStatusRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, ref, sha string
		status         int
		code           string
	}{
		{"an older sha", "gh:o/r#50", "old", http.StatusConflict, inboxapi.CodeStale},
		{"an empty sha", "gh:o/r#50", "", http.StatusBadRequest, inboxapi.CodeInvalid},
		{"a malformed ref", "nope", "abc", http.StatusBadRequest, inboxapi.CodeInvalid},
		{"an unknown project", "gh:x/y#1", "abc", http.StatusNotFound, inboxapi.CodeNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := withMergeStatus(t)
			if st, _, e := w.mergeStatus(tc.ref, tc.sha); st != tc.status || e.Code != tc.code {
				t.Fatalf("%d %+v, want %d %s", st, e, tc.status, tc.code)
			}
		})
	}
}
