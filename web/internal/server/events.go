package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

// DefaultPoll is how often an open page's list is asked of the inbox service when Server.Poll is zero. The inbox
// service caches the list, so this sets how soon a renewed list reaches the page, not how often the forges are read.
const DefaultPoll = 5 * time.Second

// keepalive is how often a stream with no change gets a comment, so that a proxy or the browser keeps it open.
const keepalive = 25 * time.Second

func (s *Server) poll() time.Duration {
	if s.Poll > 0 {
		return s.Poll
	}
	return DefaultPoll
}

// woken returns a channel closed at the next wake.
func (s *Server) woken() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wake == nil {
		s.wake = make(chan struct{})
	}
	return s.wake
}

// wakeAll makes every open stream ask for the list now, as after a reply.
func (s *Server) wakeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wake != nil {
		close(s.wake)
	}
	s.wake = make(chan struct{})
}

// snapshot is the event that tells the list as it is now, and a key that is equal for an equal list.
func (s *Server) snapshot(r *http.Request) (name string, data, key []byte) {
	in, err := s.Inbox.List(r.Context())
	if err != nil {
		e := &inboxapi.Error{}
		switch {
		case errors.As(err, &e):
		case errors.Is(err, inboxapi.ErrUnavailable):
			e = &inboxapi.Error{Code: CodeUnavailable, Message: "the inbox service (switchboard inbox serve) is not reachable"}
		default:
			e = &inboxapi.Error{Code: inboxapi.CodeForge, Message: err.Error()}
		}
		data, _ = json.Marshal(e)
		return "failure", data, append([]byte("failure"), data...)
	}
	data, _ = json.Marshal(in)
	// fetched_at changes each time the inbox service renews its cache; only the items and the errors are news.
	key, _ = json.Marshal(struct {
		Items  []inboxapi.Item       `json:"items"`
		Errors []inboxapi.ForgeError `json:"errors"`
	}{in.Items, in.Errors})
	return "inbox", data, key
}

// events streams the list to an open page as server-sent events: the list when the stream opens, then the list
// again each time it changes. An "inbox" event carries an inboxapi.Inbox, a "failure" event an inboxapi.Error.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	// The stream outlives the server's write timeout, which is there for the other requests.
	rc.SetWriteDeadline(time.Time{})
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if rc.Flush() != nil {
		return
	}
	tick := time.NewTicker(s.poll())
	defer tick.Stop()
	var last []byte
	quiet := time.Now()
	for {
		wake := s.woken()
		name, data, key := s.snapshot(r)
		switch {
		case !bytes.Equal(key, last):
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
			last, quiet = key, time.Now()
		case time.Since(quiet) >= keepalive:
			fmt.Fprint(w, ": keepalive\n\n")
			quiet = time.Now()
		}
		if rc.Flush() != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		case <-wake:
		}
	}
}
