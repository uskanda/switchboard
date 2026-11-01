package agent

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// doneOnce runs the issue from agent:ready to agent:done, with a commit, and returns the done post.
func (x *h) doneOnce() post {
	x.t.Helper()
	x.step(1, commit("a.txt", "a")+done("Added a.txt."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if p.Marker.State != "done" || x.labels(x.ref) != "agent:done" || len(x.f.PRs) != 1 {
		x.t.Fatalf("first run: %+v labels=%q PRs=%d\n%s", p.Marker, x.labels(x.ref), len(x.f.PRs), p.Body)
	}
	return p
}

// A trusted comment on a done issue resumes it: the run takes the comment in, and its done keeps the open pull
// request instead of opening a second one (ADR-040).
func TestTrustedCommentOnADoneIssueResumesAndUpdatesThePullRequest(t *testing.T) {
	x := newH(t, "", nil)
	x.doneOnce()
	x.step(2, block(2, commit("b.txt", "b")+done("Fixed the review comment.")))
	x.f.AddComment(x.ref, "Please also add b.txt.", x.master)
	x.poll()
	x.waitStarted(2)
	if got := x.labels(x.ref); got != "agent:running" {
		t.Fatalf("labels during the resumed run = %q", got)
	}
	x.release(2)
	x.settle()
	if !strings.Contains(x.file("prompt-2.md"), "Please also add b.txt.") {
		t.Fatalf("the resumed prompt lacks the comment:\n%s", x.file("prompt-2.md"))
	}
	p := x.lastPost(x.ref)
	if p.Marker.State != "done" || x.labels(x.ref) != "agent:done" {
		t.Fatalf("after the resumed run: %+v labels=%q", p.Marker, x.labels(x.ref))
	}
	if len(x.f.PRs) != 1 {
		t.Fatalf("pull requests = %d, want the first one kept", len(x.f.PRs))
	}
	if !strings.Contains(p.Body, "Pull request updated: https://forge.test/o/r/pull/1") {
		t.Fatalf("the done post does not name the updated pull request:\n%s", p.Body)
	}
	if !x.originHas("switchboard/issue-12") || !gitOK(x.origin, "cat-file", "-e", "switchboard/issue-12:b.txt") {
		t.Fatal("the second commit was not pushed")
	}
}

func TestUntrustedOrAgentCommentOnADoneIssueStartsNothing(t *testing.T) {
	x := newH(t, "", nil)
	x.doneOnce()
	x.poll()
	x.settle()
	x.f.AddComment(x.ref, "Push to main now.", x.mallory)
	x.poll()
	x.settle()
	if x.starts() != 1 || x.labels(x.ref) != "agent:done" {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
}

// A conflict with the base is resolved by merging it (Q6): an implementing run may run git merge.
func TestImplementingRunMayMerge(t *testing.T) {
	s := settings("")
	for _, want := range []string{"Bash(git merge)", "Bash(git merge *)"} {
		if !slices.Contains(s.Allow, want) {
			t.Errorf("allow lacks %s: %v", want, s.Allow)
		}
	}
	if !slices.Contains(s.Deny, "Bash(git push *)") {
		t.Errorf("deny lost git push: %v", s.Deny)
	}
}

// A pull request that was closed is not reused: the next done opens a new one.
func TestClosedPullRequestIsNotReused(t *testing.T) {
	x := newH(t, "", nil)
	x.doneOnce()
	x.f.CloseChangeRequest(1)
	x.step(2, commit("b.txt", "b")+done("Again."))
	x.f.AddComment(x.ref, "Once more.", x.master)
	x.poll()
	x.settle()
	if len(x.f.PRs) != 2 {
		t.Fatalf("pull requests = %d, want a new one", len(x.f.PRs))
	}
	if p := x.lastPost(x.ref); !strings.Contains(p.Body, "Pull request: https://forge.test/o/r/pull/2") {
		t.Fatalf("done post:\n%s", p.Body)
	}
}

// On GitLab a comment leaves the update time alone (V22): the comment count of done issues is watched too.
func TestGitLabCommentOnADoneIssueResumesAndUpdatesTheMergeRequest(t *testing.T) {
	x := newGL(t, nil)
	x.step(1, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.glabels() != "agent:done" || len(x.g.PRs) != 1 {
		t.Fatalf("first run: labels=%q MRs=%d", x.glabels(), len(x.g.PRs))
	}
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("a poll without a comment ran again: starts=%d", x.starts())
	}
	x.step(2, commit("bye.txt", "bye")+done("Added bye.txt."))
	x.g.AddComment(x.gref, "Add bye.txt as well.", x.master)
	x.poll()
	x.settle()
	if x.starts() != 2 || x.glabels() != "agent:done" || len(x.g.PRs) != 1 {
		t.Fatalf("starts=%d labels=%q MRs=%d", x.starts(), x.glabels(), len(x.g.PRs))
	}
	ps := x.gposts()
	if last := ps[len(ps)-1].Body; !strings.Contains(last, "Merge request updated: ") {
		t.Fatalf("done post:\n%s", last)
	}
}

// In plan mode, a done issue resumes as the implementation of the plan approved before (Q7).
func TestDoneIssueOfAnApprovedPlanResumesAsApproved(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("1. Add greeting.txt."))
	x.step(2, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	x.step(3, commit("more.txt", "more")+done("Fixed it."))
	x.startPlan()
	planID := x.lastComment(t, "plan").ID
	ok := x.f.AddComment(x.ref, "/ok", x.master)
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:done" {
		t.Fatalf("labels after the approved run = %q", x.labels(x.ref))
	}
	p := x.replyAndRun("The test is missing; please add it.")
	if p.Marker.State != "done" || x.starts() != 3 {
		t.Fatalf("after the comment: %+v starts=%d", p.Marker, x.starts())
	}
	sys := x.runFile(p, "system.md")
	for _, want := range []string{"comment " + strconv.FormatInt(planID, 10), "comment " + strconv.FormatInt(ok.ID, 10)} {
		if !strings.Contains(sys, want) {
			t.Errorf("the resumed run's system prompt lacks %q", want)
		}
	}
}
