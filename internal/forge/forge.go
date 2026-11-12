// Package forge defines what switchboard needs from a forge (GitHub or GitLab). The domain packages depend on this
// interface only; the GitHub implementation and the in-memory fake live in subpackages.
package forge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound reports that the forge has no such object.
var ErrNotFound = errors.New("not found")

// ErrForbidden reports that the token may not do what was asked (HTTP 403).
var ErrForbidden = errors.New("forbidden")

// Attacher is a forge client that can post a comment with image files attached, as a person attaches them in the
// forge's web page (ADR-051). The comment ends with the last line of body, so that a marker there still parses.
type Attacher interface {
	AttachComment(ctx context.Context, ref IssueRef, body string, files []string) (*Comment, error)
}

// Uploader is a forge client that can upload one image file to a project and return the Markdown that shows it, to
// be put anywhere in a comment (ADR-069). Only the GitLab client is one: GitHub has no API for it (see PlacedAttacher).
type Uploader interface {
	Upload(ctx context.Context, ref IssueRef, path string) (string, error)
}

// PlacedImage is an image of a comment that a PlacedAttacher puts where Mark, a line of the body of its own, is.
type PlacedImage struct {
	Mark string `json:"mark"`
	Path string `json:"path"`
}

// PlacedError is the error of AttachPlaced. Left tells that the comment it posted is still there, its images not
// moved to their marks, since it could not be deleted: the caller must not post the comment again.
type PlacedError struct {
	Err  error
	Left bool
}

func (e *PlacedError) Error() string { return e.Err.Error() }
func (e *PlacedError) Unwrap() error { return e.Err }

// PlacedAttacher is a forge client that attaches image files to a comment and puts each where its mark is (ADR-070).
// Only the GitHub client is one: gh --attach puts the images after the body, and the comment is edited afterwards to
// move them. The comment ends with the last line of body. When the images cannot be put in place, the comment is
// deleted and an error returned, so that the caller can post the comment without them.
type PlacedAttacher interface {
	AttachPlaced(ctx context.Context, ref IssueRef, body string, images []PlacedImage) (*Comment, error)
}

// UploadReader reads back an image uploaded to a project (ADR-069): the inbox service shows the images of a plan
// question with it, through the inbox token, since a private project's uploads need one. Only GitLab has it.
type UploadReader interface {
	// ReadUpload returns the image uploaded to owner/repo as secret/name, checked by ImageTypeUpTo with
	// MaxQuestionImageBytes; ErrNotFound when there is none.
	ReadUpload(ctx context.Context, owner, repo, secret, name string) ([]byte, error)
}

// AttachmentReader reads back an image attached to a comment on GitHub (ADR-070), the one at
// https://github.com/user-attachments/assets/<id>, checked by ImageTypeUpTo with MaxQuestionImageBytes; ErrNotFound
// when there is none.
type AttachmentReader interface {
	ReadAttachment(ctx context.Context, id string) ([]byte, error)
}

// SplitLastLine returns body without its last non-blank line, and that line, so that an Attacher can put the
// attachments before it.
func SplitLastLine(body string) (head, last string) {
	body = strings.TrimRight(body, " \t\r\n")
	i := strings.LastIndex(body, "\n")
	if i < 0 {
		return "", body
	}
	return strings.TrimRight(body[:i], " \t\r\n"), body[i+1:]
}

// Hook is a webhook to register: where it is sent, and the secret it is signed with. On GitLab the secret is a
// signing token (whsec_...) unless SecretToken is set, which sends it as X-Gitlab-Token instead.
type Hook struct {
	URL         string
	Secret      string
	SecretToken bool
}

// Hooks registers and removes the relay's webhooks. It is kept apart from Forge: the agent never manages webhooks,
// and `relay hook add` calls it with the inbox token.
type Hooks interface {
	// CreateHook registers h for the issue and comment events of owner/repo and returns its ID.
	CreateHook(ctx context.Context, owner, repo string, h Hook) (int64, error)
	// DeleteHook removes the webhook of owner/repo sent to url, and reports whether there was one.
	DeleteHook(ctx context.Context, owner, repo, url string) (bool, error)
}

// Repos reads what adding a project needs to know of a repository. It is kept apart from Forge: the inbox service
// calls it with the inbox token.
type Repos interface {
	// DefaultBranch returns the repository's default branch; ErrNotFound when the token cannot see the repository.
	DefaultBranch(ctx context.Context, owner, repo string) (string, error)
	// LabelNames lists the names of every label of the repository.
	LabelNames(ctx context.Context, owner, repo string) ([]string, error)
}

