package inboxserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/push"
)

// Registry keeps the apps' FCM tokens; *push.Devices is one.
type Registry interface {
	Register(token string, now time.Time) error
}

// Observer takes each listing read from the forges, in order; *push.Notifier is one.
type Observer interface {
	Observe(ctx context.Context, in *inboxapi.Inbox)
}

// devices serves POST /v1/devices: an app registers its FCM token.
func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	var req inboxapi.RegisterDeviceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not a device registration")
		return
	}
	if err := s.Devices.Register(req.Token, s.now()); err != nil {
		if errors.Is(err, push.ErrInvalidToken) {
			fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, err.Error())
			return
		}
		fail(w, http.StatusInternalServerError, inboxapi.CodeInvalid, "the device could not be registered")
		return
	}
	s.act(r.Context(), "device.register", "")
	w.WriteHeader(http.StatusNoContent)
}

// approve serves POST /v1/approve: the approve word is posted on an issue still waiting on the plan the
// notification reported, the wait that started at the time it carries. Anything else is stale.
func (s *Server) approve(w http.ResponseWriter, r *http.Request) {
	var req inboxapi.ApproveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not an approval")
		return
	}
	ref, err := s.inbox().ParseRef(req.Ref)
	if err != nil {
		failWith(w, err)
		return
	}
	if !s.sees(r.Context(), ref.String()) {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, ref.String()+" is not one you may see")
		return
	}
	d, err := s.inbox().Detail(r.Context(), ref)
	if err != nil {
		failWith(w, err)
		return
	}
	if !d.Waiting || d.State != marker.Waiting || d.Reason != "plan" || d.Since == nil || !d.Since.Equal(req.Since) || d.ApproveWord == "" {
		fail(w, http.StatusConflict, inboxapi.CodeStale, "the issue no longer waits on the plan the notification reported")
		return
	}
	if approvedAlready(d) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.replyAs(r.Context(), ref, d.ApproveWord, false); err != nil {
		if !failWrite(w, writeErr(err)) {
			failWith(w, err)
		}
		return
	}
	s.act(r.Context(), "approve", ref.String())
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	s.readAgain()
	w.WriteHeader(http.StatusNoContent)
}

// Watch keeps the forges read every interval, and soon after each relay event, until ctx ends, whether or not a page
// listens: the Observer gets every listing. It reads once at the start.
func (s *Server) Watch(ctx context.Context) {
	s.subMu.Lock()
	s.watching = true
	if s.stop == nil {
		loopCtx, cancel := context.WithCancel(context.Background())
		s.stop, s.refresh = cancel, make(chan struct{}, 1)
		go s.loop(loopCtx, s.refresh)
	}
	s.subMu.Unlock()
	go func() {
		<-ctx.Done()
		s.subMu.Lock()
		s.watching = false
		if len(s.subs) == 0 && s.stop != nil {
			s.stop()
			s.stop, s.refresh = nil, nil
		}
		s.subMu.Unlock()
	}()
	s.readAgain()
}

// observe hands in to the Observer, one listing at a time and in order; a listing the Observer has not taken yet is
// replaced by a newer one.
func (s *Server) observe(in *inboxapi.Inbox) {
	if s.Observer == nil {
		return
	}
	s.obsOnce.Do(func() {
		s.obs = make(chan *inboxapi.Inbox, 1)
		go func() {
			for in := range s.obs {
				s.Observer.Observe(context.Background(), in)
			}
		}()
	})
	s.obsMu.Lock()
	offer(s.obs, in)
	s.obsMu.Unlock()
}

// approvedAlready tells whether the approve word already follows the agent's latest wait post: the same
// notification's button pressed twice (the app's listener and its background task) approves once.
func approvedAlready(d *inbox.Detail) bool {
	approved := false
	for _, c := range d.Comments {
		switch {
		case c.Agent && c.Kind == marker.Waiting:
			approved = false
		case !c.Agent && strings.TrimSpace(c.Body) == d.ApproveWord:
			approved = true
		}
	}
	return approved
}
