package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// lastClaim is the marker and the body of the latest claim on ref.
func (x *h) lastClaim(ref forge.IssueRef) (*marker.Marker, string) {
	x.t.Helper()
	claims := x.claims(ref)
	if len(claims) == 0 {
		x.t.Fatal("no claim")
	}
	body := claims[len(claims)-1].Body
	m, _ := marker.Parse(body)
	return m, body
}

// conflictOnce makes the pull request of issue 12 conflict, and runs the one fix run max_fix_attempts: 1 allows
// (claude start 2); the pull request still conflicts afterwards.
func conflictOnce(t *testing.T, edit func(*config.Config)) *ciWorld {
	t.Helper()
	w := newCI(t, "agent:\n  max_fix_attempts: 1\n", edit)
	w.f.SetConflict(0, true)
	w.step(2, commit("merge.txt", "merged")+done("Merged main once."))
	w.poll()
	w.settle()
	if w.starts() != 2 || !w.isConflictFix(2) {
		t.Fatalf("starts=%d: want one fix run of the conflict", w.starts())
	}
	w.syncHead()
	return w
}

func TestAConflictPastTheLimitGoesBackToTheQueue(t *testing.T) {
	w := conflictOnce(t, nil)
	w.step(3, commit("merge2.txt", "merged again")+done("Merged main after the queue."))
	w.poll()
	w.settle()
	if w.starts() != 3 || !w.isConflictFix(3) {
		t.Fatalf("starts=%d: want a conflict fix run taken up from the queue", w.starts())
	}
	m, body := w.lastClaim(w.ref)
	if m.Reason != marker.Fix || m.Requeue != 1 || !strings.Contains(body, "(1/1)") || !strings.Contains(body, "requeue 1/1") {
		t.Fatalf("the claim of the requeued fix run:\n%s", body)
	}
	if len(w.queuedPosts(w.ref)) != 0 {
		t.Error("the queued comment stays after the fix run started")
	}
	if !w.audit.has("comment.posted", func(e map[string]any) bool { return e["state"] == marker.Queued && e["reason"] == marker.Conflict }) {
		t.Error("no queued comment of reason conflict was posted")
	}
	for _, p := range w.posts(w.ref) {
		if p.Marker != nil && p.Marker.State == marker.Waiting {
			t.Fatalf("the issue waited:\n%s", p.Body)
		}
	}
	if w.labels(w.ref) != "agent:done" {
		t.Fatalf("labels = %q", w.labels(w.ref))
	}
}

func TestARequeuedConflictFixWaitsBehindTheQueue(t *testing.T) {
	w := conflictOnce(t, nil)
	other := w.f.AddIssue("o", "r", 13, "Other", "Do something else.", w.master)
	third := w.f.AddIssue("o", "r", 14, "Third", "Do a third thing.", w.master)
	w.step(3, block(3, done("One done.")))
	w.f.AddLabel(other, "agent:ready", w.master)
	w.f.AddLabel(third, "agent:ready", w.master)
	w.poll() // one of o/r#13 and o/r#14 starts and the other queues; then o/r#12 goes back to the queue behind it
	w.waitStarted(3)
	if got := w.labels(w.ref); got != "agent:ready" {
		t.Fatalf("labels = %q, want agent:ready", got)
	}
	qs := w.queuedPosts(w.ref)
	if len(qs) != 1 || qs[0].Marker.Reason != marker.Conflict || qs[0].Marker.Run == "" {
		t.Fatalf("queued posts = %+v", qs)
	}
	if !strings.Contains(qs[0].Body, "#1") {
		t.Errorf("the queued comment does not name the pull request:\n%s", qs[0].Body)
	}
	w.step(4, done("The other done."))
	w.step(5, commit("merge2.txt", "merged again")+done("Merged main after the queue."))
	w.release(3)
	w.settle()
	if w.starts() != 5 || w.isConflictFix(4) || !w.isConflictFix(5) {
		t.Fatalf("starts=%d conflict fix 4=%v 5=%v: want the issue queued first, then the conflict fix of o/r#12", w.starts(), w.isConflictFix(4), w.isConflictFix(5))
	}
	if m, _ := w.lastClaim(w.ref); m.Requeue != 1 {
		t.Fatalf("the claim of o/r#12: %+v", m)
	}
}

