package inboxserve

import (
	"context"
	"errors"
	"net/http"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
)

// MergeStatusReader is what GET /v1/merge-status needs of the inbox: *inbox.Inbox reads where the change request of a
// done issue stands.
type MergeStatusReader interface {
	MergeStatus(ctx context.Context, c forge.CI, ref forge.IssueRef, sha string) (*inbox.MergeStatus, error)
}

// mergeStatus serves GET /v1/merge-status?ref=…&sha=…: the state, the conflict and the CI of the change request of a
// done issue, read with the inbox token for the merge dialog (ADR-050).
func (s *Server) mergeStatus(w http.ResponseWriter, r *http.Request) {
	b := s.inbox()
	ref, err := b.ParseRef(r.URL.Query().Get("ref"))
	if err != nil {
		failWith(w, err)
		return
	}
	if !s.sees(r.Context(), ref.String()) {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, ref.String()+" is not one you may see")
		return
	}
	mb, ok := b.(MergeStatusReader)
	if !ok {
		fail(w, http.StatusNotFound, inboxapi.CodeNotFound, "this inbox does not read change requests")
		return
	}
	st, err := mb.MergeStatus(r.Context(), s.CI[ref.Kind()], ref, r.URL.Query().Get("sha"))
	switch {
	case err == nil:
	case errors.Is(err, inbox.ErrStale):
		fail(w, http.StatusConflict, inboxapi.CodeStale, err.Error())
		return
	default:
		failWith(w, err)
		return
	}
	out := inboxapi.MergeStatus{State: st.State, HeadMoved: st.HeadMoved, Mergeable: st.Mergeable, CI: st.CI,
		Checks: make([]inboxapi.CheckStatus, 0, len(st.Checks))}
	for _, c := range st.Checks {
		out.Checks = append(out.Checks, inboxapi.CheckStatus{Name: c.Name, Status: c.Status, URL: c.URL})
	}
	send(w, http.StatusOK, out)
}
