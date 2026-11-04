// Package fake is an in-memory forge for tests. It is safe for concurrent use.
package fake

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// Forge holds repositories, issues, comments, label events, users and pull requests in memory.
type Forge struct {
	mu       sync.Mutex
	kind     string // forge.GitHub or forge.GitLab
	now      func() time.Time
	nextID   int64
	repos    map[string]*repo
	users    map[string]forge.User
	PRs      []forge.ChangeRequest
	closed   map[int]bool // indexes into PRs that ClosePR closed
	Comments int          // number of PostComment calls, for tests that count writes
	Edits    int          // number of EditComment calls
	// PostAs is the login PostComment posts as; "agent" when empty.
	PostAs string
	// FailSetLabels, when set, is returned by the next SetLabels call and then cleared.
	FailSetLabels error
	// FailEnsureLabel, when set, is returned by the next EnsureLabel call and then cleared.
	FailEnsureLabel error
	failNth         int // FailNthSetLabels: calls left until the failing one
	failNthEr       error
	// AfterSetLabels, when set, runs after each successful SetLabels, with the lock released. Tests use it to
	// simulate a human changing labels right after the agent did.
	AfterSetLabels func(ref forge.IssueRef)
	// AfterPostComment, when set, runs after each successful PostComment, with the lock released. Tests use it to
	// act while the agent is between two calls to the forge.
	AfterPostComment func(ref forge.IssueRef)

	failThread, failPost     int // FailNthThread, FailNthPostComment: calls left until the failing one
	failThreadEr, failPostEr error
	failThreadOf             *forge.IssueRef // FailNthThreadOf: only the calls for this issue count
	// FailSearch, when set, is returned by the next SearchOpenIssuesWithLabel call and then cleared.
	FailSearch error
	// FailRepoPrivate, when set, is returned by the next RepoPrivate call and then cleared.
	FailRepoPrivate error
	// FailCI, when set, is returned by every CommitChecks and CheckLog call.
	FailCI error
	ci     *ci
	// FailMerge, when set, is returned by every MergeChangeRequest call.
	FailMerge error
	// FailHead, when set, is returned by every ChangeRequestHead call.
	FailHead error
	changes  map[string]*Change // change requests by changeKey, for MergeChangeRequest
	// FailAttach, when set, is returned by every AttachComment call.
	FailAttach error
	// Attached are the files of each AttachComment call that posted, by base name.
	Attached [][]string
}

type repo struct {
	private bool
	branch  string // the default branch; main when empty
	avatar  []byte // the project's avatar; none when nil
	labels  map[string]bool
	issues  map[int]*issue
}

type issue struct {
	data     forge.Issue
	comments []forge.Comment
	labeled  map[string]forge.User // label -> who added it last
}

// New returns an empty fake forge. Time comes from now, or time.Now when nil.
func New(now func() time.Time) *Forge {
	if now == nil {
		now = time.Now
	}
	return &Forge{kind: forge.GitHub, now: now, nextID: 1000, repos: map[string]*repo{}, users: map[string]forge.User{}}
}

// NewGitLab returns an empty fake GitLab. Its issues are named gl:, and, as on GitLab (V22), adding a note does not
// change an issue's update time.
func NewGitLab(now func() time.Time) *Forge {
	f := New(now)
	f.kind = forge.GitLab
	return f
}

// AddUser registers a login with its ID.
func (f *Forge) AddUser(login string, id int64) forge.User {
	f.mu.Lock()
	defer f.mu.Unlock()
	u := forge.User{Login: login, ID: id}
	f.users[login] = u
	return u
}

// AddRepo creates a repository.
func (f *Forge) AddRepo(owner, name string, private bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos[owner+"/"+name] = &repo{private: private, labels: map[string]bool{}, issues: map[int]*issue{}}
}

