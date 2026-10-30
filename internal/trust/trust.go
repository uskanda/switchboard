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

// Judge holds one forge's trusted user IDs and the agent's own user ID on it. AgentIDs, keyed by "gl:group/project",
// holds the agent's ID on projects that override agent_identity.
type Judge struct {
	Trusted  map[int64]bool
	AgentID  int64
	AgentIDs map[string]int64
	Audit    state.Auditor
}

// AgentIDOf is the agent's user ID on the issue's project.
func (j *Judge) AgentIDOf(ref forge.IssueRef) int64 {
	if id, ok := j.AgentIDs[ref.Kind()+":"+ref.Project()]; ok {
		return id
	}
	return j.AgentID
}

// Resolve is ResolveFor on GitHub without project overrides, as Phase 1 configured it.
func Resolve(ctx context.Context, f forge.Forge, d state.Dir, trusted []string, agent string, audit state.Auditor) (*Judge, error) {
	return ResolveFor(ctx, f, d, forge.GitHub, trusted, agent, nil, audit)
}

// ResolveFor turns the logins configured for one forge (kind) into user IDs, compares them with the IDs recorded in
// d, and records new ones. overrides maps a project ("gl:group/project") to the agent_identity it uses instead of
// agent. A login whose ID changed (renamed and taken by someone else) is an error, so the agent does not start.
// Records are keyed "gh:login" or "gl:login"; a key without a forge is a Phase 1 GitHub record. audit may be nil.
func ResolveFor(ctx context.Context, f forge.Forge, d state.Dir, kind string, trusted []string, agent string, overrides map[string]string, audit state.Auditor) (*Judge, error) {
	recorded, err := d.Identities()
	if err != nil {
		return nil, err
	}
	logins := append(append([]string(nil), trusted...), agent)
	for _, l := range overrides {
		logins = append(logins, l)
	}
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
		old, ok := recorded[kind+":"+login]
		if !ok && kind == forge.GitHub {
			old, ok = recorded[login]
		}
		if ok && old != u.ID {
			changed = append(changed, fmt.Sprintf("%s (recorded %d, now %d)", login, old, u.ID))
		}
		ids[login] = u.ID
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		return nil, fmt.Errorf("the user ID behind these logins changed: %s; check them, then remove them from identities.json in %s", strings.Join(changed, ", "), d.Path)
	}
	for k, v := range ids {
		recorded[kind+":"+k] = v
		if kind == forge.GitHub {
			delete(recorded, k)
		}
	}
	if err := d.SaveIdentities(recorded); err != nil {
		return nil, err
	}
	j := &Judge{Trusted: map[int64]bool{}, AgentID: ids[agent], AgentIDs: map[string]int64{}, Audit: audit}
	for _, login := range trusted {
		j.Trusted[ids[login]] = true
	}
	for project, login := range overrides {
		j.AgentIDs[project] = ids[login]
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

// editOK reports whether an edit leaves the text trustworthy: unedited, or last edited by a trusted user. When it
// does not, reason says why.
func (j *Judge) editOK(e forge.Edit) (ok bool, reason string) {
	switch {
	case !e.Edited:
		return true, ""
	case e.Unknown:
		return false, "the last editor is unknown"
	case !j.trusted(e.Editor):
		return false, "last edited by an untrusted user"
	}
	return true, ""
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
	default:
		if ok, reason = j.editOK(is.BodyEdit); ok {
			reason = "trusted author, no untrusted edit"
		}
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
	editOK, why := j.editOK(c.Edit)
	if _, isMarked := marker.Parse(c.Body); isMarked && c.Author.ID == j.AgentIDOf(ref) && editOK {
		j.log(ref, subject, &c.Author, true, "agent post", c.Body)
		return AgentPost
	}
	if !j.trusted(&c.Author) {
		j.log(ref, subject, &c.Author, false, "author is not trusted", c.Body)
		return Untrusted
	}
	if !editOK {
		j.log(ref, subject, &c.Author, false, why, c.Body)
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
