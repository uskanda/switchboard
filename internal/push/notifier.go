package push

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/state"
)

// sentFile records, by issue, the state of the last notification sent (or of the issue at startup).
const sentFile = "push-sent.json"

// Sender sends one data message to one device; *Client is one.
type Sender interface {
	Send(ctx context.Context, device string, data map[string]string) error
}

type record struct {
	State  string    `json:"state"`
	Reason string    `json:"reason,omitempty"`
	Since  time.Time `json:"since"`
}

// Notifier compares each listing with what was sent before and pushes the issues that newly wait or are done, or
// whose wait changed (its reason, or a new post). The first listing after a start is only recorded, so a restart
// sends nothing. Running and queued issues are never sent, and dropping their records lets the next wait or done
// count as new.
type Notifier struct {
	Sender  Sender
	Devices *Devices
	Dir     state.Dir
	Audit   state.Auditor

	mu     sync.Mutex
	seeded bool
	sent   map[string]record
}

// Observe takes one listing. It returns once every notification is sent or has failed; failures are audited and
// never stop the listing.
func (n *Notifier) Observe(ctx context.Context, in *inboxapi.Inbox) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.sent == nil {
		n.sent = map[string]record{}
		if err := n.Dir.ReadJSON(sentFile, &n.sent); err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "switchboard: push: reading %s: %v\n", sentFile, err)
		}
	}
	var out []Notification
	listed := map[string]bool{}
	for _, it := range in.Items {
		listed[it.Ref] = true
		note, ok := FromItem(it)
		if !ok {
			delete(n.sent, it.Ref)
			continue
		}
		rec := record{State: note.State, Reason: note.Reason, Since: note.Since}
		if prev, had := n.sent[it.Ref]; !had || prev.State != rec.State || prev.Reason != rec.Reason || !prev.Since.Equal(rec.Since) {
			if n.seeded {
				out = append(out, note)
			}
			n.sent[it.Ref] = rec
		}
	}
	// A forge that could not be read leaves its issues out of the listing; they are not gone.
	if len(in.Errors) == 0 {
		for ref := range n.sent {
			if !listed[ref] {
				delete(n.sent, ref)
			}
		}
	}
	n.seeded = true
	if err := n.Dir.WriteJSON(sentFile, n.sent); err != nil {
		fmt.Fprintf(os.Stderr, "switchboard: push: writing %s: %v\n", sentFile, err)
	}
	if len(out) == 0 {
		return
	}
	devices, err := n.Devices.List()
	if err != nil {
		n.Audit.Log("push.failed", map[string]any{"reason": "reading the devices: " + err.Error()})
		return
	}
	for _, note := range out {
		for _, dev := range devices {
			err := n.Sender.Send(ctx, dev, note.Data())
			switch {
			case err == nil:
			case errors.Is(err, ErrUnregistered):
				_ = n.Devices.Remove(dev)
				n.Audit.Log("push.unregistered", map[string]any{"issue": note.Ref})
			default:
				n.Audit.Log("push.failed", map[string]any{"issue": note.Ref, "kind": note.State, "reason": err.Error()})
			}
		}
	}
}
