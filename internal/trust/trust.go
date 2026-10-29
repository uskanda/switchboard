// Package trust decides which input from an issue the agent takes as an instruction, and which comments are the
// agent's own posts. Every decision is recorded in the audit log.
package trust

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
)

// Judge holds the trusted user IDs and the agent's own user ID.
type Judge struct {
	Trusted map[int64]bool
	AgentID int64
	Audit   state.Auditor
}

// Resolve turns the configured logins into user IDs, compares them with the IDs recorded in d, and records new
// ones. A login whose ID changed (renamed and taken by someone else) is an error, so the agent does not start.
// audit may be nil.
func Resolve(ctx context.Context, f forge.Forge, d state.Dir, trusted []string, agent string, audit state.Auditor) (*Judge, error) {
	recorded, err := d.Identities()
	if err != nil {
		return nil, err
	}
	logins := append(append([]string(nil), trusted...), agent)
	ids := map[string]int64{}
	var changed []string
	for _, login := range logins {
		if _, done := ids[login]; done {
			continue
		}
		u, err := f.User(ctx, login)
		if err != nil {
			return nil, fmt.Errorf("resolve user %s: %w", login, err)
		}
		if old, ok := recorded[login]; ok && old != u.ID {
			changed = append(changed, fmt.Sprintf("%s (recorded %d, now %d)", login, old, u.ID))
		}
		ids[login] = u.ID
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		return nil, fmt.Errorf("the user ID behind these logins changed: %s; check them, then remove them from identities.json in %s", strings.Join(changed, ", "), d.Path)
	}
	for k, v := range ids {
		recorded[k] = v
	}
	if err := d.SaveIdentities(recorded); err != nil {
		return nil, err
	}
	j := &Judge{Trusted: map[int64]bool{}, AgentID: ids[agent], Audit: audit}
	for _, login := range trusted {
		j.Trusted[ids[login]] = true
	}
	return j, nil
}

func (j *Judge) log(ref forge.IssueRef, subject string, actor *forge.User, ok bool, reason, text string) {
	if j.Audit == nil {
		return
	}
	verdict := "reject"
	if ok {
		verdict = "accept"
	}
	fields := map[string]any{"issue": ref.String(), "subject": subject, "verdict": verdict, "reason": reason}
	if actor != nil {
		fields["actor_id"] = actor.ID
	}
	if text != "" {
		sum := sha256.Sum256([]byte(text))
		fields["sha256"] = hex.EncodeToString(sum[:])
	}
	j.Audit.Log("trust."+strings.SplitN(subject, ":", 2)[0], fields)
}

func (j *Judge) trusted(u *forge.User) bool { return u != nil && j.Trusted[u.ID] }

// editOK reports whether an edit leaves the text trustworthy: unedited, or last edited by a trusted user.
func (j *Judge) editOK(e forge.Edit) bool {
	if !e.Edited {
		return true
	}
	return !e.Unknown && j.trusted(e.Editor)
}

// ReadyLabel reports whether the user who last added the ready label (nil when unknown) is trusted.
func (j *Judge) ReadyLabel(ref forge.IssueRef, actor *forge.User) bool {
	ok := j.trusted(actor)
	reason := "labeled by a trusted user"
	if !ok {
		reason = "labeled by an untrusted or unknown user"
	}
	j.log(ref, "label:ready", actor, ok, reason, "")
	return ok
}

// Body reports whether the issue title and body may be taken as an instruction.
func (j *Judge) Body(is *forge.Issue) bool {
	var ok bool
	var reason string
	switch {
	case !j.trusted(&is.Author):
		reason = "author is not trusted"
	case !j.editOK(is.BodyEdit):
		reason = "last edited by an untrusted or unknown user"
	default:
		ok, reason = true, "trusted author, no untrusted edit"
	}
	j.log(is.Ref, "body", &is.Author, ok, reason, is.Title+"\n"+is.Body)
	return ok
}

// Kind classifies a comment.
type Kind int

const (
	// Untrusted comments are never instructions.
	Untrusted Kind = iota
	// Instruction comments come from a trusted user and were not edited by anyone untrusted.
	Instruction
	// AgentPost comments were posted by the agent: authored by the agent's ID, ending in a marker.
	AgentPost
)

func (k Kind) String() string {
	return [...]string{"untrusted", "instruction", "agent-post"}[k]
}

// Comment classifies c and records the decision.
func (j *Judge) Comment(ref forge.IssueRef, c forge.Comment) Kind {
	subject := fmt.Sprintf("comment:%d", c.ID)
	if _, isMarked := marker.Parse(c.Body); isMarked && c.Author.ID == j.AgentID && j.editOK(c.Edit) {
		j.log(ref, subject, &c.Author, true, "agent post", c.Body)
		return AgentPost
	}
	if !j.trusted(&c.Author) {
		j.log(ref, subject, &c.Author, false, "author is not trusted", c.Body)
		return Untrusted
	}
	if !j.editOK(c.Edit) {
		j.log(ref, subject, &c.Author, false, "last edited by an untrusted or unknown user", c.Body)
		return Untrusted
	}
	j.log(ref, subject, &c.Author, true, "trusted author", c.Body)
	return Instruction
}

// ErrUntrustedOnPublic reports include_untrusted on a public repository.
var ErrUntrustedOnPublic = errors.New("include_untrusted is refused for a public repository")

// UntrustedPolicy reports whether untrusted comments go into the prompt as reference data.
func UntrustedPolicy(includeUntrusted, private bool) (bool, error) {
	if !includeUntrusted {
		return false, nil
	}
	if !private {
		return false, ErrUntrustedOnPublic
	}
	return true, nil
}
