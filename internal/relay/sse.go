package relay

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	bufferSize = 1000 // events kept for Last-Event-ID
	queueSize  = 256  // events a subscriber may fall behind before it is dropped
)

// Hub sends every published event to every subscribed host over SSE at GET /events (design.md 4). A host proves
// itself with a bearer token whose SHA-256 is in Hosts (name -> "sha256:<hex>"); the hub never holds a token.
type Hub struct {
	Hosts map[string]string
	Ping  time.Duration // 30s when zero
	Logf  func(format string, args ...any)
	// Now is when the hub starts numbering; time.Now when nil.
	Now func() time.Time

	mu      sync.Mutex
	started bool
	last    int64
	buf     []numbered
	subs    map[chan numbered]bool
}

// begin sets the ID before the first event to the time of the hub's first use, in microseconds, so that the IDs of
// a relay that restarted are larger than every ID it gave before. Call it with mu held.
func (h *Hub) begin() {
	if h.started {
		return
	}
	h.started = true
	now := time.Now
	if h.Now != nil {
		now = h.Now
	}
	h.last = now().UnixMicro()
}

type numbered struct {
	id   int64
	data []byte
}

// Publish numbers the event, keeps it for replay, and queues it for every subscriber.
func (h *Hub) Publish(e Event) {
	data, _ := json.Marshal(e)
	h.mu.Lock()
	defer h.mu.Unlock()
	h.begin()
	h.last++
	n := numbered{h.last, data}
	h.buf = append(h.buf, n)
	if len(h.buf) > bufferSize {
		h.buf = h.buf[len(h.buf)-bufferSize:]
	}
	for ch := range h.subs {
		select {
		case ch <- n:
		default: // too far behind: close it; the agent reconnects and replays
			delete(h.subs, ch)
			close(ch)
		}
	}
}

func (h *Hub) subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// host returns the name of the host whose token is presented, or "".
func (h *Hub) host(r *http.Request) string {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || tok == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(tok))
	got := "sha256:" + hex.EncodeToString(sum[:])
	name := ""
	for n, want := range h.Hosts {
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 {
			name = n
		}
	}
	return name
}

// subscribe registers a subscriber and returns the events it missed after lastID. An ID from before a restart is
// smaller than every ID since, so the whole buffer is replayed; so is an ID newer than any held (a clock that went
// back).
func (h *Hub) subscribe(lastID int64) (chan numbered, []numbered) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.begin()
	if lastID > h.last {
		lastID = 0
	}
	var missed []numbered
	for _, n := range h.buf {
		if n.id > lastID {
			missed = append(missed, n)
		}
	}
	ch := make(chan numbered, queueSize)
	if h.subs == nil {
		h.subs = map[chan numbered]bool{}
	}
	h.subs[ch] = true
	return ch, missed
}

func (h *Hub) unsubscribe(ch chan numbered) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs[ch] {
		delete(h.subs, ch)
		close(ch)
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/events" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := h.host(r)
	if name == "" {
		h.Logf("sse: refused a subscriber without a known token from %s", r.RemoteAddr)
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	lastID, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	ch, missed := h.subscribe(lastID)
	defer h.unsubscribe(ch)
	h.Logf("sse: %s subscribed (after event %d, %d to replay)", name, lastID, len(missed))
	defer h.Logf("sse: %s left", name)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	write := func(n numbered) error {
		_, err := fmt.Fprintf(w, "id: %d\nevent: issue\ndata: %s\n\n", n.id, n.data)
		return err
	}
	for _, n := range missed {
		if write(n) != nil {
			return
		}
	}
	flusher.Flush()
	ping := h.Ping
	if ping == 0 {
		ping = 30 * time.Second
	}
	t := time.NewTicker(ping)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case n, open := <-ch:
			if !open || write(n) != nil {
				return
			}
		case <-t.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
		}
		flusher.Flush()
	}
}
