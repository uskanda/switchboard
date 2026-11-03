package inboxserve

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/automerge"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
)

// AutoMerger is what the merges the inbox service waits to make need of the inbox: *inbox.Inbox decides whether a
// refused merge waits for a fix, and looks at a waiting one (ADR-048).
type AutoMerger interface {
	AutoMergeCandidate(ctx context.Context, m forge.Merges, ref forge.IssueRef, sha string) (*automerge.Request, error)
	AutoMergeStep(ctx context.Context, m forge.Merges, r automerge.Request) (inbox.AutoMergeStep, error)
}

// waitForFix keeps the merge of ref at sha, which the forge refused, when the inbox says it waits for the agent to
// fix a conflict, and reports whether it does. Without a store, or when the inbox cannot tell, it does not.
func (s *Server) waitForFix(ctx context.Context, b Inbox, ref forge.IssueRef, sha string) bool {
	am, ok := b.(AutoMerger)
	if s.AutoMerge == nil || !ok {
		return false
	}
	r, err := am.AutoMergeCandidate(ctx, s.Merges[ref.Kind()], ref, sha)
	if err != nil || r == nil {
		return false
	}
	if err := s.AutoMerge.Put(*r); err != nil {
		s.logf("automerge: keep %s: %v", r.Ref, err)
		return false
	}
	s.audit("automerge.requested", *r, "")
	s.listingChanged()
	s.kickAutoMerge()
	return true
}

// cancelAutoMerge serves POST /v1/automerge/cancel: the merge waiting on the issue is cancelled by the person.
func (s *Server) cancelAutoMerge(w http.ResponseWriter, r *http.Request) {
	var req inboxapi.CancelAutoMergeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not a cancel of an automatic merge")
		return
	}
	ref, err := s.inbox().ParseRef(req.Ref)
	if err != nil {
		failWith(w, err)
		return
	}
	pending, waits := s.AutoMerge.Get(ref.String())
	ok, err := s.AutoMerge.Cancel(ref.String())
	switch {
	case err != nil:
		fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
		return
	case !ok || !waits:
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, ref.String()+" has no merge waiting")
		return
	}
	s.audit("automerge.cancelled", pending, automerge.ReasonUser)
	s.listingChanged()
	w.WriteHeader(http.StatusNoContent)
}

// RunAutoMerge looks at the merges waiting for a fix at once, then every interval and whenever kicked (a merge taken
// on, a relay event), until ctx ends. It runs whether or not a page listens. Without a store it returns at once.
func (s *Server) RunAutoMerge(ctx context.Context) {
	if s.AutoMerge == nil {
		return
	}
	t := time.NewTicker(s.interval())
	defer t.Stop()
	kick := s.autoMergeKick()
	for {
		s.autoMergeOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-kick:
		}
	}
}

// autoMergeOnce looks at every waiting merge once, and ends those the inbox decided on.
func (s *Server) autoMergeOnce(ctx context.Context) {
	if s.AutoMerge == nil {
		return
	}
	changed := false
	for _, r := range s.AutoMerge.Pending() {
		if ctx.Err() != nil {
			return
		}
		am, ok := s.inbox().(AutoMerger)
		if !ok {
			return
		}
		kind, _, _ := strings.Cut(r.Ref, ":")
		c, cancel := context.WithTimeout(ctx, FetchTimeout)
		st, err := am.AutoMergeStep(c, s.Merges[kind], r)
		cancel()
		if err != nil {
			s.logf("automerge: %s: %v", r.Ref, err)
			continue
		}
		if st.Outcome == "" {
			continue
		}
		if err := s.AutoMerge.Finish(r, st.Outcome, st.Reason); err != nil {
			s.logf("automerge: finish %s: %v", r.Ref, err)
		}
		if st.Outcome == automerge.Merged {
			r.SHA = st.SHA
		}
		s.audit("automerge."+st.Outcome, r, st.Reason)
		changed = true
	}
	if changed {
		s.listingChanged()
	}
}

// autoMergeKick is the channel that wakes RunAutoMerge.
func (s *Server) autoMergeKick() chan struct{} {
	s.kickOnce.Do(func() { s.kick = make(chan struct{}, 1) })
	return s.kick
}

// kickAutoMerge asks RunAutoMerge to look at the waiting merges without waiting for the interval.
func (s *Server) kickAutoMerge() {
	if s.AutoMerge == nil {
		return
	}
	select {
	case s.autoMergeKick() <- struct{}{}:
	default:
	}
}

// listingChanged drops the cached listing and asks the loop, when it runs, to read the forges again.
func (s *Server) listingChanged() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	s.readAgain()
}

// audit records what happened to a waiting merge: the issue, the change request, the commit and the reason only.
func (s *Server) audit(event string, r automerge.Request, reason string) {
	if s.Audit == nil {
		return
	}
	fields := map[string]any{"issue": r.Ref, "cr": r.CR, "sha": r.SHA}
	if reason != "" {
		fields["reason"] = reason
	}
	s.Audit.Log(event, fields)
}

func (s *Server) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// withAutoMerge adds the waiting merges and the finished ones to a listing.
func (s *Server) withAutoMerge(out *inboxapi.Inbox) {
	if s.AutoMerge == nil {
		return
	}
	for i := range out.Items {
		out.Items[i].AutoMerge = s.autoMergeOf(out.Items[i].Ref)
	}
	for _, r := range s.AutoMerge.Results() {
		out.AutoMergeResults = append(out.AutoMergeResults, inboxapi.AutoMergeResult{Ref: r.Ref, Number: r.CR, Outcome: r.Outcome, Reason: r.Reason, At: r.At})
	}
}

// autoMergeOf is the merge waiting on the issue ref, nil when there is none.
func (s *Server) autoMergeOf(ref string) *inboxapi.AutoMerge {
	if s.AutoMerge == nil {
		return nil
	}
	r, ok := s.AutoMerge.Get(ref)
	if !ok {
		return nil
	}
	return &inboxapi.AutoMerge{SHA: r.SHA, Since: r.Since}
}