// Issues opens issues. It is kept apart from Forge: the agent never opens one, and the inbox service calls it with
// the inbox token, so the issue is the person's own (ADR-040).
type Issues interface {
	// CreateIssue opens an issue in owner/repo carrying labels and returns it; ErrNotFound when the token cannot see
	// the repository.
	CreateIssue(ctx context.Context, owner, repo, title, body string, labels []string) (*IssueSummary, error)
}

// ErrHeadMoved reports that the head of a change request is no longer the commit the caller saw.
var ErrHeadMoved = errors.New("head moved")

// ErrNotMergeable reports that the forge refused to merge a change request. A *NotMergeableError carries the reason.
var ErrNotMergeable = errors.New("not mergeable")

// NotMergeableError is the forge's refusal to merge, with the reason it gave. It matches ErrNotMergeable.
type NotMergeableError struct{ Reason string }

func (e *NotMergeableError) Error() string {
	if e.Reason == "" {
		return ErrNotMergeable.Error()
	}
	return ErrNotMergeable.Error() + ": " + e.Reason
}

// Is makes errors.Is(err, ErrNotMergeable) true.
func (e *NotMergeableError) Is(target error) bool { return target == ErrNotMergeable }

// Merges merges change requests. It is kept apart from Forge: the agent never merges, and the inbox service calls it
// with the inbox token when the person presses the merge button, so the merge is the person's own (ADR-043).
type Merges interface {
	// MergeChangeRequest marks the pull request (merge request) number of owner/repo ready when it is a draft and
	// merges it, only while its head is sha. It returns ErrHeadMoved when the head is another commit, a
	// *NotMergeableError when the forge refuses, ErrForbidden when the token may not, and ErrNotFound when there is
	// no such change request.
	MergeChangeRequest(ctx context.Context, owner, repo string, number int, sha string) error
	// ChangeRequestHead returns the state, the head commit and the conflict with the base of the change request
	// number, as CI's does: the inbox service reads it to tell a merge refused for a conflict, and to merge after the
	// agent fixed it (ADR-048).
	ChangeRequestHead(ctx context.Context, owner, repo string, number int) (*ChangeRequestState, error)
}

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
	URL       string // the issue's page on the forge
	Labels    []string
	UpdatedAt time.Time
	// Notes is how many comments people wrote, as GitLab's user_notes_count. GitLab does not move UpdatedAt for a
	// new comment (V22), so the agent watches this count on waiting issues. Zero on GitHub, which moves UpdatedAt.
	Notes int
}

// Issue is one issue with its body.
type Issue struct {
	IssueSummary
	Body     string
	Author   User
	BodyEdit Edit
	// TitleEdit is who last changed the title after the issue was opened.
	TitleEdit Edit
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
	// EditComment replaces the body of a comment of the issue; ErrNotFound when there is none with that ID.
	EditComment(ctx context.Context, ref IssueRef, id int64, body string) error
	// DeleteComment removes a comment of the issue; ErrNotFound when there is none with that ID.
	DeleteComment(ctx context.Context, ref IssueRef, id int64) error
	// EnsureLabel creates the label unless one with that name exists; it reports whether it created one.
	EnsureLabel(ctx context.Context, owner, repo, name, color, description string) (bool, error)
	// CreateChangeRequest opens a pull request and returns its URL.
	CreateChangeRequest(ctx context.Context, cr ChangeRequest) (string, error)
	// OpenChangeRequest returns the URL of an open pull request from head to base, or ErrNotFound.
	OpenChangeRequest(ctx context.Context, owner, repo, head, base string) (string, error)
	// RepoPrivate reports whether the repository is private.
	RepoPrivate(ctx context.Context, owner, repo string) (bool, error)
	// User resolves a login to the user, with its numeric ID.
	User(ctx context.Context, login string) (*User, error)
	// SearchOpenIssuesWithLabel lists the open issues within scope that carry any of labels, each once. How the
	// projects are asked (one search, or one request per project) is the adapter's business.
	SearchOpenIssuesWithLabel(ctx context.Context, scope Scope, labels ...string) ([]IssueSummary, error)
}

// Scope is where SearchOpenIssuesWithLabel looks.
type Scope struct {
	// Projects are the full names of the projects ("owner/repo", "group/sub/project").
	Projects []string
	// Query, when set, takes the place of Projects on a forge that searches by a query: GitHub's search_scope, such
	// as "user:alice". A forge without such a search ignores it.
	Query string
}

// Projects is the scope of the named projects alone.
func Projects(names ...string) Scope { return Scope{Projects: names} }
