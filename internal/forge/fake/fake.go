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
	now      func() time.Time
	nextID   int64
	repos    map[string]*repo
	users    map[string]forge.User
	PRs      []forge.ChangeRequest
	Comments int // number of PostComment calls, for tests that count writes
	// PostAs is the login PostComment posts as; "agent" when empty.
	PostAs string
	// FailSetLabels, when set, is returned by the next SetLabels call and then cleared.
	FailSetLabels error
	failNth       int // FailNthSetLabels: calls left until the failing one
	failNthEr     error
	// AfterSetLabels, when set, runs after each successful SetLabels, with the lock released. Tests use it to
	// simulate a human changing labels right after the agent did.
	AfterSetLabels func(ref forge.IssueRef)
}

type repo struct {
	private bool
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
	return &Forge{now: now, nextID: 1000, repos: map[string]*repo{}, users: map[string]forge.User{}}
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

// AddIssue creates an open issue authored by author and returns its reference.
func (f *Forge) AddIssue(owner, name string, number int, title, body string, author forge.User) forge.IssueRef {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.repos[owner+"/"+name]
	ref := forge.IssueRef{Owner: owner, Repo: name, Number: number}
	now := f.now()
	r.issues[number] = &issue{
		data: forge.Issue{
			IssueSummary: forge.IssueSummary{Ref: ref, Title: title, State: "open", UpdatedAt: now},
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

// AddComment adds a comment as author and returns it.
func (f *Forge) AddComment(ref forge.IssueRef, body string, author forge.User) forge.Comment {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addComment(ref, body, author)
}

// EditComment replaces a comment body as editor.
func (f *Forge) EditComment(ref forge.IssueRef, id int64, body string, editor forge.User) {
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
	is.data.UpdatedAt = f.now()
	return c
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
			s := is.data.IssueSummary
			s.Labels = append([]string(nil), s.Labels...)
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref.Number < out[j].Ref.Number })
	return out, nil
}

// Thread implements forge.Forge.
func (f *Forge) Thread(_ context.Context, ref forge.IssueRef) (*forge.Issue, []forge.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

// SetLabels implements forge.Forge. Labels it sets are attributed to the zero user.
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
	defer f.mu.Unlock()
	f.Comments++
	as := f.PostAs
	if as == "" {
		as = "agent"
	}
	c := f.addComment(ref, body, f.users[as])
	return &c, nil
}

// EnsureLabel implements forge.Forge.
func (f *Forge) EnsureLabel(_ context.Context, owner, name, label, _, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

// RepoPrivate implements forge.Forge.
func (f *Forge) RepoPrivate(_ context.Context, owner, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

// SearchOpenIssuesWithLabel implements forge.Forge. The scope is ignored: every repository is searched.
func (f *Forge) SearchOpenIssuesWithLabel(_ context.Context, _ string, label string) ([]forge.IssueSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []forge.IssueSummary
	for _, r := range f.repos {
		for _, is := range r.issues {
			if is.data.State == "open" && contains(is.data.Labels, label) {
				out = append(out, is.data.IssueSummary)
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

var _ forge.Forge = (*Forge)(nil)