// AddProject creates a GitLab project with a visibility of "private", "internal" or "public". Only a private one is
// private: internal ones are readable by every user of the instance.
func (f *Forge) AddProject(namespace, name, visibility string) {
	f.AddRepo(namespace, name, visibility == "private")
}

// AddIssue creates an open issue authored by author and returns its reference.
func (f *Forge) AddIssue(owner, name string, number int, title, body string, author forge.User) forge.IssueRef {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.repos[owner+"/"+name]
	ref := forge.IssueRef{Forge: f.kind, Owner: owner, Repo: name, Number: number}
	now := f.now()
	r.issues[number] = &issue{
		data: forge.Issue{
			IssueSummary: forge.IssueSummary{Ref: ref, Title: title, State: "open", URL: f.issueURL(ref), UpdatedAt: now},
			Body:         body, Author: author, CreatedAt: now,
		},
		labeled: map[string]forge.User{},
	}
	return ref
}

// AddLabel adds label to the issue as by, the way a person does from the web interface.
func (f *Forge) AddLabel(ref forge.IssueRef, label string, by forge.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	is := f.issue(ref)
	if !contains(is.data.Labels, label) {
		is.data.Labels = append(is.data.Labels, label)
	}
	is.labeled[label] = by
	is.data.UpdatedAt = f.now()
}

// RemoveLabel removes label from the issue, as a person does.
func (f *Forge) RemoveLabel(ref forge.IssueRef, label string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	is := f.issue(ref)
	is.data.Labels = remove(is.data.Labels, label)
	is.data.UpdatedAt = f.now()
}

// Close closes the issue.
func (f *Forge) Close(ref forge.IssueRef) {
	f.mu.Lock()
	defer f.mu.Unlock()
	is := f.issue(ref)
	is.data.State = "closed"
	is.data.UpdatedAt = f.now()
}

// EditBody replaces the issue body as editor.
func (f *Forge) EditBody(ref forge.IssueRef, body string, editor forge.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	is := f.issue(ref)
	is.data.Body = body
	is.data.BodyEdit = forge.Edit{Edited: true, Editor: &editor}
	is.data.UpdatedAt = f.now()
}

// EditTitle renames the issue as editor.
func (f *Forge) EditTitle(ref forge.IssueRef, title string, editor forge.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	is := f.issue(ref)
	is.data.Title = title
	is.data.TitleEdit = forge.Edit{Edited: true, Editor: &editor}
	is.data.UpdatedAt = f.now()
}

// LoseBodyEditor replaces the issue body by someone the forge does not name, as when GitLab's system notes are beyond
// what was fetched.
func (f *Forge) LoseBodyEditor(ref forge.IssueRef, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	is := f.issue(ref)
	is.data.Body = body
	is.data.BodyEdit = forge.Edit{Edited: true, Unknown: true}
	is.data.UpdatedAt = f.now()
}

// AddComment adds a comment as author and returns it.
func (f *Forge) AddComment(ref forge.IssueRef, body string, author forge.User) forge.Comment {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addComment(ref, body, author)
}

// EditCommentAs replaces a comment body as editor, as a person editing it on the forge would.
func (f *Forge) EditCommentAs(ref forge.IssueRef, id int64, body string, editor forge.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	is := f.issue(ref)
	for i := range is.comments {
		if is.comments[i].ID == id {
			is.comments[i].Body = body
			is.comments[i].Edit = forge.Edit{Edited: true, Editor: &editor}
		}
	}
	is.data.UpdatedAt = f.now()
}

// Labels returns the issue's labels, sorted.
// UpdatedAt is the issue's update time.
func (f *Forge) UpdatedAt(ref forge.IssueRef) time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.issue(ref).data.UpdatedAt
}

func (f *Forge) Labels(ref forge.IssueRef) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string(nil), f.issue(ref).data.Labels...)
	sort.Strings(out)
	return out
}

