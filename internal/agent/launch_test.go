package agent

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// A poll that evaluates an issue while its next run is being started, between the claim and the running label, does
// not cancel that run (#25).
func TestPollDuringAStartDoesNotCancelTheRun(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, block(1, waiting("s", "Which default?")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.f.AddComment(x.ref, "Use off.", x.master)

	// On the claim of the second run, poll from another goroutine and give it time to run before the start goes on.
	var once sync.Once
	polled := make(chan struct{})
	x.f.AfterPostComment = func(ref forge.IssueRef) {
		cs := x.f.CommentsOf(ref)
		m, ok := marker.Parse(cs[len(cs)-1].Body)
		if !ok || m.State != "running" {
			return
		}
		once.Do(func() {
			go func() {
				_ = x.a.Poll(x.ctx)
				close(polled)
			}()
			select {
			case <-polled:
			case <-time.After(300 * time.Millisecond):
			}
		})
	}
	x.release(1)
	x.waitFor("the poll during the start", func() bool {
		select {
		case <-polled:
			return true
		default:
			return false
		}
	})
	x.settle()
	if x.audit.has("run.cancel", nil) {
		t.Fatal("the poll canceled the run being started")
	}
	if x.starts() != 2 || !strings.Contains(x.file("prompt-2.md"), "Use off.") || x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
}

// A run of a project removed from the configuration ends as waiting, and the reply it missed starts no next run.
func TestRemovedProjectDoesNotResumeOnAReply(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, block(1, waiting("s", "Which default?")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	x.f.AddComment(x.ref, "Use off.", x.master)
	x.a.Reload(nil)
	x.poll()
	x.release(1)
	x.settle()
	if x.starts() != 1 || x.labels(x.ref) != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.labels(x.ref))
	}
}
