package server

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

// streamInbox is a fakeInbox whose event stream sends each listing after delay, then waits for the caller to leave.
type streamInbox struct {
	fakeInbox
	listings []*inboxapi.Inbox
	delay    time.Duration
	refuse   error
}

func (f *streamInbox) Events(ctx context.Context, fn func(*inboxapi.Inbox)) error {
	if f.refuse != nil {
		return f.refuse
	}
	for _, in := range f.listings {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
		fn(in)
	}
	<-ctx.Done()
	return ctx.Err()
}

// events opens /api/events on a server whose WriteTimeout is short, and returns the stream's lines as they come.
func events(t *testing.T, f *streamInbox) <-chan string {
	t.Helper()
	ts := httptest.NewUnstartedServer(&Server{Inbox: f, Listen: listen, Assets: assets})
	ts.Config.WriteTimeout = 100 * time.Millisecond
	ts.Start()
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d, content type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	return lines
}

// until reads lines until one starts with prefix, failing after a while or when the stream ends.
func until(t *testing.T, lines <-chan string, prefix string) string {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				t.Fatalf("the stream ended before %q", prefix)
			}
			if strings.HasPrefix(l, prefix) {
				return l
			}
		case <-deadline:
			t.Fatalf("no line %q", prefix)
		}
	}
}

func TestEventsPassTheListingsOnPastTheWriteTimeout(t *testing.T) {
	f := &streamInbox{delay: 150 * time.Millisecond, listings: []*inboxapi.Inbox{
		{Items: []inboxapi.Item{{Ref: "gh:o/r#1"}}},
		{Items: []inboxapi.Item{{Ref: "gh:o/r#1"}, {Ref: "gh:o/r#2"}}},
	}}
	lines := events(t, f)
	until(t, lines, "retry: ")
	until(t, lines, "event: inbox")
	if d := until(t, lines, "data: "); !strings.Contains(d, "gh:o/r#1") {
		t.Fatalf("first data = %s", d)
	}
	// The second listing comes 300ms after the stream opened, past the server's 100ms WriteTimeout.
	until(t, lines, "event: inbox")
	if d := until(t, lines, "data: "); !strings.Contains(d, "gh:o/r#2") {
		t.Fatalf("second data = %s", d)
	}
}

func TestEventsSayUnavailableWhenTheServiceIsDown(t *testing.T) {
	lines := events(t, &streamInbox{refuse: inboxapi.ErrUnavailable})
	until(t, lines, "retry: ")
	until(t, lines, "event: unavailable")
	for range lines {
	}
}

func TestEventsTakeGetOnly(t *testing.T) {
	if w := do(&Server{Inbox: &streamInbox{}, Listen: listen, Assets: assets}, http.MethodPost, "/api/events", "", good); w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/events = %d", w.Code)
	}
}
