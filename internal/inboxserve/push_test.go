package inboxserve

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/push"
)

type registry struct {
	tokens []string
	err    error
}

func (r *registry) Register(token string, _ time.Time) error {
	if r.err != nil {
		return r.err
	}
	r.tokens = append(r.tokens, token)
	return nil
}

func (w *world) post(path string, body any) (int, inboxapi.Error) {
	data, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	w.srv.ServeHTTP(rec, req)
	var e inboxapi.Error
	_ = json.Unmarshal(rec.Body.Bytes(), &e)
	return rec.Code, e
}

func TestDevicesIsServedOnlyWithDevices(t *testing.T) {
	w := newServer(t)
	if st, _ := w.post("/v1/devices", inboxapi.RegisterDeviceRequest{Token: "t"}); st != http.StatusNotFound {
		t.Fatalf("without Devices: %d", st)
	}
}

func TestRegisterDevice(t *testing.T) {
	w := newServer(t)
	reg := &registry{}
	w.srv.Devices = reg
	if st, e := w.post("/v1/devices", inboxapi.RegisterDeviceRequest{Token: "fcm-1"}); st != http.StatusNoContent {
		t.Fatalf("register: %d %+v", st, e)
	}
	if len(reg.tokens) != 1 || reg.tokens[0] != "fcm-1" {
		t.Fatalf("tokens = %v", reg.tokens)
	}
	reg.err = push.ErrInvalidToken
	if st, e := w.post("/v1/devices", inboxapi.RegisterDeviceRequest{Token: ""}); st != http.StatusBadRequest || e.Code != inboxapi.CodeInvalid {
		t.Fatalf("empty token: %d %+v", st, e)
	}
}

// planWait adds gh:o/r#20 waiting on a plan posted at at.
func (w *world) planWait(at time.Time) forge.IssueRef {
	me := forge.User{Login: "me", ID: 1}
	ref := w.f.AddIssue("o", "r", 20, "Plan a greeting", "body", me)
	w.f.PostAs = "bot"
	w.f.PostComment(ctx, ref, "### Plan\n\n1. Say hi\n\n"+
		marker.Marker{Host: "laptop", Run: "p1", State: "waiting", Reason: "plan", At: at}.Format()+"\n")
	w.f.AddLabel(ref, "agent:waiting", w.bot)
	w.f.PostAs = "me"
	w.srv.Inbox.(*inbox.Inbox).Config.Plan.ApproveWord = "/ok"
	return ref
}

func lastComment(w *world, ref forge.IssueRef) string {
	cs := w.f.CommentsOf(ref)
	return cs[len(cs)-1].Body
}

func TestApprovePostsTheApproveWord(t *testing.T) {
	w := newServer(t)
	at := w.now.Add(-time.Hour)
	ref := w.planWait(at)
	if st, e := w.post("/v1/approve", inboxapi.ApproveRequest{Ref: "gh:o/r#20", Since: at}); st != http.StatusNoContent {
		t.Fatalf("approve: %d %+v", st, e)
	}
	if got := lastComment(w, ref); got != "/ok" {
		t.Fatalf("last comment = %q", got)
	}
}

func TestApproveRefusals(t *testing.T) {
	at := time.Date(2026, 10, 31, 11, 0, 0, 0, time.UTC)
	for name, c := range map[string]struct {
		ref   string
		since time.Time
	}{
		"a newer plan":        {"gh:o/r#20", at.Add(-time.Minute)},
		"waiting on question": {"gh:o/r#12", time.Date(2026, 10, 31, 11, 0, 0, 0, time.UTC)},
	} {
		t.Run(name, func(t *testing.T) {
			w := newServer(t)
			ref := w.planWait(at)
			before := len(w.f.CommentsOf(ref))
			if st, e := w.post("/v1/approve", inboxapi.ApproveRequest{Ref: c.ref, Since: c.since}); st != http.StatusConflict || e.Code != inboxapi.CodeStale {
				t.Fatalf("approve: %d %+v", st, e)
			}
			if n := len(w.f.CommentsOf(ref)); n != before {
				t.Fatal("a comment was posted")
			}
		})
	}
}

