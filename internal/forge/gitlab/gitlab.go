// Package gitlab implements forge.Forge over the GitLab REST API (v4), plus one GraphQL query for who last edited
// each note, which the REST API does not expose. Projects are named by their URL-escaped path, which GitLab accepts
// wherever it takes a project ID.
package gitlab

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// Client talks to one GitLab instance with one token.
type Client struct {
	token   string
	bearer  bool   // the token is an OAuth access token, sent as a bearer token
	baseURL string // the instance, such as https://gitlab.com, without /api/v4
	http    *http.Client
}

// New returns a client for the GitLab at baseURL authenticated with token, a personal, project or group access token.
func New(token, baseURL string) *Client {
	return &Client{token: token, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 30 * time.Second}}
}

// NewOAuth returns a client authenticated with an OAuth access token: the token of a person logged in to the web
// inbox, which GitLab takes as a bearer token only (issue #146).
func NewOAuth(token, baseURL string) *Client {
	c := New(token, baseURL)
	c.bearer = true
	return c
}

// authorize puts the client's token on req.
func (c *Client) authorize(h http.Header) {
	if c.bearer {
		h.Set("Authorization", "Bearer "+c.token)
		return
	}
	h.Set("PRIVATE-TOKEN", c.token)
}

// apiError is a non-2xx response. The message comes from GitLab's JSON body; the token is never part of it.
type apiError struct {
	method, path string
	status       int
	message      string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("gitlab: %s %s: %d %s", e.method, e.path, e.status, e.message)
}

func (e *apiError) Unwrap() error {
	switch e.status {
	case http.StatusNotFound:
		return forge.ErrNotFound
	case http.StatusForbidden:
		return forge.ErrForbidden
	}
	return nil
}

// do sends one request. in, when non-nil, is sent as JSON; out, when non-nil, receives the JSON response, or the
// raw body when it is a *[]byte.
func (c *Client) do(ctx context.Context, method, rawURL string, in, out any) (*http.Response, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	c.authorize(req.Header)
	req.Header.Set("User-Agent", "switchboard")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: %s %s: %w", method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var err error = &apiError{method: method, path: req.URL.Path, status: resp.StatusCode, message: message(data)}
		if resp.StatusCode == http.StatusTooManyRequests {
			err = &forge.RateLimitError{Reset: rateLimitReset(resp.Header, time.Now()), Err: err}
		}
		return nil, err
	}
	if raw, ok := out.(*[]byte); ok {
		*raw = data
		return resp, nil
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("gitlab: %s %s: decode: %w", method, req.URL.Path, err)
		}
	}
	return resp, nil
}

// rateLimitReset is when a 429 says the limit resets (V61): RateLimit-Reset (Unix seconds), else Retry-After
// (seconds), else a minute from now.
func rateLimitReset(h http.Header, now time.Time) time.Time {
	if n, err := strconv.ParseInt(h.Get("RateLimit-Reset"), 10, 64); err == nil {
		return time.Unix(n, 0)
	}
	if n, err := strconv.Atoi(h.Get("Retry-After")); err == nil {
		return now.Add(time.Duration(n) * time.Second)
	}
	return now.Add(time.Minute)
}

// message reads GitLab's error body, whose "message" is a string, a list or a map of field errors, or whose
// "error" holds the text.
func message(data []byte) string {
	var m struct {
		Message json.RawMessage `json:"message"`
		Error   string          `json:"error"`
	}
	_ = json.Unmarshal(data, &m)
	var s string
	if json.Unmarshal(m.Message, &s) == nil {
		return s
	}
	if len(m.Message) > 0 {
		return string(m.Message)
	}
	return m.Error
}

// project is the API path of a project: /projects/<escaped path>.
func project(owner, repo string) string { return "/projects/" + url.PathEscape(owner+"/"+repo) }

