package agent

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

// otherHost starts a second agent, host "other", by mistake assigned the same project as x's agent, with a state
// directory of its own.
func otherHost(t *testing.T, x *h) *Agent {
	t.Helper()
	cfgB := *x.cfg
	cfgB.Host = config.Host{Name: "other"}
	cfgB.Projects = append([]config.Project(nil), x.cfg.Projects...)
	for i := range cfgB.Projects {
		cfgB.Projects[i].Host = "other"
	}
	dirB := state.Dir{Path: filepath.Join(t.TempDir(), "state")}
	jb, err := trust.Resolve(x.ctx, x.f, dirB, x.cfg.TrustedAuthors.GitHub, x.cfg.Forges.GitHub.AgentIdentity, x.audit)
	if err != nil {
		t.Fatal(err)
	}
	b := &Agent{Config: &cfgB, Forge: x.f, Judge: jb, State: dirB, Audit: x.audit, Env: x.a.Env, CheckSandbox: noSandboxCheck,
		IntGrace: 200 * time.Millisecond, TermGrace: 200 * time.Millisecond}
	ctx, cancel := context.WithCancel(x.ctx)
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); b.Wait(); b.Close() })
	return b
}

// The other host reads the issue after this host's claim is posted, before the label moves: it acknowledges the
// same trigger, so the claim rule still finds one winner.
func TestAHostThatReadsTheOthersClaimStillYields(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	a := x.agent()
	b := otherHost(t, x)
	var once sync.Once
	var pb *plan
	x.f.AfterPostComment = func(ref forge.IssueRef) {
		once.Do(func() {
			var err error
			if pb, err = b.decide(x.ctx, b.projects[0], ref); err != nil || pb == nil {
				t.Errorf("b.decide = %v, %v", pb, err)
			}
		})
	}
	pa, err := a.decide(x.ctx, a.projects[0], x.ref)
	if err != nil || pa == nil {
		t.Fatalf("a.decide = %v, %v", pa, err)
	}
	if err := a.launch(x.ctx, pa); err != nil {
		t.Fatal(err)
	}
	a.Wait()
	x.f.AfterPostComment = nil
	if pb == nil {
		t.Fatal("b did not decide")
	}
	if pb.ack() != pa.ack() {
		t.Errorf("ack: a %d, b %d", pa.ack(), pb.ack())
	}
	if err := b.launch(x.ctx, pb); err != nil {
		t.Fatal(err)
	}
	b.Wait()
	if x.starts() != 1 {
		t.Fatalf("claude started %d times", x.starts())
	}
}

// The recovery of a host leaves alone an issue whose latest claim is another host's: that host's run may still go.
func TestRecoveryLeavesAnotherHostsRunAlone(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:running", x.bot)
	x.f.PostCommentAs(x.ref, "Started on host desk.\n\n"+marker.Marker{Host: "desk", Run: "r1", State: marker.Running, At: time.Now()}.Format(), "agent")
	x.agent()
	if x.labels(x.ref) != "agent:running" {
		t.Fatalf("labels after recovery = %q", x.labels(x.ref))
	}
	if ps := x.posts(x.ref); len(ps) != 0 {
		t.Fatalf("posts after recovery = %+v", ps)
	}
	if !x.audit.has("run.not_recovered", func(e map[string]any) bool { return e["host"] == "desk" }) {
		t.Fatal("the skip is not in the audit log")
	}
}

// A project whose visibility cannot be read at the start does not keep the agent from starting: it is taken up on a
// later poll.
func TestAFailedVisibilityAtTheStartWaitsForTheNextPoll(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.FailRepoPrivate = errors.New("502 bad gateway")
	x.agentWith(func(a *Agent) {
		a.CloneURL = func(config.Project) string { return x.origin }
	})
	if n := len(x.a.projectList()); n != 0 {
		t.Fatalf("%d project(s) taken up before the visibility was read", n)
	}
	x.poll()
	x.settle()
	if n := len(x.a.projectList()); n != 1 {
		t.Fatalf("%d project(s) after the poll, want 1", n)
	}
	if x.starts() != 1 {
		t.Fatalf("starts = %d", x.starts())
	}
}
