package inboxserve

import (
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
)

// RelayDebounce is how long the service waits after a relay event before reading the forges: the events that arrive
// meanwhile (a label change and a comment, say) are read once.
const RelayDebounce = 2 * time.Second

// RelayState records whether the relay is connected. A change is sent to the listeners at once with the last
// listing, without reading the forges.
func (s *Server) RelayState(connected bool) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if s.relayUp == connected {
		return
	}
	s.relayUp = connected
	if s.last == nil {
		return
	}
	next := *s.last
	next.Relay = s.relayLocked()
	s.last = &next
	for ch := range s.subs {
		offer(ch, &next)
	}
}

// RelayEvent takes an event of the relay. While a page listens, the forges are read RelayDebounce later, and no
// sooner than the gap (config.MinRefreshInterval) after the last listing; the events arriving meanwhile join that
// read. With no page listening, the cached listing is dropped so that the next page reads the forges.
func (s *Server) RelayEvent() {
	s.subMu.Lock()
	if len(s.subs) == 0 {
		s.subMu.Unlock()
		s.drop()
		return
	}
	defer s.subMu.Unlock()
	if s.pending != nil {
		return
	}
	wait := or(s.debounce, RelayDebounce)
	if s.last != nil {
		if g := or(s.gap, config.MinRefreshInterval) - s.now().Sub(s.last.FetchedAt); g > wait {
			wait = g
		}
	}
	s.pending = time.AfterFunc(wait, func() {
		s.subMu.Lock()
		s.pending = nil
		n := len(s.subs)
		s.subMu.Unlock()
		if n == 0 {
			// The last page left while waiting: the change is read when the next one opens.
			s.drop()
			return
		}
		s.readAgain()
	})
}

// drop forgets the cached and the last listing.
func (s *Server) drop() {
	s.subMu.Lock()
	s.last = nil
	s.subMu.Unlock()
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

// relayState is the relay's state for a listing.
func (s *Server) relayState() string {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	return s.relayLocked()
}

// relayLocked is relayState with s.subMu held.
func (s *Server) relayLocked() string {
	switch {
	case !s.Relay:
		return ""
	case s.relayUp:
		return inboxapi.RelayConnected
	default:
		return inboxapi.RelayDisconnected
	}
}

// listeners counts the listeners of /v1/events.
func (s *Server) listeners() int {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	return len(s.subs)
}

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}
