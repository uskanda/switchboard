package relay

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func hashOf(token string) string {
	s := sha256.Sum256([]byte(token))
	return "sha256:" + hex.EncodeToString(s[:])
}

func hub() *Hub {
	return &Hub{Hosts: map[string]string{"ubuntu-a": hashOf("tok-a"), "ubuntu-b": hashOf("tok-b")}, Ping: 50 * time.Millisecond, Logf: func(string, ...any) {}}
}

// subscribe connects with token and lastID and returns a reader of the stream's lines.
func subscribe(t *testing.T, ctx context.Context, url, token, lastID string) (*http.Response, *bufio.Scanner) {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, "GET", url+"/events", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp, bufio.NewScanner(resp.Body)
}

// next returns the next event's id and data, skipping pings.
func next(t *testing.T, sc *bufio.Scanner) (id, data string) {
	t.Helper()
	for sc.Scan() {
		l := sc.Text()
		switch {
		case strings.HasPrefix(l, "id: "):
			id = l[4:]
		case strings.HasPrefix(l, "data: "):
			data = l[6:]
		case l == "" && data != "":
			return id, data
		}
	}
	t.Fatalf("the stream ended: %v", sc.Err())
	return
}

func TestSSENeedsAKnownToken(t *testing.T) {
	srv := httptest.NewServer(hub())
	defer srv.Close()
	for _, tok := range []string{"", "wrong"} {
		resp, _ := subscribe(t, context.Background(), srv.URL, tok, "")
		resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("token %q: status %d", tok, resp.StatusCode)
		}
	}
	resp, _ := subscribe(t, context.Background(), srv.URL, "tok-a", "")
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d, type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}

func TestSSEBroadcastsToEveryHost(t *testing.T) {
	h := hub()
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, a := subscribe(t, ctx, srv.URL, "tok-a", "")
	_, b := subscribe(t, ctx, srv.URL, "tok-b", "")
	waitSubscribers(t, h, 2)
	h.Publish(Event{Forge: "gl", Project: "nas/x", Issue: 4, Kind: "note", Delivery: "d"})
	for _, sc := range []*bufio.Scanner{a, b} {
		id, data := next(t, sc)
		if id != "1" || data != `{"forge":"gl","project":"nas/x","issue":4,"kind":"note","delivery":"d"}` {
			t.Fatalf("id %s data %s", id, data)
		}
	}
}

func TestSSEReplaysAfterLastEventID(t *testing.T) {
	h := hub()
	for i := 1; i <= 3; i++ {
		h.Publish(Event{Forge: "gh", Project: "a/b", Issue: i, Kind: "issues"})
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, sc := subscribe(t, ctx, srv.URL, "tok-a", "1")
	for _, want := range []string{"2", "3"} {
		if id, _ := next(t, sc); id != want {
			t.Fatalf("id %s, want %s", id, want)
		}
	}
}

// A Last-Event-ID from before the relay restarted is newer than anything it holds: it gets the whole buffer.
func TestSSEReplaysAllForAnIDFromBeforeARestart(t *testing.T) {
	h := hub()
	h.Publish(Event{Forge: "gh", Project: "a/b", Issue: 1, Kind: "issues"})
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, sc := subscribe(t, ctx, srv.URL, "tok-a", "900")
	if id, _ := next(t, sc); id != "1" {
		t.Fatalf("id %s", id)
	}
}

func TestSSEBufferHoldsTheLast1000(t *testing.T) {
	h := hub()
	for i := 1; i <= 1005; i++ {
		h.Publish(Event{Forge: "gh", Project: "a/b", Issue: i, Kind: "issues"})
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, sc := subscribe(t, ctx, srv.URL, "tok-a", "0")
	if id, _ := next(t, sc); id != "6" {
		t.Fatalf("the oldest replayed is %s", id)
	}
}

func TestSSEPings(t *testing.T) {
	srv := httptest.NewServer(hub())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, sc := subscribe(t, ctx, srv.URL, "tok-a", "")
	for sc.Scan() {
		if sc.Text() == ": ping" {
			return
		}
	}
	t.Fatal("no ping")
}

func waitSubscribers(t *testing.T, h *Hub, n int) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if h.subscribers() == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("subscribers = %d", h.subscribers())
}