func (c *Client) url(path string, q url.Values) string {
	u := c.baseURL + "/api/v4" + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

var nextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// pages fetches up to max pages of a list starting at first, passing each to each. It reports whether pages remained.
func pages[T any](ctx context.Context, c *Client, first string, max int, each func([]T)) (bool, error) {
	u := first
	for n := 0; u != ""; n++ {
		if max > 0 && n == max {
			return true, nil
		}
		var page []T
		resp, err := c.do(ctx, http.MethodGet, u, nil, &page)
		if err != nil {
			return false, err
		}
		each(page)
		u = ""
		if m := nextLink.FindStringSubmatch(resp.Header.Get("Link")); m != nil {
			u = m[1]
		}
	}
	return false, nil
}

type glUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func (u glUser) user() forge.User { return forge.User{Login: u.Username, ID: u.ID} }

type glIssue struct {
	IID            int       `json:"iid"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	State          string    `json:"state"` // "opened" or "closed"
	Author         glUser    `json:"author"`
	Labels         []string  `json:"labels"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	UserNotesCount int       `json:"user_notes_count"`
	WebURL         string    `json:"web_url"`
}

func (i glIssue) summary(owner, repo string) forge.IssueSummary {
	state := i.State
	if state == "opened" {
		state = "open"
	}
	return forge.IssueSummary{
		Ref:   forge.IssueRef{Forge: forge.GitLab, Owner: owner, Repo: repo, Number: i.IID},
		Title: i.Title, State: state, URL: i.WebURL, Labels: append([]string(nil), i.Labels...), UpdatedAt: i.UpdatedAt,
		Notes: i.UserNotesCount,
	}
}

// UpdatedIssues implements forge.Forge.
func (c *Client) UpdatedIssues(ctx context.Context, owner, repo string, since time.Time) ([]forge.IssueSummary, error) {
	q := url.Values{"state": {"all"}, "order_by": {"updated_at"}, "sort": {"desc"}, "per_page": {"100"}}
	if !since.IsZero() {
		q.Set("updated_after", since.UTC().Format(time.RFC3339))
	}
	var out []forge.IssueSummary
	_, err := pages(ctx, c, c.url(project(owner, repo)+"/issues", q), 0, func(p []glIssue) {
		for _, i := range p {
			out = append(out, i.summary(owner, repo))
		}
	})
	return out, err
}

type glNote struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	System    bool      `json:"system"`
	Author    glUser    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

// The bodies of the system notes GitLab writes when the description or the title changes. A title note goes on with
// the old and the new title ("changed title from **a** to **b**"), so it is matched by its start, and a title that
// holds "description" cannot pass for a description note.
const (
	descriptionNote = "changed the description"
	titleNotePrefix = "changed title "
)

// Thread implements forge.Forge, reading every page of the notes. The body's last editor is the author of the last
// system note about the description (V21); the title's, of the last one about the title.
// The issue, its notes and who edited them are three requests that need nothing of one another, so they are sent
// at once.
func (c *Client) Thread(ctx context.Context, ref forge.IssueRef) (*forge.Issue, []forge.Comment, error) {
	base := project(ref.Owner, ref.Repo) + "/issues/" + strconv.Itoa(ref.Number)
	var (
		wg                          sync.WaitGroup
		gi                          glIssue
		comments                    []forge.Comment
		bodyEditor, titleEditor     *forge.User
		byID                        map[int64]forge.Edit
		issueErr, notesErr, editErr error
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		_, issueErr = c.do(ctx, http.MethodGet, c.url(base, nil), nil, &gi)
	}()
	go func() {
		defer wg.Done()
		_, notesErr = pages(ctx, c, c.url(base+"/notes", url.Values{"sort": {"asc"}, "order_by": {"created_at"}, "per_page": {"100"}}), 0,
			func(p []glNote) {
				for _, n := range p {
					if n.System {
						u := n.Author.user()
						switch {
						case strings.TrimSpace(n.Body) == descriptionNote:
							bodyEditor = &u
						case strings.HasPrefix(n.Body, titleNotePrefix):
							titleEditor = &u
						}
						continue
					}
					comments = append(comments, forge.Comment{ID: n.ID, Body: n.Body, Author: n.Author.user(), CreatedAt: n.CreatedAt})
				}
			})
	}()
	go func() {
		defer wg.Done()
		byID, editErr = c.edits(ctx, ref)
	}()
	wg.Wait()
	if err := cmp.Or(issueErr, notesErr, editErr); err != nil {
		return nil, nil, err
	}
	is := &forge.Issue{IssueSummary: gi.summary(ref.Owner, ref.Repo), Body: gi.Description, Author: gi.Author.user(), CreatedAt: gi.CreatedAt}
	if bodyEditor != nil {
		is.BodyEdit = forge.Edit{Edited: true, Editor: bodyEditor}
	}
	if titleEditor != nil {
		is.TitleEdit = forge.Edit{Edited: true, Editor: titleEditor}
	}
	// A comment the query did not return is marked Unknown, so that trust decisions treat it as edited by an
	// untrusted user.
	for i := range comments {
		e, ok := byID[comments[i].ID]
		if !ok {
			e = forge.Edit{Edited: true, Unknown: true}
		}
		comments[i].Edit = e
	}
	return is, comments, nil
}

// Comments implements forge.Forge: the notes of the issue, oldest first, without the system notes.
func (c *Client) Comments(ctx context.Context, ref forge.IssueRef) ([]forge.Comment, error) {
	base := project(ref.Owner, ref.Repo) + "/issues/" + strconv.Itoa(ref.Number)
	var out []forge.Comment
	_, err := pages(ctx, c, c.url(base+"/notes", url.Values{"sort": {"asc"}, "order_by": {"created_at"}, "per_page": {"100"}}), 0,
		func(p []glNote) {
			for _, n := range p {
				if !n.System {
					out = append(out, forge.Comment{ID: n.ID, Body: n.Body, Author: n.Author.user(), CreatedAt: n.CreatedAt})
				}
			}
		})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// editQuery asks for a page of the notes, newest first: $before is the cursor of the page after, null for the newest.
const editQuery = `query($path: ID!, $iid: String!, $before: String) {
  project(fullPath: $path) {
    issue(iid: $iid) {
      notes(last: 100, before: $before) {
        pageInfo { hasPreviousPage startCursor }
        nodes { id lastEditedBy { id username } }
      }
    }
  }
}`

// maxEditPages bounds how many pages of notes the edit query reads (10,000 notes). A note beyond them is marked
// Unknown.
const maxEditPages = 100

type gqlUser struct {
	ID       string `json:"id"` // gid://gitlab/User/<n>
	Username string `json:"username"`
}

// gid returns the number at the end of a GraphQL global ID such as gid://gitlab/Note/81.
func gid(s string) int64 {
	n, _ := strconv.ParseInt(s[strings.LastIndex(s, "/")+1:], 10, 64)
	return n
}

// edits reads who last edited each note, by note ID, from the newest page back to the oldest. lastEditedBy is set
// only for an edited note (V21).
func (c *Client) edits(ctx context.Context, ref forge.IssueRef) (map[int64]forge.Edit, error) {
	byID := map[int64]forge.Edit{}
	var before any
	for page := 0; page < maxEditPages; page++ {
		in := map[string]any{"query": editQuery, "variables": map[string]any{"path": ref.Project(), "iid": strconv.Itoa(ref.Number), "before": before}}
		var out struct {
			Data struct {
				Project struct {
					Issue struct {
						Notes struct {
							PageInfo struct {
								HasPreviousPage bool   `json:"hasPreviousPage"`
								StartCursor     string `json:"startCursor"`
							} `json:"pageInfo"`
							Nodes []struct {
								ID           string   `json:"id"`
								LastEditedBy *gqlUser `json:"lastEditedBy"`
							} `json:"nodes"`
						} `json:"notes"`
					} `json:"issue"`
				} `json:"project"`
			} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if _, err := c.do(ctx, http.MethodPost, c.baseURL+"/api/graphql", in, &out); err != nil {
			return nil, err
		}
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("gitlab: graphql: %s", out.Errors[0].Message)
		}
		notes := out.Data.Project.Issue.Notes
		for _, n := range notes.Nodes {
			e := forge.Edit{}
			if n.LastEditedBy != nil {
				e = forge.Edit{Edited: true, Editor: &forge.User{Login: n.LastEditedBy.Username, ID: gid(n.LastEditedBy.ID)}}
				if e.Editor.ID == 0 {
					e = forge.Edit{Edited: true, Unknown: true}
				}
			}
			byID[gid(n.ID)] = e
		}
		if !notes.PageInfo.HasPreviousPage || notes.PageInfo.StartCursor == "" {
			break
		}
		before = notes.PageInfo.StartCursor
	}
	return byID, nil
}

type glLabelEvent struct {
	Action string `json:"action"`
	Label  *struct {
		Name string `json:"name"`
	} `json:"label"`
	User glUser `json:"user"`
}

// LabelActor implements forge.Forge.
func (c *Client) LabelActor(ctx context.Context, ref forge.IssueRef, label string) (*forge.User, error) {
	var last *forge.User
	path := project(ref.Owner, ref.Repo) + "/issues/" + strconv.Itoa(ref.Number) + "/resource_label_events"
	_, err := pages(ctx, c, c.url(path, url.Values{"per_page": {"100"}}), 0, func(p []glLabelEvent) {
		for _, e := range p {
			if e.Action == "add" && e.Label != nil && e.Label.Name == label {
				u := e.User.user()
				last = &u
			}
		}
	})
	if err != nil {
		return nil, err
	}
	if last == nil {
		return nil, forge.ErrNotFound
	}
	return last, nil
}

// SetLabels implements forge.Forge. GitLab replaces every label with the comma-separated list.
func (c *Client) SetLabels(ctx context.Context, ref forge.IssueRef, labels []string) error {
	_, err := c.do(ctx, http.MethodPut, c.url(project(ref.Owner, ref.Repo)+"/issues/"+strconv.Itoa(ref.Number), nil),
		map[string]any{"labels": strings.Join(labels, ",")}, nil)
	return err
}

// PostComment implements forge.Forge.
func (c *Client) PostComment(ctx context.Context, ref forge.IssueRef, body string) (*forge.Comment, error) {
	var n glNote
	if _, err := c.do(ctx, http.MethodPost, c.url(project(ref.Owner, ref.Repo)+"/issues/"+strconv.Itoa(ref.Number)+"/notes", nil),
		map[string]any{"body": body}, &n); err != nil {
		return nil, err
	}
	return &forge.Comment{ID: n.ID, Body: n.Body, Author: n.Author.user(), CreatedAt: n.CreatedAt}, nil
}

// EditComment implements forge.Forge.
func (c *Client) EditComment(ctx context.Context, ref forge.IssueRef, id int64, body string) error {
	_, err := c.do(ctx, http.MethodPut, c.url(project(ref.Owner, ref.Repo)+"/issues/"+strconv.Itoa(ref.Number)+"/notes/"+strconv.FormatInt(id, 10), nil),
		map[string]any{"body": body}, nil)
	return err
}

// DeleteComment implements forge.Forge.
func (c *Client) DeleteComment(ctx context.Context, ref forge.IssueRef, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, c.url(project(ref.Owner, ref.Repo)+"/issues/"+strconv.Itoa(ref.Number)+"/notes/"+strconv.FormatInt(id, 10), nil), nil, nil)
	return err
}

// EnsureLabel implements forge.Forge. color is a hex color without the leading "#", as on GitHub.
func (c *Client) EnsureLabel(ctx context.Context, owner, repo, name, color, description string) (bool, error) {
	_, err := c.do(ctx, http.MethodGet, c.url(project(owner, repo)+"/labels/"+url.PathEscape(name), nil), nil, nil)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, forge.ErrNotFound) {
		return false, err
	}
	_, err = c.do(ctx, http.MethodPost, c.url(project(owner, repo)+"/labels", nil),
		map[string]any{"name": name, "color": "#" + strings.TrimPrefix(color, "#"), "description": description}, nil)
	return err == nil, err
}

