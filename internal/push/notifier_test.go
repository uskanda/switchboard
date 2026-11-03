package push

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/state"
)

type sent struct {
	device string
	data   map[string]string
}

type fakeSender struct {
	mu   sync.Mutex
	sent []sent
	fail map[string]error // by device
}

func (f *fakeSender) Send(_ context.Context, device string, data map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail[device]; err != nil {
		return err
	}
	f.sent = append(f.sent, sent{device, data})
	return nil
}

func (f *fakeSender) titles() []string {
	var out []string
	for _, s := range f.sent {
		out = append(out, s.device+":"+s.data["title"])
	}
	return out
}

type audit struct {
	events []string
	fields []map[string]any
}

func (a *audit) Log(event string, fields map[string]any) {
	a.events = append(a.events, event)
	a.fields = append(a.fields, fields)
}

func newNotifier(t *testing.T, devices ...string) (*Notifier, *fakeSender, *audit) {
	t.Helper()
	dir := state.Dir{Path: t.TempDir()}
	d := &Devices{Dir: dir}
	for _, tok := range devices {
		if err := d.Register(tok, at); err != nil {
			t.Fatal(err)
		}
	}
	s, a := &fakeSender{fail: map[string]error{}}, &audit{}
	return &Notifier{Sender: s, Devices: d, Dir: dir, Audit: a}, s, a
}

func listing(items ...inboxapi.Item) *inboxapi.Inbox { return &inboxapi.Inbox{Items: items} }

func TestTheFirstListingIsOnlyRecorded(t *testing.T) {
	n, s, _ := newNotifier(t, "phone")
	n.Observe(context.Background(), listing(waiting("gh:o/r#1", "question", at), waiting("gh:o/r#2", "plan", at), done("gh:o/r#3", at, 4, "s")))
	if len(s.sent) != 0 {
		t.Fatalf("sent %v on the first listing", s.titles())
	}
	n.Observe(context.Background(), listing(waiting("gh:o/r#1", "question", at), waiting("gh:o/r#2", "plan", at), done("gh:o/r#3", at, 4, "s")))
	if len(s.sent) != 0 {
		t.Fatalf("sent %v for issues already recorded", s.titles())
	}
}

func TestANewWaitIsSentToEveryDevice(t *testing.T) {
	n, s, _ := newNotifier(t, "phone", "tablet")
	n.Observe(context.Background(), listing())
	n.Observe(context.Background(), listing(waiting("gh:o/r#12", "question", at)))
	if got := strings.Join(s.titles(), ","); got != "phone:o/r#12 question,tablet:o/r#12 question" {
		t.Fatalf("sent %s", got)
	}
}

func TestChangesThatAreSent(t *testing.T) {
	later := at.Add(time.Hour)
	for name, c := range map[string]struct {
		before, after inboxapi.Item
		want          string
	}{
		"plan posted again": {waiting("gh:o/r#1", "plan", at), waiting("gh:o/r#1", "plan", later), "phone:o/r#1 plan"},
		"reason changes":    {waiting("gh:o/r#1", "plan-questions", at), waiting("gh:o/r#1", "plan", at), "phone:o/r#1 plan"},
		"waiting to done":   {waiting("gh:o/r#1", "question", at), done("gh:o/r#1", later, 2, "s"), "phone:o/r#1 done"},
		"unchanged":         {waiting("gh:o/r#1", "question", at), waiting("gh:o/r#1", "question", at), ""},
	} {
		t.Run(name, func(t *testing.T) {
			n, s, _ := newNotifier(t, "phone")
			n.Observe(context.Background(), listing(c.before))
			n.Observe(context.Background(), listing(c.after))
			if got := strings.Join(s.titles(), ","); got != c.want {
				t.Fatalf("sent %q, want %q", got, c.want)
			}
		})
	}
}

func TestRunningAndQueuedSendNothing(t *testing.T) {
	n, s, _ := newNotifier(t, "phone")
	n.Observe(context.Background(), listing())
	n.Observe(context.Background(), listing(inboxapi.Item{State: "running", Ref: "gh:o/r#1"}, inboxapi.Item{State: "queued", Ref: "gh:o/r#2"}))
	if len(s.sent) != 0 {
		t.Fatalf("sent %v", s.titles())
	}
}

func TestAnIssueThatLeavesAndComesBackIsSentAgain(t *testing.T) {
	n, s, _ := newNotifier(t, "phone")
	n.Observe(context.Background(), listing(done("gh:o/r#1", at, 2, "s")))
	n.Observe(context.Background(), listing())
	n.Observe(context.Background(), listing(done("gh:o/r#1", at, 2, "s")))
	if got := strings.Join(s.titles(), ","); got != "phone:o/r#1 done" {
		t.Fatalf("sent %q", got)
	}
}

func TestAListingWithAForgeErrorKeepsTheRecords(t *testing.T) {
	n, s, _ := newNotifier(t, "phone")
	n.Observe(context.Background(), listing(waiting("gl:g/p#1", "question", at)))
	n.Observe(context.Background(), &inboxapi.Inbox{Errors: []inboxapi.ForgeError{{Forge: "gitlab", Message: "down"}}})
	n.Observe(context.Background(), listing(waiting("gl:g/p#1", "question", at)))
	if len(s.sent) != 0 {
		t.Fatalf("sent %v after the forge came back", s.titles())
	}
}

func TestTheRecordSurvivesARestart(t *testing.T) {
	n, s, _ := newNotifier(t, "phone")
	n.Observe(context.Background(), listing())
	n.Observe(context.Background(), listing(waiting("gh:o/r#1", "question", at)))
	restarted := &Notifier{Sender: s, Devices: n.Devices, Dir: n.Dir, Audit: n.Audit}
	restarted.Observe(context.Background(), listing(waiting("gh:o/r#1", "question", at), waiting("gh:o/r#2", "question", at)))
	restarted.Observe(context.Background(), listing(waiting("gh:o/r#1", "question", at), waiting("gh:o/r#2", "question", at)))
	if got := strings.Join(s.titles(), ","); got != "phone:o/r#1 question" {
		t.Fatalf("sent %q, want only the wait before the restart", got)
	}
}

func TestAFailureIsAuditedAndTheOtherDevicesStillGetIt(t *testing.T) {
	n, s, a := newNotifier(t, "broken", "phone")
	s.fail["broken"] = errors.New("fcm: 500 INTERNAL")
	n.Observe(context.Background(), listing())
	n.Observe(context.Background(), listing(waiting("gh:o/r#1", "question", at)))
	if got := strings.Join(s.titles(), ","); got != "phone:o/r#1 question" {
		t.Fatalf("sent %q", got)
	}
	if len(a.events) != 1 || a.events[0] != "push.failed" || a.fields[0]["issue"] != "gh:o/r#1" || a.fields[0]["reason"] != "fcm: 500 INTERNAL" {
		t.Fatalf("audit = %v %v", a.events, a.fields)
	}
	for _, v := range a.fields[0] {
		if s, ok := v.(string); ok && strings.Contains(s, "broken") {
			t.Fatalf("the audit holds the device token: %v", a.fields[0])
		}
	}
}

func TestAnUnregisteredDeviceIsRemoved(t *testing.T) {
	n, s, _ := newNotifier(t, "gone", "phone")
	s.fail["gone"] = ErrUnregistered
	n.Observe(context.Background(), listing())
	n.Observe(context.Background(), listing(waiting("gh:o/r#1", "question", at)))
	got, _ := n.Devices.List()
	if len(got) != 1 || got[0] != "phone" {
		t.Fatalf("devices = %v", got)
	}
}
