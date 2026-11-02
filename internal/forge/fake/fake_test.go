package fake

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

func TestFakeRecordsLabelActorAndUpdates(t *testing.T) {
	f := New(nil)
	alice := f.AddUser("alice", 1)
	f.AddRepo("alice", "sandbox", true)
	ref := f.AddIssue("alice", "sandbox", 1, "t", "b", alice)
	since := time.Now().Add(-time.Minute)
	f.AddLabel(ref, "agent:ready", alice)

	got, err := f.LabelActor(context.Background(), ref, "agent:ready")
	if err != nil || got.ID != 1 {
		t.Fatalf("LabelActor = %+v, %v", got, err)
	}
	list, _ := f.UpdatedIssues(context.Background(), "alice", "sandbox", since)
	if len(list) != 1 || list[0].Labels[0] != "agent:ready" {
		t.Fatalf("UpdatedIssues = %+v", list)
	}
}

func TestFakeIssueURL(t *testing.T) {
	gh := New(nil)
	alice := gh.AddUser("alice", 1)
	gh.AddRepo("alice", "sandbox", true)
	is, _, err := gh.Thread(context.Background(), gh.AddIssue("alice", "sandbox", 3, "t", "b", alice))
	if err != nil || is.URL != "https://github.test/alice/sandbox/issues/3" {
		t.Fatalf("GitHub URL = %q, %v", is.URL, err)
	}
	gl := NewGitLab(nil)
	bob := gl.AddUser("bob", 2)
	gl.AddProject("nas", "sb", "private")
	is, _, err = gl.Thread(context.Background(), gl.AddIssue("nas", "sb", 4, "t", "b", bob))
	if err != nil || is.URL != "https://gitlab.test/nas/sb/-/issues/4" {
		t.Fatalf("GitLab URL = %q, %v", is.URL, err)
	}
}

func TestRepos(t *testing.T) {
	f := New(nil)
	f.AddRepo("o", "r", false)
	if b, err := f.DefaultBranch(context.Background(), "o", "r"); err != nil || b != "main" {
		t.Errorf("DefaultBranch = %q, %v; want main", b, err)
	}
	f.SetDefaultBranch("o", "r", "trunk")
	if b, _ := f.DefaultBranch(context.Background(), "o", "r"); b != "trunk" {
		t.Errorf("DefaultBranch = %q, want trunk", b)
	}
	if _, err := f.DefaultBranch(context.Background(), "o", "nope"); !errors.Is(err, forge.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	_, _ = f.EnsureLabel(context.Background(), "o", "r", "b", "", "")
	_, _ = f.EnsureLabel(context.Background(), "o", "r", "a", "", "")
	if got, _ := f.LabelNames(context.Background(), "o", "r"); strings.Join(got, ",") != "a,b" {
		t.Errorf("LabelNames = %v, want [a b]", got)
	}
}

func TestFailNthAndDelete(t *testing.T) {
	ctx := context.Background()
	f := New(nil)
	alice := f.AddUser("alice", 1)
	f.AddRepo("alice", "sandbox", true)
	ref := f.AddIssue("alice", "sandbox", 1, "t", "b", alice)
	boom := errors.New("502")

	f.FailNthThread(2, boom)
	if _, _, err := f.Thread(ctx, ref); err != nil {
		t.Fatalf("1st Thread = %v", err)
	}
	if _, _, err := f.Thread(ctx, ref); !errors.Is(err, boom) {
		t.Fatalf("2nd Thread = %v, want the injected error", err)
	}
	if _, _, err := f.Thread(ctx, ref); err != nil {
		t.Fatalf("3rd Thread = %v", err)
	}

	hooked := 0
	f.AfterPostComment = func(forge.IssueRef) { hooked++ }
	f.FailNthPostComment(1, boom)
	if _, err := f.PostComment(ctx, ref, "x"); !errors.Is(err, boom) {
		t.Fatalf("1st PostComment = %v, want the injected error", err)
	}
	if _, err := f.PostComment(ctx, ref, "y"); err != nil {
		t.Fatalf("2nd PostComment = %v", err)
	}
	if n := len(f.CommentsOf(ref)); n != 1 || hooked != 1 {
		t.Fatalf("comments = %d, hook calls = %d; want 1 and 1", n, hooked)
	}

	other := f.AddIssue("alice", "sandbox", 2, "t", "b", alice)
	f.FailNthThreadOf(ref, 1, boom)
	if _, _, err := f.Thread(ctx, other); err != nil {
		t.Fatalf("Thread of another issue = %v", err)
	}
	if _, _, err := f.Thread(ctx, ref); !errors.Is(err, boom) {
		t.Fatalf("Thread of the issue = %v, want the injected error", err)
	}

	f.DeleteIssue(ref)
	if _, _, err := f.Thread(ctx, ref); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("Thread of a deleted issue = %v, want ErrNotFound", err)
	}
	if list, _ := f.UpdatedIssues(ctx, "alice", "sandbox", time.Time{}); len(list) != 1 || list[0].Ref != other {
		t.Fatalf("UpdatedIssues lists a deleted issue: %+v", list)
	}
}

