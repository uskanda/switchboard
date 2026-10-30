package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

// One host claims the run with a comment before claude starts; the claim stays, and the outcome is still posted.
func TestRunIsClaimedAndTheOutcomeStillPosted(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	cs := x.claims(x.ref)
	if len(cs) != 1 {
		t.Fatalf("claims = %+v", cs)
	}
	m, _ := marker.Parse(cs[0].Body)
	if m.Host != "test" || m.Run == "" {
		t.Fatalf("claim marker = %+v", m)
	}
	ps := x.posts(x.ref)
	if len(ps) != 1 || ps[0].Marker.State != "waiting" || ps[0].Marker.Run != m.Run {
		t.Fatalf("posts = %+v", ps)
	}
	if !strings.Contains(x.file("count"), "1") || x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
	// The claim is not the agent's latest question: the reply resumes the issue as before.
	x.f.AddComment(x.ref, "Off by default.", x.master)
	x.poll()
	x.settle()
	if x.starts() != 2 || len(x.claims(x.ref)) != 2 {
		t.Fatalf("after the reply: starts=%d claims=%d", x.starts(), len(x.claims(x.ref)))
	}
}

// Two hosts, by mistake both assigned the project, decide to run the same issue at the same time. The first claim
// wins: claude starts once, and the other host removes its claim and leaves the labels alone.
func TestTwoHostsClaimTheSameIssue(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	a := x.agent()

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
	b := &Agent{Config: &cfgB, Forge: x.f, Judge: jb, State: dirB, Audit: x.audit, Env: a.Env,
		IntGrace: 200 * time.Millisecond, TermGrace: 200 * time.Millisecond}
	ctx, cancel := context.WithCancel(x.ctx)
	defer cancel()
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { b.Wait(); b.Close() }()

	// Both decide before either has swapped the label.
	pa, err := a.decide(x.ctx, a.projects[0], x.ref)
	if err != nil || pa == nil {
		t.Fatalf("a.decide = %v, %v", pa, err)
	}
	pb, err := b.decide(ctx, b.projects[0], x.ref)
	if err != nil || pb == nil {
		t.Fatalf("b.decide = %v, %v", pb, err)
	}
	if err := a.launch(x.ctx, pa); err != nil {
		t.Fatal(err)
	}
	a.Wait() // a's claim is posted, and its run ends, before b claims
	if err := b.launch(ctx, pb); err != nil {
		t.Fatal(err)
	}
	b.Wait()

	if x.starts() != 1 {
		t.Fatalf("claude started %d times", x.starts())
	}
	cs := x.claims(x.ref)
	if len(cs) != 1 {
		t.Fatalf("claims left = %+v", cs)
	}
	if m, _ := marker.Parse(cs[0].Body); m.Host != "test" {
		t.Fatalf("the remaining claim is from %s", m.Host)
	}
	if x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("labels = %q", x.labels(x.ref))
	}
	if !x.audit.has("run.yielded", func(e map[string]any) bool { return e["winner"] == "test" }) {
		t.Fatal("the yield is not in the audit log")
	}
}

// A claim left after the question (by a host that lost and failed to remove it) does not stop a reply from
// resuming the issue.
func TestALeftoverClaimDoesNotBlockTheReply(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	x.f.PostCommentAs(x.ref, "Started on host desk.\n\n"+marker.Marker{Host: "desk", Run: "zz", State: "running", Ack: 1}.Format(), "agent")
	x.f.AddComment(x.ref, "Off by default.", x.master)
	x.poll()
	x.settle()
	if x.starts() != 2 {
		t.Fatalf("the reply did not resume the issue: starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
}

// After a restart, an issue left running with a claim but no outcome becomes waiting, as before.
func TestRecoveryWithAClaim(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.stop() // the agent dies mid-run
	x.a.Wait()
	x.a.Close()
	x.release(1)
	x.a = nil
	x.ctx, x.stop = context.WithCancel(context.Background())
	x.agent()
	if x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("labels after recovery = %q", x.labels(x.ref))
	}
	if ps := x.posts(x.ref); len(ps) != 1 || ps[0].Marker.Reason != "error" {
		t.Fatalf("posts after recovery = %+v", ps)
	}
}
