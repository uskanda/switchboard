package inboxserve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

// page listens as /v1/events does: it subscribes, and fetches a listing of its own when there is nothing recent.
// It leaves when the returned function is called, or when the test ends.
func page(t *testing.T, w *world) (<-chan *inboxapi.Inbox, func()) {
	t.Helper()
	ch := make(chan *inboxapi.Inbox, 1)
	if !w.srv.subscribe(ch) {
		offer(ch, w.srv.fetch(ctx))
	}
	left := false
	leave := func() {
		if !left {
			left = true
			w.srv.unsubscribe(ch)
		}
	}
	t.Cleanup(leave)
	return ch, leave
}

// get is GET /v1/inbox.
func get(t *testing.T, w *world) *inboxapi.Inbox {
	t.Helper()
	rec := httptest.NewRecorder()
	w.srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/inbox", nil))
	var out inboxapi.Inbox
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("GET /v1/inbox: %v: %s", err, rec.Body.String())
	}
	return &out
}

// waitSearches waits until the forges were searched at least n times, or fails.
func waitSearches(t *testing.T, w *world, n int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for w.searches() < n && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := w.searches(); got < n {
		t.Fatalf("searches = %d, want at least %d", got, n)
	}
}

// Events that arrive close together are read once, after the debounce.
func TestRelayEventsAreReadOnceAfterTheDebounce(t *testing.T) {
	w := newServer(t)
	w.srv.Interval = time.Hour
	w.srv.debounce, w.srv.gap = 80*time.Millisecond, time.Millisecond
	got, _ := page(t, w)
	next(t, got)
	before := w.searches()
	w.srv.RelayEvent()
	w.srv.RelayEvent()
	time.Sleep(20 * time.Millisecond)
	w.srv.RelayEvent()
	if n := w.searches(); n != before {
		t.Fatalf("the forges were read before the debounce ended: %d then %d", before, n)
	}
	waitSearches(t, w, before+1)
	time.Sleep(200 * time.Millisecond)
	// One listing searches each forge once.
	if n := w.searches(); n != before+2 {
		t.Fatalf("searches = %d after three events, want one listing (%d)", n, before+2)
	}
}

// An event right after a listing waits until the gap since that listing has passed.
func TestRelayEventWaitsForTheGapSinceTheLastListing(t *testing.T) {
	w := newServer(t)
	w.srv.Interval = time.Hour
	w.srv.debounce, w.srv.gap = time.Millisecond, time.Hour
	got, _ := page(t, w)
	next(t, got)
	before := w.searches()
	w.srv.RelayEvent()
	time.Sleep(150 * time.Millisecond)
	if n := w.searches(); n != before {
		t.Fatalf("an event read the forges within the gap: %d then %d", before, n)
	}
}

// With no page listening, an event reads nothing but drops the cached listing: the next GET reads the forges.
func TestRelayEventWithNoOneListeningDropsTheCache(t *testing.T) {
	w := newServer(t)
	w.srv.Interval = time.Hour
	got, leave := page(t, w)
	next(t, got)
	leave()
	before := w.searches()
	get(t, w)
	if n := w.searches(); n != before {
		t.Fatalf("GET before the event did not take the cached listing: %d then %d", before, n)
	}
	w.srv.RelayEvent()
	if n := w.searches(); n != before {
		t.Fatalf("an event with no one listening read the forges: %d then %d", before, n)
	}
	get(t, w)
	if n := w.searches(); n == before {
		t.Fatal("GET /v1/inbox after an event served the cached listing")
	}
}

// A page opening after an event reads the forges instead of taking the last listing.
func TestRelayEventWithNoOneListeningMakesTheNextPageRead(t *testing.T) {
	w := newServer(t)
	w.srv.Interval = time.Hour
	got, leave := page(t, w)
	next(t, got)
	leave()
	w.srv.RelayEvent()
	before := w.searches()
	got, _ = page(t, w)
	next(t, got)
	if n := w.searches(); n == before {
		t.Fatal("the page got the listing from before the event")
	}
}

// The listing carries the relay's state and the interval; a change of state is sent without reading the forges.
func TestRelayStateIsSentWithoutReading(t *testing.T) {
	w := newServer(t)
	w.srv.Interval = 45 * time.Second
	w.srv.Relay = true
	got, _ := page(t, w)
	first := next(t, got)
	if first.Relay != inboxapi.RelayDisconnected || first.RefreshIntervalSeconds != 45 {
		t.Fatalf("relay = %q, interval = %d", first.Relay, first.RefreshIntervalSeconds)
	}
	before := w.searches()
	w.srv.RelayState(true)
	if in := next(t, got); in.Relay != inboxapi.RelayConnected || len(in.Items) != len(first.Items) {
		t.Fatalf("after connecting: relay = %q, items = %d", in.Relay, len(in.Items))
	}
	w.srv.RelayState(true)
	quiet(t, got, 50*time.Millisecond)
	w.srv.RelayState(false)
	if in := next(t, got); in.Relay != inboxapi.RelayDisconnected {
		t.Fatalf("after losing the relay: relay = %q", in.Relay)
	}
	if n := w.searches(); n != before {
		t.Fatalf("a change of the relay's state read the forges: %d then %d", before, n)
	}
	w.srv.RelayState(true)
	l := get(t, w)
	if n := w.searches(); n != before {
		t.Fatalf("GET did not take the cached listing: %d then %d", before, n)
	}
	if l.Relay != inboxapi.RelayConnected || l.RefreshIntervalSeconds != 45 {
		t.Fatalf("GET: relay = %q, interval = %d", l.Relay, l.RefreshIntervalSeconds)
	}
}

// Without inbox.relay the listing says nothing of a relay.
func TestNoRelayConfigured(t *testing.T) {
	w := newServer(t)
	w.srv.Interval = time.Hour
	l := get(t, w)
	if l.Relay != "" || l.RefreshIntervalSeconds != 3600 {
		t.Fatalf("relay = %q, interval = %d", l.Relay, l.RefreshIntervalSeconds)
	}
}

func TestSameTellsRelayStatesApart(t *testing.T) {
	a := &inboxapi.Inbox{Items: []inboxapi.Item{}, Relay: inboxapi.RelayConnected}
	b := &inboxapi.Inbox{Items: []inboxapi.Item{}, Relay: inboxapi.RelayDisconnected}
	if same(a, b) {
		t.Fatal("listings with different relay states are the same")
	}
}
