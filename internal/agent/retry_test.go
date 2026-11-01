package agent

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/marker"
)

// pendingWriteBacks is how many write-backs the agent keeps to try again.
func (x *h) pendingWriteBacks() int {
	x.a.mu.Lock()
	defer x.a.mu.Unlock()
	return len(x.a.writebacks)
}

// The label of a write-back that failed once is set by the next poll, without a second comment (#26).
func TestFailedLabelOfAWriteBackIsSetByTheNextPoll(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthSetLabels(2, errBadGateway) // the first sets running, the second waiting
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:running" || len(x.posts(x.ref)) != 1 {
		t.Fatalf("after the failure: labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:waiting" || len(x.posts(x.ref)) != 1 || x.starts() != 1 {
		t.Fatalf("after the retry: labels=%q posts=%d starts=%d", x.labels(x.ref), len(x.posts(x.ref)), x.starts())
	}
	if !x.audit.has("writeback.failed", nil) || !x.audit.has("writeback.retried", nil) || x.pendingWriteBacks() != 0 {
		t.Fatalf("audit or list: failed=%v retried=%v pending=%d",
			x.audit.has("writeback.failed", nil), x.audit.has("writeback.retried", nil), x.pendingWriteBacks())
	}
}

// A write-back that fails before it posts anything posts its comment on the next poll.
func TestWriteBackThatFailedBeforePostingPostsOnTheNextPoll(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, block(1, waiting("s", "Which default?")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.f.FailNthThreadOf(x.ref, 1, errBadGateway)
	x.release(1)
	x.settle()
	if x.labels(x.ref) != "agent:running" || len(x.posts(x.ref)) != 0 {
		t.Fatalf("after the failure: labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:waiting" || len(x.posts(x.ref)) != 1 || x.lastPost(x.ref).Marker.State != "waiting" {
		t.Fatalf("after the retry: labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
}

// A GitLab issue made waiting by a retry keeps its note count, so that the next poll takes nothing for a reply.
func TestGitLabRetryKeepsTheNoteCount(t *testing.T) {
	x := newGL(t, nil)
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.g.FailNthSetLabels(2, errBadGateway)
	x.poll()
	x.settle()
	x.poll()
	x.settle()
	if x.glabels() != "agent:waiting" {
		t.Fatalf("labels=%q", x.glabels())
	}
	x.a.mu.Lock()
	kept := x.a.notes[x.gref]
	x.a.mu.Unlock()
	if want := len(x.g.CommentsOf(x.gref)); kept != want {
		t.Fatalf("note count kept = %d, want %d", kept, want)
	}
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("a poll without a reply ran again: starts=%d", x.starts())
	}
}

// A reply that arrived while the write-back waited starts the next run as soon as the retry makes the issue wait.
func TestRetryResumesOnAReplyThatArrivedMeanwhile(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthSetLabels(2, errBadGateway)
	x.poll()
	x.settle()
	x.f.AddComment(x.ref, "Use off.", x.master)
	x.poll()
	x.settle()
	if x.starts() != 2 || !strings.Contains(x.file("prompt-2.md"), "Use off.") {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
}

// A project removed from the configuration still gets its write-back, and no next run.
func TestRemovedProjectKeepsRetryingItsWriteBack(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthSetLabels(2, errBadGateway)
	x.poll()
	x.settle()
	x.f.AddComment(x.ref, "Use off.", x.master)
	x.a.Reload(&config.Change{MaxParallel: x.cfg.Agent.MaxParallel})
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:waiting" || x.starts() != 1 || x.pendingWriteBacks() != 0 {
		t.Fatalf("labels=%q starts=%d pending=%d", x.labels(x.ref), x.starts(), x.pendingWriteBacks())
	}
}

// A done comment that could not be posted is posted when the issue was closed meanwhile; the labels stay, and no
// notification goes out.
func TestRetryOnAClosedIssuePostsWithoutLabelsOrNotification(t *testing.T) {
	x := newH(t, "", nil)
	n := &notes{}
	x.step(1, commit("a.txt", "a")+done("Added a.txt."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.agentWith(withNotes(n))
	x.f.FailNthPostComment(2, errBadGateway) // the first posts the claim, the second the done comment
	x.poll()
	x.settle()
	if len(x.posts(x.ref)) != 0 || x.labels(x.ref) != "agent:running" {
		t.Fatalf("after the failure: labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
	x.f.Close(x.ref)
	x.poll()
	x.settle()
	if len(x.posts(x.ref)) != 1 || x.lastPost(x.ref).Marker.State != "done" || x.labels(x.ref) != "agent:running" {
		t.Fatalf("after the retry: labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
	if len(n.sent()) != 0 || x.pendingWriteBacks() != 0 || !x.audit.has("writeback.abandoned", nil) {
		t.Fatalf("notified=%d pending=%d", len(n.sent()), x.pendingWriteBacks())
	}
	x.poll()
	if len(x.posts(x.ref)) != 1 {
		t.Fatal("the retry went on after it was given up")
	}
}

// A user who moved the issue back to ready takes it over: the retry leaves the labels, and the issue runs as ready.
func TestRetryLeavesLabelsAUserChanged(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthSetLabels(2, errBadGateway)
	x.poll()
	x.settle()
	x.f.RemoveLabel(x.ref, "agent:running")
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if !x.audit.has("writeback.abandoned", nil) || x.pendingWriteBacks() != 0 || x.starts() != 2 {
		t.Fatalf("labels=%q starts=%d pending=%d", x.labels(x.ref), x.starts(), x.pendingWriteBacks())
	}
}

// A later claim of another run means the issue is no longer this run's: the retry leaves the labels.
func TestRetryLeavesAnIssueAnotherRunClaimed(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthSetLabels(2, errBadGateway)
	x.poll()
	x.settle()
	m := marker.Marker{Host: "other", Run: "0123456789abcdef", State: "running", At: time.Now()}
	if _, err := x.f.PostComment(x.ctx, x.ref, "claimed\n\n"+m.Format()+"\n"); err != nil {
		t.Fatal(err)
	}
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:running" || len(x.posts(x.ref)) != 1 || x.pendingWriteBacks() != 0 {
		t.Fatalf("labels=%q posts=%d pending=%d", x.labels(x.ref), len(x.posts(x.ref)), x.pendingWriteBacks())
	}
}

// A start whose running label was set but could not be read back becomes an interrupted run on the next poll.
func TestStartWhoseLabelWasSetIsInterruptedOnTheNextPoll(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	// decide, the claim, and the swap's read before setting the label succeed; its read afterwards fails.
	x.f.FailNthThreadOf(x.ref, 4, errBadGateway)
	if err := x.pollErr(); err == nil {
		t.Fatal("the poll reported no error")
	}
	x.settle()
	if x.labels(x.ref) != "agent:running" || x.starts() != 0 {
		t.Fatalf("after the failure: labels=%q starts=%d", x.labels(x.ref), x.starts())
	}
	x.poll()
	x.settle()
	p := x.lastPost(x.ref)
	if x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != "error" || !strings.Contains(p.Body, "interrupted") || x.starts() != 0 {
		t.Fatalf("labels=%q starts=%d post=%+v\n%s", x.labels(x.ref), x.starts(), p.Marker, p.Body)
	}
}

// A start whose running label was not set is started again by the next poll.
func TestStartWhoseLabelWasNotSetStartsAgain(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthSetLabels(1, errBadGateway)
	if err := x.pollErr(); err == nil {
		t.Fatal("the poll reported no error")
	}
	x.settle()
	if x.labels(x.ref) != "agent:ready" || x.starts() != 0 {
		t.Fatalf("after the failure: labels=%q starts=%d", x.labels(x.ref), x.starts())
	}
	x.poll()
	x.settle()
	if x.starts() != 1 || x.labels(x.ref) != "agent:waiting" || x.pendingWriteBacks() != 0 {
		t.Fatalf("labels=%q starts=%d pending=%d", x.labels(x.ref), x.starts(), x.pendingWriteBacks())
	}
}

// A recovery at start that cannot write back does not stop the agent; the next poll finishes it.
func TestFailedRecoveryDoesNotStopTheStart(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:running", x.bot)
	x.f.FailNthPostComment(1, errBadGateway)
	x.agent() // fails the test when Start fails
	if x.labels(x.ref) != "agent:running" || x.pendingWriteBacks() != 1 {
		t.Fatalf("after the start: labels=%q pending=%d", x.labels(x.ref), x.pendingWriteBacks())
	}
	x.poll()
	p := x.lastPost(x.ref)
	if x.labels(x.ref) != "agent:waiting" || p.Marker.Reason != "error" {
		t.Fatalf("labels=%q post=%+v", x.labels(x.ref), p.Marker)
	}
}

// A recovery that cannot even search for the running issues fails the start.
func TestFailedRecoverySearchFailsTheStart(t *testing.T) {
	x := newH(t, "", nil)
	x.f.FailSearch = errBadGateway
	a := &Agent{Config: x.cfg, Forge: x.f, State: x.dir, Audit: x.audit}
	if err := a.Start(x.ctx); err == nil {
		a.Close()
		t.Fatal("the start succeeded without searching for the running issues")
	}
	a.Close()
}

// agent status shows a run whose write-back waits apart from the running ones.
func TestStatusShowsAPendingWriteBack(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthSetLabels(2, errBadGateway)
	x.poll()
	x.settle()
	var out bytes.Buffer
	if err := Status(x.cfg, x.dir, &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "o/r#12  write-back pending since ") || !strings.Contains(s, "retried at each poll") || strings.Contains(s, "running since") {
		t.Fatalf("status:\n%s", s)
	}
	x.a.Close()
	out.Reset()
	if err := Status(x.cfg, x.dir, &out); err != nil {
		t.Fatal(err)
	}
	if s := out.String(); !strings.Contains(s, "write-back pending since ") || !strings.Contains(s, "recovered when the agent starts") {
		t.Fatalf("status without an agent:\n%s", s)
	}
}