func TestApproveRefusesAnIssueNoLongerWaiting(t *testing.T) {
	w := newServer(t)
	at := w.now.Add(-time.Hour)
	ref := w.planWait(at)
	w.f.AddLabel(ref, "agent:running", w.bot)
	if st, e := w.post("/v1/approve", inboxapi.ApproveRequest{Ref: "gh:o/r#20", Since: at}); st != http.StatusConflict || e.Code != inboxapi.CodeStale {
		t.Fatalf("approve: %d %+v", st, e)
	}
}

type observer struct {
	mu   sync.Mutex
	seen []*inboxapi.Inbox
}

func (o *observer) Observe(_ context.Context, in *inboxapi.Inbox) {
	o.mu.Lock()
	o.seen = append(o.seen, in)
	o.mu.Unlock()
}

func (o *observer) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.seen)
}

func TestWatchReadsWithNoOneListening(t *testing.T) {
	w := newServer(t)
	w.srv.Interval = 20 * time.Millisecond
	obs := &observer{}
	w.srv.Observer = obs
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	w.srv.Watch(c)
	waitSearches(t, w, 6) // three listings of two forges
	deadline := time.Now().Add(5 * time.Second)
	for obs.count() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if obs.count() < 3 {
		t.Fatalf("the observer saw %d listings", obs.count())
	}
	// A page that comes and goes leaves the reads running.
	_, leave := page(t, w)
	leave()
	before := w.searches()
	waitSearches(t, w, before+2)
}

func TestRelayEventWhileWatchingReads(t *testing.T) {
	w := newServer(t)
	w.srv.Interval = time.Hour
	w.srv.debounce, w.srv.gap = 10*time.Millisecond, 10*time.Millisecond
	w.srv.Observer = &observer{}
	c, cancel := context.WithCancel(ctx)
	defer cancel()
	w.srv.Watch(c)
	waitSearches(t, w, 2)
	w.srv.RelayEvent()
	waitSearches(t, w, 4)
}

// A second press of the same notification's button (the app's listener and its background task) posts nothing more.
func TestApproveTwicePostsOnce(t *testing.T) {
	w := newServer(t)
	at := w.now.Add(-time.Hour)
	ref := w.planWait(at)
	for i := 0; i < 2; i++ {
		if st, e := w.post("/v1/approve", inboxapi.ApproveRequest{Ref: "gh:o/r#20", Since: at}); st != http.StatusNoContent {
			t.Fatalf("approve %d: %d %+v", i, st, e)
		}
	}
	n := 0
	for _, c := range w.f.CommentsOf(ref) {
		if c.Body == "/ok" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d approvals posted, want 1", n)
	}
}

// An approval of an earlier plan does not count for the plan posted after it.
func TestApproveOfAnEarlierPlanDoesNotCount(t *testing.T) {
	w := newServer(t)
	at := w.now.Add(-2 * time.Hour)
	ref := w.planWait(at)
	w.f.PostComment(ctx, ref, "/ok")
	later := w.now.Add(-time.Hour)
	w.f.PostAs = "bot"
	w.f.PostComment(ctx, ref, "### Plan\n\n1. Say hello\n\n"+
		marker.Marker{Host: "laptop", Run: "p2", State: "waiting", Reason: "plan", At: later}.Format()+"\n")
	w.f.PostAs = "me"
	if st, e := w.post("/v1/approve", inboxapi.ApproveRequest{Ref: "gh:o/r#20", Since: later}); st != http.StatusNoContent {
		t.Fatalf("approve: %d %+v", st, e)
	}
	if got := lastComment(w, ref); got != "/ok" {
		t.Fatalf("last comment = %q, want the new plan approved", got)
	}
}
