package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// queuedPosts are the agent's queued comments on ref.
func (x *h) queuedPosts(ref forge.IssueRef) []post {
	var out []post
	for _, p := range x.posts(ref) {
		if p.Marker != nil && p.Marker.State == marker.Queued {
			out = append(out, p)
		}
	}
	return out
}

// queueH is the harness with o/r#12 running (blocked on start 1) and o/r#13 ready behind it, on agent.max_parallel 1.
type queueH struct {
	*h
	second forge.IssueRef
	log    *lockedLog
}

func newQueueH(t *testing.T, edit func(*config.Config)) *queueH {
	x := &queueH{h: newH(t, "", edit), log: &lockedLog{}}
	x.second = x.f.AddIssue("o", "r", 13, "Another", "Do another thing.", x.master)
	x.step(1, block(1, waiting("s", "q")))
	x.agentWith(func(a *Agent) { a.Log = x.log })
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.f.AddLabel(x.second, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	return x
}

func TestQueuedIssueGetsOneComment(t *testing.T) {
	x := newQueueH(t, nil)
	x.poll()
	x.poll()
	x.poll()
	qs := x.queuedPosts(x.second)
	if len(qs) != 1 {
		t.Fatalf("queued comments = %d, want 1", len(qs))
	}
	if m := qs[0].Marker; m.Reason != marker.QueuedHost || m.Host != "test" {
		t.Errorf("marker = %+v, want the host limit on host test", m)
	}
	if !strings.Contains(qs[0].Body, "(1)") || x.labels(x.second) != "agent:ready" {
		t.Errorf("body = %q labels = %q; want the limit 1 and agent:ready kept", qs[0].Body, x.labels(x.second))
	}
	if n := strings.Count(x.log.String(), "gh:o/r#13: queued"); n != 1 {
		t.Errorf("queued log lines = %d, want 1\n%s", n, x.log.String())
	}
	x.release(1)
	x.settle()
}

func TestQueuedCommentGoesWhenTheRunStarts(t *testing.T) {
	x := newQueueH(t, nil)
	x.release(1)
	x.settle()
	if n := len(x.queuedPosts(x.second)); n != 0 {
		t.Fatalf("queued comments after the start = %d, want 0", n)
	}
	if len(x.claims(x.second)) != 1 || x.labels(x.second) != "agent:waiting" {
		t.Fatalf("claims = %d labels = %q", len(x.claims(x.second)), x.labels(x.second))
	}
	if !strings.Contains(x.log.String(), "gh:o/r#13: taken up from the queue") {
		t.Errorf("no log of the start from the queue:\n%s", x.log.String())
	}
	p := x.file("prompt-2.md")
	if strings.Contains(p, "Queued") {
		t.Errorf("the run read its queued comment:\n%s", p)
	}
}

func TestQueuedCommentGoesWhenTheLabelIsRemoved(t *testing.T) {
	x := newQueueH(t, nil)
	x.f.RemoveLabel(x.second, "agent:ready")
	x.poll()
	if n := len(x.queuedPosts(x.second)); n != 0 {
		t.Fatalf("queued comments after the label went = %d, want 0", n)
	}
	x.release(1)
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("starts = %d; the issue ran after its label went", x.starts())
	}
}

func TestQueuedCommentOfThePreviousProcessGoesOnStart(t *testing.T) {
	x := newQueueH(t, nil)
	if n := len(x.queuedPosts(x.second)); n != 1 {
		t.Fatalf("queued comments before the restart = %d", n)
	}
	x.stop() // the agent dies while #12 runs and #13 waits for a slot
	x.a.Wait()
	x.a.Close()
	x.release(1)
	x.a = nil
	x.ctx, x.stop = context.WithCancel(context.Background())
	x.agent()
	if n := len(x.queuedPosts(x.second)); n != 0 {
		t.Fatalf("queued comments after the restart = %d, want the old one removed", n)
	}
	x.poll()
	x.settle()
	if x.labels(x.second) != "agent:waiting" || len(x.queuedPosts(x.second)) != 0 {
		t.Fatalf("labels = %q queued = %d; #13 should run once the slot is free", x.labels(x.second), len(x.queuedPosts(x.second)))
	}
}

func TestProjectLimitQueuedCommentNamesNoOtherProject(t *testing.T) {
	x := newQueueH(t, func(c *config.Config) {
		c.Agent.MaxParallel = 3
		c.Projects[0].MaxParallel = limit(1)
	})
	qs := x.queuedPosts(x.second)
	if len(qs) != 1 || qs[0].Marker.Reason != marker.QueuedProject || !strings.Contains(qs[0].Body, "this project") {
		t.Fatalf("queued comments = %+v, want one on the project limit", qs)
	}
	if strings.Contains(qs[0].Body, "o/r") {
		t.Errorf("the comment names a repository: %q", qs[0].Body)
	}
	x.release(1)
	x.settle()
}

func TestQueuedCommentInJapanese(t *testing.T) {
	x := newQueueH(t, func(c *config.Config) { c.Comments.Language = "ja" })
	qs := x.queuedPosts(x.second)
	if len(qs) != 1 || !strings.Contains(qs[0].Body, "順番待ちにしました") || !strings.Contains(qs[0].Body, "端末test") {
		t.Fatalf("queued comments = %+v", qs)
	}
	x.release(1)
	x.settle()
}