// CreateChangeRequest implements forge.Forge with a merge request. A draft one is titled "Draft: ..." (V23).
func (c *Client) CreateChangeRequest(ctx context.Context, cr forge.ChangeRequest) (string, error) {
	title := cr.Title
	if cr.Draft {
		title = "Draft: " + title
	}
	var out struct {
		WebURL string `json:"web_url"`
	}
	_, err := c.do(ctx, http.MethodPost, c.url(project(cr.Owner, cr.Repo)+"/merge_requests", nil),
		map[string]any{"source_branch": cr.Head, "target_branch": cr.Base, "title": title, "description": cr.Body}, &out)
	return out.WebURL, err
}

// OpenChangeRequest implements forge.Forge with the open merge requests from head to base.
func (c *Client) OpenChangeRequest(ctx context.Context, owner, repo, head, base string) (string, error) {
	var out []struct {
		WebURL string `json:"web_url"`
	}
	q := url.Values{"state": {"opened"}, "source_branch": {head}, "target_branch": {base}}
	if _, err := c.do(ctx, http.MethodGet, c.url(project(owner, repo)+"/merge_requests", q), nil, &out); err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "", forge.ErrNotFound
	}
	return out[0].WebURL, nil
}

type glProject struct {
	ID            int64  `json:"id"`
	Visibility    string `json:"visibility"`
	DefaultBranch string `json:"default_branch"`
	Namespace     struct {
		AvatarURL string `json:"avatar_url"`
	} `json:"namespace"`
}

