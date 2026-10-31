package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// notes is a fake Notifier that records what it was asked to send.
type notes struct {
	mu    sync.Mutex
	texts []string
	err   error
}

func (n *notes) Send(_ context.Context, text string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.texts = append(n.texts, text)
	return n.err
}

func (n *notes) sent() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.texts...)
}

func withNotes(n *notes) func(*Agent) { return func(a *Agent) { a.Notifier = n } }

func TestWaitingIsNotifiedOnce(t *testing.T) {
	x := newH(t, "", nil)
	n := &notes{}
	x.step(1, waiting("Looked at the flags.", "Should --dry-run print the plan?"))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.agentWith(withNotes(n))
	x.poll()
	x.settle()
	got := n.sent()
	if len(got) != 1 {
		t.Fatalf("sent %d notifications: %q", len(got), got)
	}
	for _, want := range []string{"⏸ Waiting (question) gh:o/r#12", "Add a flag", "Question: Should --dry-run print the plan?", "https://github.test/o/r/issues/12"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("notification lacks %q:\n%s", want, got[0])
		}
	}
}

func TestDoneIsNotifiedWithThePullRequest(t *testing.T) {
	x := newH(t, "", nil)
	n := &notes{}
	x.step(1, commit("flag.txt", "dry-run")+done("Added --dry-run."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.agentWith(withNotes(n))
	x.poll()
	x.settle()
	got := n.sent()
	if len(got) != 1 || !strings.Contains(got[0], "✅ Done gh:o/r#12") || !strings.Contains(got[0], "PR: https://forge.test/o/r/pull/1") {
		t.Fatalf("notifications: %q", got)
	}
}

func TestWebInboxIsLinkedWhenConfigured(t *testing.T) {
	x := newH(t, "", nil)
	x.cfg.Notify.WebURL = "http://192.168.0.82:8783"
	n := &notes{}
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.agentWith(withNotes(n))
	x.poll()
	x.settle()
	if got := n.sent(); len(got) != 1 || !strings.Contains(got[0], "Inbox: http://192.168.0.82:8783/issue/gh:o%2Fr%2312") {
		t.Fatalf("notifications: %q", got)
	}
}

// A write-back cut off after its comment is finished on the next start without posting again; the notification
// went out with the comment, so the recovery sends none.
func TestInterruptedWriteBackIsNotifiedOnce(t *testing.T) {
	x := newH(t, "", nil)
	n := &notes{}
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.agentWith(withNotes(n))
	x.f.FailNthSetLabels(2, errors.New("connection reset"))
	x.poll()
	x.settle()
	x.agentWith(withNotes(n)) // restart
	if x.labels(x.ref) != "agent:waiting" || len(x.posts(x.ref)) != 1 {
		t.Fatalf("labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
	if got := n.sent(); len(got) != 1 {
		t.Fatalf("sent %d notifications: %q", len(got), got)
	}
}

func TestFailedNotificationIsRecordedAndChangesNothing(t *testing.T) {
	x := newH(t, "", nil)
	n := &notes{err: errors.New("telegram: no answer within 10s")}
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.agentWith(withNotes(n))
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:waiting" || len(x.posts(x.ref)) != 1 {
		t.Fatalf("labels=%q posts=%d", x.labels(x.ref), len(x.posts(x.ref)))
	}
	if !x.audit.has("notify.failed", func(e map[string]any) bool {
		return e["issue"] == "gh:o/r#12" && e["kind"] == "waiting" && strings.Contains(e["reason"].(string), "no answer")
	}) {
		t.Fatalf("no notify.failed in %v", x.audit.events)
	}
}

func TestWithheldCommentKeepsTheSecretOutOfTheNotification(t *testing.T) {
	x := newH(t, "", nil)
	n := &notes{}
	token := "ghp_" + strings.Repeat("Zx9", 12)
	x.step(1, waiting("Found it.", "Rotate "+token+"?"))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.agentWith(withNotes(n))
	x.poll()
	x.settle()
	got := n.sent()
	if len(got) != 1 || strings.Contains(got[0], token) {
		t.Fatalf("notifications: %q", got)
	}
}

func TestPlanWaitsAreNotified(t *testing.T) {
	x := newH(t, "", nil)
	n := &notes{}
	x.step(1, questions([3]string{"Default", "Which default should the flag have?", "Off."}))
	x.step(2, planDecl("Add --dry-run, off by default.\n\nThen test it."))
	x.agentWith(withNotes(n))
	x.startPlan()
	x.replyAndRun("/ok")
	got := n.sent()
	if len(got) != 2 {
		t.Fatalf("sent %d notifications: %q", len(got), got)
	}
	for _, want := range []string{"Waiting (plan-questions)", "Q1", "Default"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("the questions notification lacks %q:\n%s", want, got[0])
		}
	}
	if strings.Contains(got[0], "Read the config package.") {
		t.Errorf("the questions notification quotes the summary, not the question:\n%s", got[0])
	}
	for _, want := range []string{"Waiting (plan)", "Add --dry-run, off by default."} {
		if !strings.Contains(got[1], want) {
			t.Errorf("the plan notification lacks %q:\n%s", want, got[1])
		}
	}
}
