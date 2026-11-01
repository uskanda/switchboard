package agent

import (
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// While connected with a webhook for every project, an evaluation that failed is tried again at poll_interval, not
// at reconcile_interval; once it succeeded, the loop goes back to reconcile_interval.
func TestFailedEvaluationIsRetriedAtThePollInterval(t *testing.T) {
	x := newH(t, "", nil)
	_, stop := runConnected(t, x, []config.Hook{{Project: "gh:o/r", Key: "k", SecretRef: "s"}})
	defer stop()
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailNthThreadOf(x.ref, 1, errBadGateway) // the refresh for the event fails to read the issue
	x.a.onEvent(ours)
	x.waitFor("the retried evaluation to start claude", func() bool { return x.starts() == 1 })
	x.waitFor("the interval to go back to reconcile_interval", func() bool { return x.a.interval() == time.Hour })
}

// Only an issue that waits for a slot does not shorten the interval: the end of a run drains the queue anyway.
func TestQueuedIssuesKeepTheReconcileInterval(t *testing.T) {
	x := newH(t, "", nil)
	x.cfg.Agent.PollInterval = 30 * time.Millisecond
	x.cfg.Agent.ReconcileInterval = time.Hour
	x.cfg.Relay.Hooks = []config.Hook{{Project: "gh:o/r", Key: "k", SecretRef: "s"}}
	x.agent()
	x.a.onRelayState(true)
	x.a.rmu.Lock()
	x.a.reconciled = time.Now()
	x.a.rmu.Unlock()
	other := forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: 99}
	x.a.unsettle(other, time.Now(), true)
	if got := x.a.interval(); got != time.Hour {
		t.Fatalf("interval with a queued issue = %s", got)
	}
	if got := x.a.due(time.Now()); len(got) != 0 {
		t.Fatalf("due with a queued issue = %v", got)
	}
	x.a.unsettle(other, time.Now(), false)
	if got := x.a.interval(); got != 30*time.Millisecond {
		t.Fatalf("interval with an issue to evaluate again = %s", got)
	}
	if got := x.a.due(time.Now()); len(got) != 1 || got[0].Ref != "gh:o/r" {
		t.Fatalf("due with an issue to evaluate again = %v", got)
	}
}