func TestOpenChangeRequestSkipsClosed(t *testing.T) {
	ctx := context.Background()
	f := New(nil)
	cr := forge.ChangeRequest{Owner: "o", Repo: "r", Head: "switchboard/issue-3", Base: "main"}
	if _, err := f.OpenChangeRequest(ctx, "o", "r", cr.Head, "main"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	url, _ := f.CreateChangeRequest(ctx, cr)
	if got, err := f.OpenChangeRequest(ctx, "o", "r", cr.Head, "main"); err != nil || got != url {
		t.Fatalf("OpenChangeRequest = %q, %v, want %q", got, err, url)
	}
	if _, err := f.OpenChangeRequest(ctx, "o", "r", cr.Head, "dev"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("other base: err = %v, want ErrNotFound", err)
	}
	f.ClosePR(0)
	if _, err := f.OpenChangeRequest(ctx, "o", "r", cr.Head, "main"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("closed: err = %v, want ErrNotFound", err)
	}
}

func TestFakeCI(t *testing.T) {
	ctx := context.Background()
	f := New(nil)
	f.AddRepo("o", "r", true)
	f.SetChecks("o", "r", "abc", forge.Check{Name: "test", Done: true, Conclusion: forge.Failure, Job: 7}, forge.Check{Name: "build"})
	f.SetLog(7, "FAIL")
	checks, err := f.CommitChecks(ctx, "o", "r", "abc")
	if err != nil || len(checks) != 2 || !checks[0].Failed() || checks[1].Done {
		t.Fatalf("CommitChecks = %+v, %v", checks, err)
	}
	if log, _ := f.CheckLog(ctx, "o", "r", checks[0]); log != "FAIL" {
		t.Errorf("CheckLog = %q", log)
	}
	url, _ := f.CreateChangeRequest(ctx, forge.ChangeRequest{Owner: "o", Repo: "r", Head: "switchboard/issue-1", Base: "main"})
	n := forge.ChangeRequestNumber(url)
	f.SetPRHead(n-1, "abc")
	if st, err := f.ChangeRequestHead(ctx, "o", "r", n); err != nil || st.State != forge.CROpen || st.Head != "abc" {
		t.Fatalf("ChangeRequestHead = %+v, %v", st, err)
	}
	f.MergePR(n - 1)
	if st, _ := f.ChangeRequestHead(ctx, "o", "r", n); st.State != forge.CRMerged {
		t.Errorf("state = %s, want merged", st.State)
	}
	f.FailCI = forge.ErrForbidden
	if _, err := f.CommitChecks(ctx, "o", "r", "abc"); err != forge.ErrForbidden {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}