// CommentsOf returns the issue's comments.
func (f *Forge) CommentsOf(ref forge.IssueRef) []forge.Comment {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]forge.Comment(nil), f.issue(ref).comments...)
}

// RepoLabels returns the labels defined in a repository, sorted.
func (f *Forge) RepoLabels(owner, name string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for l := range f.repos[owner+"/"+name].labels {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

func (f *Forge) issue(ref forge.IssueRef) *issue {
	r, ok := f.repos[ref.Owner+"/"+ref.Repo]
	if !ok {
		panic("fake: no repo " + ref.Owner + "/" + ref.Repo)
	}
	is, ok := r.issues[ref.Number]
	if !ok {
		panic(fmt.Sprintf("fake: no issue %s", ref))
	}
	return is
}

func (f *Forge) addComment(ref forge.IssueRef, body string, author forge.User) forge.Comment {
	is := f.issue(ref)
	f.nextID++
	c := forge.Comment{ID: f.nextID, Author: author, Body: body, CreatedAt: f.now()}
	is.comments = append(is.comments, c)
	if f.kind != forge.GitLab {
		is.data.UpdatedAt = f.now()
	}
	return c
}

// summary is what a listing returns for is. A GitLab fake counts the comments, as user_notes_count does.
func (f *Forge) summary(is *issue) forge.IssueSummary {
	s := is.data.IssueSummary
	s.Labels = append([]string(nil), s.Labels...)
	if f.kind == forge.GitLab {
		s.Notes = len(is.comments)
	}
	return s
}

// UpdatedIssues implements forge.Forge.
func (f *Forge) UpdatedIssues(_ context.Context, owner, name string, since time.Time) ([]forge.IssueSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.repos[owner+"/"+name]
	if !ok {
		return nil, forge.ErrNotFound
	}
	var out []forge.IssueSummary
	for _, is := range r.issues {
		if !is.data.UpdatedAt.Before(since) {
			out = append(out, f.summary(is))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref.Number < out[j].Ref.Number })
	return out, nil
}

// FailNthThread makes the n-th Thread call from now fail with err.
func (f *Forge) FailNthThread(n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failThread, f.failThreadEr, f.failThreadOf = n, err, nil
}

// FailNthThreadOf is FailNthThread counting only the calls that read ref.
func (f *Forge) FailNthThreadOf(ref forge.IssueRef, n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failThread, f.failThreadEr, f.failThreadOf = n, err, &ref
}

// FailNthPostComment makes the n-th PostComment call from now fail with err; the comment is not posted.
func (f *Forge) FailNthPostComment(n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failPost, f.failPostEr = n, err
}

// nth counts down *left and reports whether this call is the failing one. Call it with f.mu held.
func nth(left *int) bool {
	if *left <= 0 {
		return false
	}
	*left--
	return *left == 0
}

// DeleteIssue removes an issue, as a deletion or a transfer does: it is no longer listed, and reading it reports
// forge.ErrNotFound.
func (f *Forge) DeleteIssue(ref forge.IssueRef) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issue(ref)
	delete(f.repos[ref.Owner+"/"+ref.Repo].issues, ref.Number)
}

// Thread implements forge.Forge.
func (f *Forge) Thread(_ context.Context, ref forge.IssueRef) (*forge.Issue, []forge.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if (f.failThreadOf == nil || *f.failThreadOf == ref) && nth(&f.failThread) {
		return nil, nil, f.failThreadEr
	}
	if r, ok := f.repos[ref.Owner+"/"+ref.Repo]; ok && r.issues[ref.Number] == nil {
		return nil, nil, forge.ErrNotFound
	}
	is := f.issue(ref)
	d := is.data
	d.Labels = append([]string(nil), d.Labels...)
	return &d, append([]forge.Comment(nil), is.comments...), nil
}

// LabelActor implements forge.Forge.
func (f *Forge) LabelActor(_ context.Context, ref forge.IssueRef, label string) (*forge.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.issue(ref).labeled[label]
	if !ok {
		return nil, forge.ErrNotFound
	}
	return &u, nil
}

// FailNthSetLabels makes the n-th SetLabels call from now fail with err. Unlike FailSetLabels, it is safe to call
// while other goroutines use the forge.
func (f *Forge) FailNthSetLabels(n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failNth, f.failNthEr = n, err
}

// SetLabels implements forge.Forge. The labels it adds are attributed to the user PostComment posts as ("agent" by
// default, the zero user when not registered), as a forge attributes them to the token's owner.
func (f *Forge) SetLabels(_ context.Context, ref forge.IssueRef, labels []string) error {
	f.mu.Lock()
	if err := f.FailSetLabels; err != nil {
		f.FailSetLabels = nil
		f.mu.Unlock()
		return err
	}
	if f.failNth > 0 {
		f.failNth--
		if f.failNth == 0 {
			f.mu.Unlock()
			return f.failNthEr
		}
	}
	is := f.issue(ref)
	as := f.PostAs
	if as == "" {
		as = "agent"
	}
	for _, l := range labels {
		if !contains(is.data.Labels, l) {
			is.labeled[l] = f.users[as] // as on the forges, the token's owner added the label
		}
	}
	is.data.Labels = append([]string(nil), labels...)
	is.data.UpdatedAt = f.now()
	hook := f.AfterSetLabels
	f.mu.Unlock()
	if hook != nil {
		hook(ref)
	}
	return nil
}

// PostComment implements forge.Forge. The comment is authored by the user named by PostAs ("agent" by default),
// or the zero user when no such user is registered.
func (f *Forge) PostComment(_ context.Context, ref forge.IssueRef, body string) (*forge.Comment, error) {
	f.mu.Lock()
	as := f.PostAs
	f.mu.Unlock()
	if as == "" {
		as = "agent"
	}
	return f.PostCommentAs(ref, body, as)
}

// PostCommentAs posts as the user registered as login. Tests that give the agent and the user separate clients of
// one fake forge use it through As.
func (f *Forge) PostCommentAs(ref forge.IssueRef, body, login string) (*forge.Comment, error) {
	f.mu.Lock()
	if nth(&f.failPost) {
		f.mu.Unlock()
		return nil, f.failPostEr
	}
	f.Comments++
	c := f.addComment(ref, body, f.users[login])
	hook := f.AfterPostComment
	f.mu.Unlock()
	if hook != nil {
		hook(ref)
	}
	return &c, nil
}

// EditComment implements forge.Forge. As on GitHub, editing a comment moves the issue's update time; a GitLab
// fake leaves it, as for a new note.
func (f *Forge) EditComment(_ context.Context, ref forge.IssueRef, id int64, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Edits++
	is := f.issue(ref)
	for i, c := range is.comments {
		if c.ID == id {
			is.comments[i].Body = body
			if f.kind != forge.GitLab {
				is.data.UpdatedAt = f.now()
			}
			return nil
		}
	}
	return forge.ErrNotFound
}

// DeleteComment implements forge.Forge.
func (f *Forge) DeleteComment(_ context.Context, ref forge.IssueRef, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	is := f.issue(ref)
	for i, c := range is.comments {
		if c.ID == id {
			is.comments = append(is.comments[:i], is.comments[i+1:]...)
			return nil
		}
	}
	return forge.ErrNotFound
}

// As is a client of the fake forge that posts as login.
func (f *Forge) As(login string) forge.Forge { return client{f, login} }

type client struct {
	*Forge
	login string
}

func (c client) PostComment(_ context.Context, ref forge.IssueRef, body string) (*forge.Comment, error) {
	return c.PostCommentAs(ref, body, c.login)
}

// EnsureLabel implements forge.Forge.
func (f *Forge) EnsureLabel(_ context.Context, owner, name, label, _, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.FailEnsureLabel; err != nil {
		f.FailEnsureLabel = nil
		return false, err
	}
	r := f.repos[owner+"/"+name]
	if r.labels[label] {
		return false, nil
	}
	r.labels[label] = true
	return true, nil
}

// CreateChangeRequest implements forge.Forge.
func (f *Forge) CreateChangeRequest(_ context.Context, cr forge.ChangeRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.PRs = append(f.PRs, cr)
	return fmt.Sprintf("https://forge.test/%s/%s/pull/%d", cr.Owner, cr.Repo, len(f.PRs)), nil
}

// OpenChangeRequest implements forge.Forge.
func (f *Forge) OpenChangeRequest(_ context.Context, owner, repo, head, base string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.PRs) - 1; i >= 0; i-- {
		cr := f.PRs[i]
		if !f.closed[i] && cr.Owner == owner && cr.Repo == repo && cr.Head == head && cr.Base == base {
			return fmt.Sprintf("https://forge.test/%s/%s/pull/%d", owner, repo, i+1), nil
		}
	}
	return "", forge.ErrNotFound
}

