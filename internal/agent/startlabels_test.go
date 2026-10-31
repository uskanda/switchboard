package agent

import (
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/marker"
)

func TestGoLabelStartsARun(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:go", x.master)
	x.poll()
	x.settle()
	if x.starts() != 1 || x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
}

func TestUntrustedGoLabelStartsNothing(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:go", x.mallory)
	x.poll()
	x.settle()
	if x.starts() != 0 || len(x.posts(x.ref)) != 0 || x.labels(x.ref) != "agent:go" {
		t.Fatalf("starts=%d comments=%d labels=%q", x.starts(), len(x.posts(x.ref)), x.labels(x.ref))
	}
	if !x.audit.has("trust.label", func(e map[string]any) bool { return e["verdict"] == "reject" }) {
		t.Fatal("the rejection is not in the audit log")
	}
}

func TestUntrustedGoLabelStartsNothingOnGitLab(t *testing.T) {
	x := newGL(t, nil)
	mallory := x.g.AddUser("mallory", 3)
	x.g.AddLabel(x.gref, "agent:go", mallory)
	x.poll()
	x.settle()
	if x.starts() != 0 || len(x.g.CommentsOf(x.gref)) != 0 || x.glabels() != "agent:go" {
		t.Fatalf("starts=%d comments=%d labels=%q", x.starts(), len(x.g.CommentsOf(x.gref)), x.glabels())
	}
}

func claimMode(t *testing.T, x *h, i int) string {
	t.Helper()
	cs := x.claims(x.ref)
	if len(cs) <= i {
		t.Fatalf("claims = %d, want more than %d", len(cs), i)
	}
	m, _ := marker.Parse(cs[i].Body)
	return m.Mode
}

func TestTheStartLabelSetsTheModeOfTheClaim(t *testing.T) {
	for label, want := range map[string]string{"agent:plan": "plan", "agent:go": "direct", "agent:ready": "direct"} {
		x := newH(t, "", nil)
		x.f.AddLabel(x.ref, label, x.master)
		x.poll()
		x.settle()
		if got := claimMode(t, x, 0); got != want {
			t.Errorf("%s: claim mode = %q, want %q", label, got, want)
		}
	}
}

func TestPlanFirstMakesTheReadyLabelPlan(t *testing.T) {
	x := newH(t, "", func(c *config.Config) { c.Projects[0].PlanFirst = true })
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if got := claimMode(t, x, 0); got != "plan" {
		t.Fatalf("claim mode = %q, want plan", got)
	}
}

func TestAResumedRunsClaimHasNoMode(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:go", x.master)
	x.poll()
	x.settle()
	x.f.AddComment(x.ref, "Use the first.", x.master)
	x.poll()
	x.settle()
	if x.starts() != 2 {
		t.Fatalf("starts = %d, want 2", x.starts())
	}
	if got := claimMode(t, x, 1); got != "" {
		t.Fatalf("the resumed claim has mode %q", got)
	}
}

func TestTwoStartLabelsWaitWithAnError(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:plan", x.master)
	x.f.AddLabel(x.ref, "agent:go", x.master)
	x.poll()
	x.settle()
	if x.starts() != 0 {
		t.Fatalf("claude ran %d times", x.starts())
	}
	if got := x.labels(x.ref); got != "agent:waiting" {
		t.Fatalf("labels = %q, want agent:waiting", got)
	}
	p := x.lastPost(x.ref)
	if p.Marker.Reason != "error" || !strings.Contains(p.Body, "`agent:plan`") || !strings.Contains(p.Body, "`agent:go`") {
		t.Fatalf("post = %+v\n%s", p.Marker, p.Body)
	}
	if got := claimMode(t, x, 0); got != "" {
		t.Fatalf("the claim of the clash has mode %q", got)
	}
}

func TestTwoStartLabelsWithAnUntrustedOneWriteNothing(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:plan", x.master)
	x.f.AddLabel(x.ref, "agent:go", x.mallory)
	x.poll()
	x.settle()
	if x.starts() != 0 || len(x.f.CommentsOf(x.ref)) != 0 {
		t.Fatalf("starts=%d comments=%d", x.starts(), len(x.f.CommentsOf(x.ref)))
	}
	if got := x.labels(x.ref); got != "agent:go,agent:plan" {
		t.Fatalf("labels = %q", got)
	}
}
