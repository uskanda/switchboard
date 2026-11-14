package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// limitH is h with the agent's GitHub calls counted, and the agent's clock following the fake forge's.
type limitH struct {
	*h
	cf  *countingForge
	log *lockedLog
}

func newLimitH(t *testing.T) *limitH {
	x := &limitH{h: newH(t, "", nil), log: &lockedLog{}}
	x.cf = newCountingForge(x.f)
	x.cf.now = x.clock
	x.agentWith(func(a *Agent) { a.Forge, a.Now, a.Log = x.cf, x.clock, x.log })
	x.cf.take() // the calls of the start
	return x
}

func (x *limitH) clock() time.Time { return time.Now().Add(time.Duration(x.skew.Load())) }

func TestRateLimitStopsTheReadsUntilTheReset(t *testing.T) {
	x := newLimitH(t)
	others := []forge.IssueRef{
		x.f.AddIssue("o", "r", 13, "Second", "Do it.", x.master),
		x.f.AddIssue("o", "r", 14, "Third", "Do it.", x.master),
	}
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	for _, r := range others {
		x.f.AddLabel(r, "agent:ready", x.master)
	}
	x.cf.limit(1, x.clock().Add(30*time.Minute)) // the listing goes through, the first read is refused
	x.poll()
	if n := x.cf.take(); n["UpdatedIssues"] != 1 || n["Thread"] != 1 || len(n) != 2 {
		t.Fatalf("the poll that met the limit called %v, want the listing and one read", n)
	}
	for i := 0; i < 3; i++ {
		x.advance(time.Minute)
		x.poll()
	}
	if n := x.cf.take(); len(n) != 0 {
		t.Errorf("polls under the limit called the forge: %v", n)
	}
	if n := strings.Count(x.log.String(), "the API rate limit is used up"); n != 1 {
		t.Errorf("limit log lines = %d, want 1\n%s", n, x.log.String())
	}
	if x.starts() != 0 || strings.Contains(x.log.String(), "403") {
		t.Errorf("starts=%d; the refusals must not be logged as errors:\n%s", x.starts(), x.log.String())
	}

	x.advance(30 * time.Minute)
	x.poll()
	x.settle()
	for _, r := range append(others, x.ref) {
		if x.labels(r) != "agent:waiting" {
			t.Errorf("%s: labels %q after the reset, want agent:waiting", r, x.labels(r))
		}
	}
	if n := strings.Count(x.log.String(), "the API rate limit has reset"); n != 1 {
		t.Errorf("reset log lines = %d, want 1\n%s", n, x.log.String())
	}
}

func TestRateLimitKeepsTheQueue(t *testing.T) {
	x := newLimitH(t)
	second := x.f.AddIssue("o", "r", 13, "Second", "Do it.", x.master)
	x.step(1, block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.AddLabel(second, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.a.noteLimit(second, &forge.RateLimitError{Reset: x.clock().Add(30 * time.Minute), Err: forge.ErrForbidden})
	x.release(1)
	x.settle()
	x.a.mu.Lock()
	queue := x.a.queueOrder()
	x.a.mu.Unlock()
	if x.starts() != 1 || len(queue) != 1 || queue[0] != second {
		t.Fatalf("starts=%d queue=%v: the queued issue must wait out the limit", x.starts(), queue)
	}
	x.advance(31 * time.Minute)
	x.poll()
	x.settle()
	if x.starts() != 2 || x.labels(second) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q: the queued issue must run after the reset", x.starts(), x.labels(second))
	}
}

func TestRateLimitOfGitHubLeavesGitLabAlone(t *testing.T) {
	x := newGL(t, nil)
	x.agent()
	x.a.noteLimit(x.ref, &forge.RateLimitError{Reset: time.Now().Add(30 * time.Minute), Err: forge.ErrForbidden})
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.glabels() != "agent:waiting" {
		t.Fatalf("GitLab labels = %q under a GitHub limit, want agent:waiting", x.glabels())
	}
}