func (c *Client) project(ctx context.Context, owner, repo string) (*glProject, error) {
	var p glProject
	if _, err := c.do(ctx, http.MethodGet, c.url(project(owner, repo), nil), nil, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// RepoPrivate implements forge.Forge. Only a private project is private: an internal one is readable by every user
// of the instance.
func (c *Client) RepoPrivate(ctx context.Context, owner, repo string) (bool, error) {
	p, err := c.project(ctx, owner, repo)
	if err != nil {
		return false, err
	}
	return p.Visibility == "private", nil
}

// ProjectID resolves the project's path to its numeric ID.
func (c *Client) ProjectID(ctx context.Context, owner, repo string) (int64, error) {
	p, err := c.project(ctx, owner, repo)
	if err != nil {
		return 0, err
	}
	return p.ID, nil
}

// User implements forge.Forge.
func (c *Client) User(ctx context.Context, login string) (*forge.User, error) {
	var us []glUser
	if _, err := c.do(ctx, http.MethodGet, c.url("/users", url.Values{"username": {login}}), nil, &us); err != nil {
		return nil, err
	}
	for _, u := range us {
		if strings.EqualFold(u.Username, login) {
			out := u.user()
			return &out, nil
		}
	}
	return nil, fmt.Errorf("gitlab: user %s: %w", login, forge.ErrNotFound)
}

// Version returns the version of the GitLab instance.
func (c *Client) Version(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"version"`
	}
	_, err := c.do(ctx, http.MethodGet, c.url("/version", nil), nil, &v)
	return v.Version, err
}

// SearchOpenIssuesWithLabel implements forge.Forge. GitLab has no search over a set of projects, so the projects of
// the scope are read one by one; its query is ignored.
func (c *Client) SearchOpenIssuesWithLabel(ctx context.Context, scope forge.Scope, labels ...string) ([]forge.IssueSummary, error) {
	var out []forge.IssueSummary
	for _, path := range scope.Projects {
		found, err := c.searchProject(ctx, path, labels)
		if err != nil {
			if len(scope.Projects) > 1 {
				err = fmt.Errorf("%s: %w", path, err)
			}
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}

// searchProject lists the open issues of the project at path ("group/project") that carry any of labels. GitLab's
// labels parameter matches the issues that carry all of them, so each label is asked for on its own.
func (c *Client) searchProject(ctx context.Context, path string, labels []string) ([]forge.IssueSummary, error) {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return nil, fmt.Errorf("gitlab: %q is not a project path", path)
	}
	owner, repo := path[:i], path[i+1:]
	var out []forge.IssueSummary
	seen := map[int]bool{}
	for _, label := range labels {
		_, err := pages(ctx, c, c.url(project(owner, repo)+"/issues", url.Values{"labels": {label}, "state": {"opened"}, "per_page": {"100"}}), 0,
			func(p []glIssue) {
				for _, gi := range p {
					if s := gi.summary(owner, repo); !seen[s.Ref.Number] {
						seen[s.Ref.Number] = true
						out = append(out, s)
					}
				}
			})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

var _ forge.Forge = (*Client)(nil)
