package relayclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/relay"
)

// server sends one event per connection, numbered on from the client's Last-Event-ID, then drops the connection.
type server struct {
	mu      sync.Mutex
	lastIDs []string
	auth    []string
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.lastIDs = append(s.lastIDs, r.Header.Get("Last-Event-ID"))
	s.auth = append(s.auth, r.Header.Get("Authorization"))
	n := len(s.lastIDs)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprintf(w, ": ping\n\nid: %d\nevent: issue\ndata: {\"forge\":\"gl\",\"project\":\"nas/x\",\"issue\":%d,\"kind\":\"note\",\"delivery\":\"d%d\"}\n\n", 10+n, n, n)
	w.(http.Flusher).Flush()
}

func TestReconnectsWithLastEventID(t *testing.T) {
	s := &server{}
	srv := httptest.NewServer(s)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var got []relay.Event
	var states []bool
	c := &Client{URL: srv.URL + "/events", Token: "tok", MinBackoff: time.Millisecond, MaxBackoff: 5 * time.Millisecond, Logf: func(string, ...any) {}}
	go c.Run(ctx, func(e relay.Event) {
		mu.Lock()
		got = append(got, e)
		if len(got) == 3 {
			cancel()
		}
		mu.Unlock()
	}, func(up bool) { mu.Lock(); states = append(states, up); mu.Unlock() })
	<-ctx.Done()
	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(got) < 3 || got[0].Issue != 1 || got[2].Delivery != "d3" {
		t.Fatalf("events = %+v", got)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastIDs[0] != "" || s.lastIDs[1] != "11" || s.lastIDs[2] != "12" {
		t.Fatalf("Last-Event-ID sent: %q", s.lastIDs)
	}
	if s.auth[0] != "Bearer tok" {
		t.Fatalf("Authorization %q", s.auth[0])
	}
	if len(states) < 2 || !states[0] || states[1] {
		t.Fatalf("states = %v", states)
	}
}

func TestRefusedTokenIsReportedAndRetried(t *testing.T) {
	var mu sync.Mutex
	tries := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		tries++
		mu.Unlock()
		http.Error(w, "unauthorized", 401)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	var logs []string
	var lmu sync.Mutex
	c := &Client{URL: srv.URL, Token: "bad", MinBackoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond,
		Logf: func(f string, a ...any) { lmu.Lock(); logs = append(logs, fmt.Sprintf(f, a...)); lmu.Unlock() }}
	ups := 0
	c.Run(ctx, func(relay.Event) {}, func(up bool) {
		if up {
			ups++
		}
	})
	mu.Lock()
	defer mu.Unlock()
	if tries < 2 || ups != 0 {
		t.Fatalf("tries %d, connected %d times", tries, ups)
	}
	lmu.Lock()
	defer lmu.Unlock()
	if len(logs) == 0 || !contains(logs[0], "401") {
		t.Fatalf("logs %v", logs)
	}
}

func TestBackoffDoublesUpToTheMaximum(t *testing.T) {
	c := &Client{MinBackoff: time.Second, MaxBackoff: 60 * time.Second}
	d := c.MinBackoff
	var seen []time.Duration
	for i := 0; i < 8; i++ {
		seen = append(seen, d)
		d = c.grow(d)
	}
	if seen[0] != time.Second || seen[1] != 2*time.Second || seen[7] != 60*time.Second {
		t.Fatalf("backoff %v", seen)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