func TestARequeuedConflictFixResumesAfterARestart(t *testing.T) {
	w := conflictOnce(t, nil)
	other := w.f.AddIssue("o", "r", 13, "Other", "Do something else.", w.master)
	w.step(3, block(3, done("Other done.")))
	w.f.AddLabel(other, "agent:ready", w.master)
	w.poll()
	w.waitStarted(3)
	w.poll()
	if len(w.queuedPosts(w.ref)) != 1 {
		t.Fatal("o/r#12 did not go back to the queue")
	}
	w.stop()
	w.a.Wait()
	w.a.Close()
	w.a = nil
	w.ctx, w.stop = context.WithCancel(context.Background())
	w.release(3)
	w.step(0, commit("merge2.txt", "merged again")+done("Merged main after the restart."))
	w.agentWith(func(a *Agent) { a.Now = w.now })
	if len(w.queuedPosts(w.ref)) != 1 {
		t.Fatal("the restart removed the queued comment that marks the conflict fix")
	}
	w.poll()
	w.settle()
	m, _ := w.lastClaim(w.ref)
	if m.Requeue != 1 || m.Reason != marker.Fix || len(w.queuedPosts(w.ref)) != 0 {
		t.Fatalf("claim %+v queued=%d: want the conflict fix taken up after the restart", m, len(w.queuedPosts(w.ref)))
	}
}

func TestAConflictPastTheRequeuesWaits(t *testing.T) {
	w := conflictOnce(t, nil)
	w.step(3, commit("merge2.txt", "merged again")+done("Merged main after the queue."))
	w.poll()
	w.settle()
	w.syncHead()
	w.poll()
	w.settle()
	p := w.lastPost(w.ref)
	if w.starts() != 3 || w.labels(w.ref) != "agent:waiting" || p.Marker.State != marker.Waiting || p.Marker.Reason != marker.Conflict {
		t.Fatalf("starts=%d labels=%q marker=%+v\n%s", w.starts(), w.labels(w.ref), p.Marker, p.Body)
	}
	if !strings.Contains(p.Body, "went back to the queue 1 time(s)") {
		t.Fatalf("the wait does not say how often the issue went back to the queue:\n%s", p.Body)
	}
	if len(w.queuedPosts(w.ref)) != 0 {
		t.Error("a queued comment is left")
	}
}

func TestAReplyStartsTheRequeueCountOver(t *testing.T) {
	w := conflictOnce(t, nil)
	w.step(3, commit("merge2.txt", "merged again")+done("Merged main after the queue."))
	w.poll()
	w.settle()
	w.syncHead()
	w.poll() // past the requeues: waits
	w.settle()
	w.f.AddComment(w.ref, "Keep the flag name when you merge.", w.master)
	w.step(4, commit("reply.txt", "replied")+done("Kept the name."))
	w.poll()
	w.settle()
	w.syncHead()
	w.step(5, commit("merge3.txt", "merged")+done("Merged main after the reply."))
	w.poll()
	w.settle()
	w.syncHead()
	w.step(6, commit("merge4.txt", "merged")+done("Merged main from the queue after the reply."))
	w.poll()
	w.settle()
	if w.starts() != 6 {
		t.Fatalf("starts=%d", w.starts())
	}
	m, body := w.lastClaim(w.ref)
	if m.Requeue != 1 || !strings.Contains(body, "(1/1)") {
		t.Fatalf("the reply did not start the counts over:\n%s", body)
	}
}

func TestAConflictWithoutFixesNeverGoesBackToTheQueue(t *testing.T) {
	w := newCI(t, "agent:\n  max_fix_attempts: 0\n", nil)
	w.f.SetConflict(0, true)
	w.poll()
	w.settle()
	if p := w.lastPost(w.ref); p.Marker.State != marker.Waiting || p.Marker.Reason != marker.Conflict || len(w.queuedPosts(w.ref)) != 0 {
		t.Fatalf("marker=%+v queued=%d", p.Marker, len(w.queuedPosts(w.ref)))
	}
}

func TestACIFailurePastTheLimitNeverGoesBackToTheQueue(t *testing.T) {
	w := newCI(t, "agent:\n  max_fix_attempts: 1\n", nil)
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("test", 7))
	w.step(2, commit("fix.txt", "fixed")+done("Fix 1."))
	w.poll()
	w.settle()
	w.f.SetChecks("o", "r", w.syncHead(), failedCheck("test", 8))
	w.poll()
	w.settle()
	if p := w.lastPost(w.ref); p.Marker.Reason != marker.CI || w.labels(w.ref) != "agent:waiting" || len(w.queuedPosts(w.ref)) != 0 || w.starts() != 2 {
		t.Fatalf("marker=%+v labels=%q queued=%d starts=%d", p.Marker, w.labels(w.ref), len(w.queuedPosts(w.ref)), w.starts())
	}
}

func TestTheRequeuedConflictFixIsInJapanese(t *testing.T) {
	w := conflictOnce(t, func(c *config.Config) { c.Comments.Language = "ja" })
	w.step(3, commit("merge2.txt", "merged again")+done("Merged main after the queue."))
	w.poll()
	w.settle()
	_, body := w.lastClaim(w.ref)
	if !strings.Contains(body, "baseとの衝突を解消します（1/1）") || !strings.Contains(body, "順番待ちから再開しました（入り直し1/1）") {
		t.Fatalf("the claim:\n%s", body)
	}
}
