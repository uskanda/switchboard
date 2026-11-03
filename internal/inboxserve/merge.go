package inboxserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
)

// Merger is what POST /v1/merge needs of the inbox: *inbox.Inbox merges the change request of a done issue.
type Merger interface {
	Merge(ctx context.Context, m forge.Merges, ref forge.IssueRef, sha string) error
}

// merge serves POST /v1/merge: the change request of a done issue is marked ready and merged with the inbox token,
// only at the commit the page showed (ADR-043). A merge the forge refused for a conflict is answered with 202 and made
// once the agent has fixed the conflict (ADR-047).
func (s *Server) merge(w http.ResponseWriter, r *http.Request) {
	var req inboxapi.MergeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not a merge request")
		return
	}
	b := s.inbox()
	ref, err := b.ParseRef(req.Ref)
	if err != nil {
		failWith(w, err)
		return
	}
	mb, ok := b.(Merger)
	if !ok {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "this inbox does not merge")
		return
	}
	err = mb.Merge(r.Context(), s.Merges[ref.Kind()], ref, req.SHA)
	var nm *forge.NotMergeableError
	switch {
	case err == nil:
	case errors.Is(err, inbox.ErrStale):
		fail(w, http.StatusConflict, inboxapi.CodeStale, err.Error())
		return
	case errors.Is(err, forge.ErrHeadMoved):
		fail(w, http.StatusConflict, inboxapi.CodeStale, "the change request has commits the page did not show; open it again")
		return
	case errors.As(err, &nm):
		if s.waitForFix(r.Context(), b, ref, req.SHA) {
			send(w, http.StatusAccepted, inboxapi.MergeAnswer{Code: inboxapi.CodeAutoMergePending,
				Message: "the change request conflicts with its base; it is merged once the agent has fixed the conflict"})
			return
		}
		fail(w, http.StatusConflict, inboxapi.CodeNotMergeable, nm.Reason)
		return
	case errors.Is(err, forge.ErrForbidden):
		fail(w, http.StatusForbidden, inboxapi.CodeForbidden,
			"the inbox token may not merge: give it Pull requests and Contents write on GitHub, or the api scope on GitLab")
		return
	default:
		failWith(w, err)
		return
	}
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	s.readAgain()
	w.WriteHeader(http.StatusNoContent)
}
