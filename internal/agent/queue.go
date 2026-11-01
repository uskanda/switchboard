package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/trust"
)

// queuedFile keeps the queued comments this host posted, so that an agent started again removes the ones its
// previous process left.
const queuedFile = "queued.json"

// queuedNote is one queued comment in queuedFile.
type queuedNote struct {
	Issue   forge.IssueRef `json:"issue"`
	Comment int64          `json:"comment"`
}

// noteQueued posts the queued comment of ref, which has just been queued on the limit named by limitKind, and logs
// it. The caller has put a placeholder in a.queued, so that a second queueing of the issue posts nothing.
func (a *Agent) noteQueued(ctx context.Context, ref forge.IssueRef, limitKind string, value int) {
	a.logf("%s: queued: %s (%d) is reached", ref, limitName(limitKind), value)
	text := fmt.Sprintf(a.msg.queuedHost, a.Config.Host.Name, value)
	if limitKind == marker.QueuedProject {
		text = fmt.Sprintf(a.msg.queuedProject, value)
	}
	m := marker.Marker{Host: a.Config.Host.Name, State: marker.Queued, Reason: limitKind, At: time.Now()}
	c, err := a.forgeOf(ref).PostComment(ctx, ref, text+"\n\n"+m.Format()+"\n")
	if err != nil {
		a.mu.Lock()
		delete(a.queued, ref) // posted again on the next queueing
		a.mu.Unlock()
		a.logf("%s: post the queued comment: %v", ref, err)
		return
	}
	a.mu.Lock()
	_, still := a.queued[ref]
	if still {
		a.queued[ref] = c.ID
		a.saveQueued()
	}
	a.mu.Unlock()
	if !still { // the issue left the queue while the comment was posted
		a.removeQueued(ctx, ref, c.ID)
	}
}

func limitName(kind string) string {
	if kind == marker.QueuedProject {
		return "the project's max_parallel"
	}
	return "agent.max_parallel"
}

// unqueue forgets that ref is queued and removes its queued comment. It reports whether the issue was queued.
func (a *Agent) unqueue(ctx context.Context, ref forge.IssueRef) bool {
	a.mu.Lock()
	id, ok := a.queued[ref]
	delete(a.queued, ref)
	if ok {
		a.saveQueued()
	}
	a.mu.Unlock()
	if ok && id != 0 {
		a.removeQueued(ctx, ref, id)
	}
	return ok
}

// leaveQueue takes ref out of the queue without running it: its start label went, or the issue was closed.
func (a *Agent) leaveQueue(ctx context.Context, ref forge.IssueRef) {
	a.mu.Lock()
	delete(a.pending, ref)
	a.mu.Unlock()
	if a.unqueue(ctx, ref) {
		a.logf("%s: left the queue without running", ref)
	}
}

func (a *Agent) removeQueued(ctx context.Context, ref forge.IssueRef, id int64) {
	if err := a.forgeOf(ref).DeleteComment(ctx, ref, id); err != nil && !errors.Is(err, forge.ErrNotFound) {
		a.logf("%s: remove the queued comment %d: %v", ref, id, err)
	}
}

// saveQueued writes the posted queued comments to queuedFile. Call it with mu held.
func (a *Agent) saveQueued() {
	notes := []queuedNote{}
	for ref, id := range a.queued {
		if id != 0 {
			notes = append(notes, queuedNote{Issue: ref, Comment: id})
		}
	}
	if err := a.State.WriteJSON(queuedFile, notes); err != nil {
		a.logf("save the queued comments: %v", err)
	}
}

// clearQueued removes the queued comments a previous process of this host left. Issues still queued are queued
// again, with a new comment, by the next poll.
func (a *Agent) clearQueued(ctx context.Context) {
	var notes []queuedNote
	if err := a.State.ReadJSON(queuedFile, &notes); err != nil {
		return
	}
	for _, n := range notes {
		a.removeQueued(ctx, n.Issue, n.Comment)
	}
	if err := a.State.WriteJSON(queuedFile, []queuedNote{}); err != nil {
		a.logf("save the queued comments: %v", err)
	}
}

// dropQueued leaves the agent's queued comments out of what the run reads: they are removed before the claim.
func (pl *plan) dropQueued() {
	var comments []forge.Comment
	var kinds []trust.Kind
	for i, c := range pl.comments {
		if m := pl.agentMarker(i); m != nil && m.State == marker.Queued {
			continue
		}
		comments = append(comments, c)
		kinds = append(kinds, pl.kinds[i])
	}
	pl.comments, pl.kinds = comments, kinds
}
