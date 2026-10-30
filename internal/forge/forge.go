// Package forge defines what switchboard needs from a forge (GitHub or GitLab). The domain packages depend on this
// interface only; the GitHub implementation and the in-memory fake live in subpackages.
package forge

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound reports that the forge has no such object.
var ErrNotFound = errors.New("not found")

// The forge kinds, as written before the colon of a project reference.
const (
	GitHub = "gh"
	GitLab = "gl"
)

// IssueRef names one issue. On GitLab, Owner is the namespace and may hold subgroups ("nas/tools").
type IssueRef struct {
	Forge  string // GitHub or GitLab; empty means GitHub, as in Phase 1
	Owner  string
	Repo   string
	Number int
}

// Kind is the forge of the issue, GitHub when Forge is empty.
func (r IssueRef) Kind() string {
	if r.Forge == "" {
		return GitHub
	}
	return r.Forge
}

// Project is "owner/repo", the path of the project on its forge.
func (r IssueRef) Project() string { return r.Owner + "/" + r.Repo }

// String is "gh:owner/repo#n" or "gl:group/project#n", the form switchboard show and reply accept.
func (r IssueRef) String() string { return fmt.Sprintf("%s:%s#%d", r.Kind(), r.Project(), r.Number) }

// User is an account. ID is the forge's numeric user ID, which survives a rename; trust decisions use it.
type User struct {
	Login string
	ID    int64
}

// Edit describes the last edit of an issue body or a comment.
type Edit struct {
	Edited  bool  // the text was changed after it was created
	Unknown bool  // the forge did not say who edited it; callers treat the editor as untrusted
	Editor  *User // the last editor when Edited and not Unknown
}

// IssueSummary is what a listing returns.
type IssueSummary struct {
	Ref       IssueRef
	Title     string
	State     string // "open" or "closed"
	Labels    []string
	UpdatedAt time.Time
}

// Issue is one issue with its body.
type Issue struct {
	IssueSummary
	Body      string
	Author    User
	BodyEdit  Edit
	CreatedAt time.Time
}

// Comment is one issue comment.
type Comment struct {
	ID        int64
	Author    User
	Body      string
	CreatedAt time.Time
	Edit      Edit
}

// ChangeRequest is a pull request to open.
type ChangeRequest struct {
	Owner, Repo string
	Title, Body string
	Head, Base  string
	Draft       bool
}

// Forge is the set of forge operations switchboard uses.
type Forge interface {
	// UpdatedIssues lists the issues (not pull requests) of a repository updated at or after since.
	UpdatedIssues(ctx context.Context, owner, repo string, since time.Time) ([]IssueSummary, error)
	// Thread returns an issue and all its comments, oldest first, with who last edited each.
	Thread(ctx context.Context, ref IssueRef) (*Issue, []Comment, error)
	// LabelActor returns who most recently added label to the issue, or ErrNotFound.
	LabelActor(ctx context.Context, ref IssueRef, label string) (*User, error)
	// SetLabels replaces every label of the issue with labels.
	SetLabels(ctx context.Context, ref IssueRef, labels []string) error
	// PostComment adds a comment and returns it.
	PostComment(ctx context.Context, ref IssueRef, body string) (*Comment, error)
	// EnsureLabel creates the label unless one with that name exists; it reports whether it created one.
	EnsureLabel(ctx context.Context, owner, repo, name, color, description string) (bool, error)
	// CreateChangeRequest opens a pull request and returns its URL.
	CreateChangeRequest(ctx context.Context, cr ChangeRequest) (string, error)
	// RepoPrivate reports whether the repository is private.
	RepoPrivate(ctx context.Context, owner, repo string) (bool, error)
	// User resolves a login to the user, with its numeric ID.
	User(ctx context.Context, login string) (*User, error)
	// SearchOpenIssuesWithLabel lists the open issues within scope (such as "user:alice") that carry label.
	SearchOpenIssuesWithLabel(ctx context.Context, scope, label string) ([]IssueSummary, error)
}
