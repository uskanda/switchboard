package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

// event is one server-sent event.
type event struct{ name, data string }

// stream opens /api/events on a running server and returns its events, read in the background.
func stream(t *testing.T, url string) <-chan event {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", url+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("events: %d %v", resp.StatusCode, resp.Header)
	}
	out := make(chan event, 16)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(resp.Body)
		var e event
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				e.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				e.data = strings.TrimPrefix(line, "data: ")
			case line == "" && e.name != "":
				out <- e
				e = event{}
			}
		}
	}()
	return out
}

func next(t *testing.T, events <-chan event, within time.Duration) event {
	t.Helper()
	select {
	case e, ok := <-events:
		if !ok {
			t.Fatal("the stream ended")
		}
		return e
	case <-time.After(within):
		t.Fatalf("no event within %v", within)
	}
	return event{}
}

func refs(t *testing.T, e event) string {
	t.Helper()
	var in inboxapi.Inbox
	if e.name != "inbox" || json.Unmarshal([]byte(e.data), &in) != nil {
		t.Fatalf("not an inbox event: %+v", e)
	}
	var r []string
	for _, it := range in.Items {
		r = append(r, it.Ref)
	}
	return strings.Join(r, ",")
}

func started(t *testing.T, s *Server, writeTimeout time.Duration) *httptest.Server {
	ts := httptest.NewUnstartedServer(s)
	ts.Config.WriteTimeout = writeTimeout
	ts.Config.ReadTimeout = writeTimeout
	ts.Start()
	t.Cleanup(ts.Close)
	return ts
}

func TestEventsSendTheListAndEachChange(t *testing.T) {
	f := &fakeInbox{}
	s := newServer(f)
	s.Poll = 10 * time.Millisecond
	events := stream(t, started(t, s, 0).URL)

	if got := refs(t, next(t, events, time.Second)); got != "gh:o/r#12" {
		t.Fatalf("first = %s", got)
	}
	f.set([]inboxapi.Item{{Ref: "gh:o/r#12"}, {Ref: "gl:nas/sb#6"}}, nil)
	if got := refs(t, next(t, events, time.Second)); got != "gh:o/r#12,gl:nas/sb#6" {
		t.Fatalf("after a change = %s", got)
	}
	// A list that was only fetched again is not sent again.
	select {
	case e := <-events:
		t.Fatalf("sent without a change: %+v", e)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEventsTellAnUnreachableInboxService(t *testing.T) {
	f := &fakeInbox{err: inboxapi.ErrUnavailable}
	s := newServer(f)
	s.Poll = 10 * time.Millisecond
	events := stream(t, started(t, s, 0).URL)

	e := next(t, events, time.Second)
	var got inboxapi.Error
	if e.name != "failure" || json.Unmarshal([]byte(e.data), &got) != nil || got.Code != CodeUnavailable {
		t.Fatalf("event = %+v", e)
	}
	f.set(nil, nil)
	if got := refs(t, next(t, events, time.Second)); got != "gh:o/r#12" {
		t.Fatalf("after recovering = %s", got)
	}
}

func TestAReplyIsFollowedAtOnce(t *testing.T) {
	f := &fakeInbox{}
	s := newServer(f)
	s.Poll = time.Hour
	ts := started(t, s, 0)
	events := stream(t, ts.URL)
	next(t, events, time.Second)

	f.set([]inboxapi.Item{}, nil)
	req, _ := http.NewRequest("POST", ts.URL+"/api/reply", strings.NewReader(`{"ref":"gh:o/r#12","text":"B"}`))
	req.Host = listen
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://"+listen)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reply: %v %v", resp, err)
	}
	if got := refs(t, next(t, events, time.Second)); got != "" {
		t.Fatalf("after the reply = %q", got)
	}
}

func TestEventsOutliveTheTimeouts(t *testing.T) {
	f := &fakeInbox{}
	s := newServer(f)
	s.Poll = 10 * time.Millisecond
	events := stream(t, started(t, s, 50*time.Millisecond).URL)
	next(t, events, time.Second)

	time.Sleep(150 * time.Millisecond)
	f.set([]inboxapi.Item{}, nil)
	if got := refs(t, next(t, events, time.Second)); got != "" {
		t.Fatalf("after the write timeout = %q", got)
	}
}