// ClosePR closes the i-th change request in PRs, counted from 0.
func (f *Forge) ClosePR(i int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed == nil {
		f.closed = map[int]bool{}
	}
	f.closed[i] = true
}

// RepoPrivate implements forge.Forge.
func (f *Forge) RepoPrivate(_ context.Context, owner, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.FailRepoPrivate; err != nil {
		f.FailRepoPrivate = nil
		return false, err
	}
	r, ok := f.repos[owner+"/"+name]
	if !ok {
		return false, forge.ErrNotFound
	}
	return r.private, nil
}

// User implements forge.Forge.
func (f *Forge) User(_ context.Context, login string) (*forge.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[login]
	if !ok {
		return nil, forge.ErrNotFound
	}
	return &u, nil
}

// SearchOpenIssuesWithLabel implements forge.Forge. A GitHub fake ignores the scope and searches every repository;
// a GitLab fake searches the one project the scope names, as GitLab does.
func (f *Forge) SearchOpenIssuesWithLabel(_ context.Context, scope string, labels ...string) ([]forge.IssueSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.FailSearch; err != nil {
		f.FailSearch = nil
		return nil, err
	}
	var out []forge.IssueSummary
	for name, r := range f.repos {
		if f.kind == forge.GitLab && name != scope {
			continue
		}
		for _, is := range r.issues {
			for _, label := range labels {
				if is.data.State == "open" && contains(is.data.Labels, label) {
					out = append(out, f.summary(is))
					break
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref.String() < out[j].Ref.String() })
	return out, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func remove(xs []string, x string) []string {
	var out []string
	for _, v := range xs {
		if v != x {
			out = append(out, v)
		}
	}
	return out
}

// Version is what a GitLab instance reports as its version.
func (f *Forge) Version(context.Context) (string, error) { return "fake", nil }

// ProjectID numbers the projects in the order of their names, as a stand-in for GitLab's project IDs.
func (f *Forge) ProjectID(_ context.Context, owner, name string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.repos[owner+"/"+name]; !ok {
		return 0, forge.ErrNotFound
	}
	var names []string
	for n := range f.repos {
		names = append(names, n)
	}
	sort.Strings(names)
	for i, n := range names {
		if n == owner+"/"+name {
			return int64(i + 1), nil
		}
	}
	return 0, forge.ErrNotFound
}

var _ forge.Forge = (*Forge)(nil)

// issueURL is the page of the issue on this fake forge, shaped as on the real one.
func (f *Forge) issueURL(ref forge.IssueRef) string {
	if f.kind == forge.GitLab {
		return fmt.Sprintf("https://gitlab.test/%s/%s/-/issues/%d", ref.Owner, ref.Repo, ref.Number)
	}
	return fmt.Sprintf("https://github.test/%s/%s/issues/%d", ref.Owner, ref.Repo, ref.Number)
}
