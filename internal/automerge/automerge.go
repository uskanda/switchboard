// Package automerge keeps the merges the inbox service takes on after the forge refused them for a conflict: the
// person pressed merge, the change request conflicted with its base, and the inbox service merges it once the agent's
// fix runs have fixed it (ADR-048). The requests are kept in a file of the state directory, so that they survive a
// restart; issue labels and comments are never used for them.
package automerge

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/state"
)

// File is where the requests are kept, inside the state directory.
const File = "inbox/automerge.json"

// Keep is how long a finished request is kept, for a page opened later to tell the person how it ended.
const Keep = 24 * time.Hour

// Request is one merge to make once the conflict is fixed: the issue, its change request, the commit the person
// pressed merge on, when, and the ID of the issue's last comment then. What comes after that comment decides whether
// the merge still goes ahead.
type Request struct {
	Ref   string    `json:"ref"`
	CR    int       `json:"cr"`
	SHA   string    `json:"sha"`
	Since time.Time `json:"since"`
	After int64     `json:"after"`
}

// The outcomes of a finished request.
const (
	Merged    = "merged"
	Cancelled = "cancelled"
)

// The reasons a request is cancelled.
const (
	ReasonClosed       = "closed"        // the issue was closed
	ReasonWaiting      = "waiting"       // the issue waits for a person, such as after the fixes ran out
	ReasonInstruction  = "instruction"   // a person commented, or a run that is not a fix run started
	ReasonNotFix       = "not_fix"       // a newer done post is not a fix run's
	ReasonChanged      = "changed"       // the done post names another change request, or none
	ReasonChangeClosed = "change_closed" // the change request was closed or merged
	ReasonPushed       = "pushed"        // the change request's head is not the agent's commit
	ReasonForbidden    = "forbidden"     // the inbox token may not merge
	ReasonUser         = "user"          // the person cancelled it
	ReasonRestarted    = "restarted"     // the inbox service restarted and forgot who asked for it (issue #146)
)

// Result is a finished request: merged, or cancelled for Reason.
type Result struct {
	Ref     string    `json:"ref"`
	CR      int       `json:"cr"`
	Outcome string    `json:"outcome"`
	Reason  string    `json:"reason,omitempty"`
	At      time.Time `json:"at"`
}

type content struct {
	Pending  []Request `json:"pending"`
	Finished []Result  `json:"finished"`
}

// Store is the requests of one inbox service. It is safe for concurrent use.
type Store struct {
	Now func() time.Time // time.Now when nil

	dir  state.Dir
	mu   sync.Mutex
	data content
}

// Open reads the requests kept in d; none when the file does not exist.
func Open(d state.Dir) (*Store, error) {
	s := &Store{dir: d}
	err := d.ReadJSON(File, &s.data)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", File, err)
	}
	return s, nil
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// save writes the requests, without the results older than Keep. Call it with s.mu held.
func (s *Store) save() error {
	s.prune()
	return s.dir.WriteJSON(File, s.data)
}

// prune drops the results older than Keep. Call it with s.mu held.
func (s *Store) prune() {
	cut := s.now().Add(-Keep)
	s.data.Finished = slices.DeleteFunc(s.data.Finished, func(r Result) bool { return r.At.Before(cut) })
}

// Put keeps r, replacing a request of the same issue.
func (s *Store) Put(r Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Pending = slices.DeleteFunc(s.data.Pending, func(p Request) bool { return p.Ref == r.Ref })
	s.data.Pending = append(s.data.Pending, r)
	return s.save()
}

// Get returns the request of the issue ref, and whether there is one.
func (s *Store) Get(ref string) (Request, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.data.Pending {
		if p.Ref == ref {
			return p, true
		}
	}
	return Request{}, false
}

// Pending lists the requests, in the order they were made.
func (s *Store) Pending() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data.Pending)
}

// Finish ends r with outcome and reason, and keeps the result for Keep. A request of the same issue made after r (a
// new press while r was looked at) is left as it is.
func (s *Store) Finish(r Request, outcome, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.data.Pending, func(p Request) bool { return p.Ref == r.Ref && p.Since.Equal(r.Since) })
	if i < 0 {
		return nil
	}
	s.data.Pending = slices.Delete(s.data.Pending, i, i+1)
	s.data.Finished = append(s.data.Finished, Result{Ref: r.Ref, CR: r.CR, Outcome: outcome, Reason: reason, At: s.now()})
	return s.save()
}

// Cancel ends the request of the issue ref as cancelled by the person, and reports whether there was one.
func (s *Store) Cancel(ref string) (bool, error) {
	r, ok := s.Get(ref)
	if !ok {
		return false, nil
	}
	return true, s.Finish(r, Cancelled, ReasonUser)
}

// Results lists the requests finished within Keep, oldest first.
func (s *Store) Results() []Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	return slices.Clone(s.data.Finished)
}
