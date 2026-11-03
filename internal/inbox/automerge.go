package inbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/automerge"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/marker"
)

// AutoMergeCandidate is the request to keep after the forge refused to merge the change request of ref at sha: the
// change request is open, still at sha, and conflicts with its base. It is nil when the refusal was for anything
// else, which the person sees as before (ADR-048).
func (b *Inbox) AutoMergeCandidate(ctx context.Context, m forge.Merges, ref forge.IssueRef, sha string) (*automerge.Request, error) {
	agentID, err := b.agentID(ctx, ref)
	if err != nil {
		return nil, err
	}
	_, comments, err := b.forgeOf(ref.Kind()).Thread(ctx, ref)
	if err != nil {
		return nil, err
	}
	_, dm := latestOf(comments, agentID, marker.Done)
	if dm == nil || dm.CR == 0 || dm.SHA != sha || len(comments) == 0 {
		return nil, nil
	}
	st, err := m.ChangeRequestHead(ctx, ref.Owner, ref.Repo, dm.CR)
	if err != nil {
		return nil, err
	}
	if st.State != forge.CROpen || st.Head != sha || !st.Conflict {
		return nil, nil
	}
	return &automerge.Request{Ref: refString(ref), CR: dm.CR, SHA: sha, Since: b.now(), After: comments[len(comments)-1].ID}, nil
}

func (b *Inbox) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

// AutoMergeStep is what one look at a request decided: Outcome is "" while it waits, else automerge.Merged or
// automerge.Cancelled with the Reason.
type AutoMergeStep struct {
	Outcome string
	Reason  string
	// SHA is the commit merged, when Outcome is automerge.Merged.
	SHA string
}

func cancelled(reason string) AutoMergeStep {
	return AutoMergeStep{Outcome: automerge.Cancelled, Reason: reason}
}

// AutoMergeStep looks at the request r once and merges its change request when the agent's fix runs have fixed it:
// the issue is done again, its latest done post is a fix run's at a new commit, and the change request's head is that
// commit. Only markers, labels and authors are read; no text of the issue is taken as an instruction. An error is a
// forge that could not be read or written, and leaves the request for the next look.
func (b *Inbox) AutoMergeStep(ctx context.Context, m forge.Merges, r automerge.Request) (AutoMergeStep, error) {
	ref, err := b.ParseRef(r.Ref)
	if err != nil {
		return cancelled(automerge.ReasonChanged), nil // the project left the configuration
	}
	if m == nil {
		return AutoMergeStep{}, fmt.Errorf("no client merges on %s", forgeName(ref.Kind()))
	}
	agentID, err := b.agentID(ctx, ref)
	if err != nil {
		return AutoMergeStep{}, err
	}
	trusted, err := b.trustedIDs(ctx, ref.Kind())
	if err != nil {
		return AutoMergeStep{}, err
	}
	is, comments, err := b.forgeOf(ref.Kind()).Thread(ctx, ref)
	if err != nil {
		return AutoMergeStep{}, err
	}
	n := b.Config.Labels
	st := labels.StateOf(is.Labels, n)
	switch {
	case is.State != "open":
		return cancelled(automerge.ReasonClosed), nil
	case st == n.Waiting:
		return cancelled(automerge.ReasonWaiting), nil
	case instructedAfter(comments, r.After, agentID, trusted):
		return cancelled(automerge.ReasonInstruction), nil
	case st != n.Done:
		return AutoMergeStep{}, nil // a fix run goes on, or the issue is between labels
	}
	_, dm := latestOf(comments, agentID, marker.Done)
	if dm == nil || dm.CR != r.CR || dm.SHA == "" {
		return cancelled(automerge.ReasonChanged), nil
	}
	cr, err := m.ChangeRequestHead(ctx, ref.Owner, ref.Repo, dm.CR)
	if err != nil {
		return AutoMergeStep{}, err
	}
	switch {
	case cr.State != forge.CROpen:
		return cancelled(automerge.ReasonChangeClosed), nil
	case cr.Head != dm.SHA:
		return cancelled(automerge.ReasonPushed), nil
	case dm.SHA == r.SHA:
		return AutoMergeStep{}, nil // the agent has not fixed the conflict yet
	case dm.Fix == "":
		return cancelled(automerge.ReasonNotFix), nil
	}
	err = m.MergeChangeRequest(ctx, ref.Owner, ref.Repo, dm.CR, dm.SHA)
	switch {
	case err == nil:
		return AutoMergeStep{Outcome: automerge.Merged, SHA: dm.SHA}, nil
	case errors.Is(err, forge.ErrHeadMoved):
		return cancelled(automerge.ReasonPushed), nil
	case errors.Is(err, forge.ErrForbidden):
		return cancelled(automerge.ReasonForbidden), nil
	case errors.Is(err, forge.ErrNotMergeable):
		return AutoMergeStep{}, nil // still conflicting, or the CI has not passed yet: tried again at the next look
	}
	return AutoMergeStep{}, err
}

// instructedAfter reports whether, after the comment after, a trusted person commented or the agent claimed a run that
// is not a fix run: a run a person asked for, whose changes the person has not seen. A trusted account's comment
// counts whether or not it was edited, since it only stops the merge.
func instructedAfter(comments []forge.Comment, after, agentID int64, trusted map[int64]bool) bool {
	for _, c := range comments {
		if c.ID <= after {
			continue
		}
		if c.Author.ID == agentID {
			if m, ok := marker.Parse(c.Body); ok {
				if m.State == marker.Running && m.Reason != marker.Fix {
					return true
				}
				continue
			}
		}
		if trusted[c.Author.ID] {
			return true
		}
	}
	return false
}

// trustedIDs resolves the trusted_authors of a forge to their account IDs, once per login.
func (b *Inbox) trustedIDs(ctx context.Context, kind string) (map[int64]bool, error) {
	out := map[int64]bool{}
	for _, login := range b.Config.TrustedAuthorsOf(kind) {
		key := kind + "\x00trusted\x00" + strings.ToLower(login)
		b.idMu.Lock()
		id, ok := b.ids[key]
		b.idMu.Unlock()
		if !ok {
			u, err := b.forgeOf(kind).User(ctx, login)
			if err != nil {
				return nil, fmt.Errorf("resolve trusted author %s: %w", login, err)
			}
			id = u.ID
			b.idMu.Lock()
			if b.ids == nil {
				b.ids = map[string]int64{}
			}
			b.ids[key] = id
			b.idMu.Unlock()
		}
		out[id] = true
	}
	return out, nil
}
