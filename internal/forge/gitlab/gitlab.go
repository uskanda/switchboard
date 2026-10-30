// Package gitlab implements forge.Forge over the GitLab REST API (v4), plus one GraphQL query for who last edited
// each note, which the REST API does not expose. Projects are named by their URL-escaped path, which GitLab accepts
// wherever it takes a project ID.
package gitlab

import (
	"bytes"
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
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// Client talks to one GitLab instance with one token.
type Client struct {
	token   string
	baseURL string // the instance, such as https://gitlab.com, without /api/v4
	http    *http.Client
}

// New returns a client for the GitLab at baseURL authenticated with token.
func New(token, baseURL string) *Client {
	return &Client{token: token, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 30 * time.Second}}
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
	if e.status == http.StatusNotFound {
		return forge.ErrNotFound
	}
	return nil
}

// do sends one request. in, when non-nil, is sent as JSON; out, when non-nil, receives the JSON response.
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
	req.Header.Set("PRIVATE-TOKEN", c.token)
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
		return nil, &apiError{method: method, path: req.URL.Path, status: resp.StatusCode, message: message(data)}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("gitlab: %s %s: decode: %w", method, req.URL.Path, err)
		}
	}
	return resp, nil
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
}

func (i glIssue) summary(owner, repo string) forge.IssueSummary {
	state := i.State
	if state == "opened" {
		state = "open"
	}
	return forge.IssueSummary{
		Ref:   forge.IssueRef{Forge: forge.GitLab, Owner: owner, Repo: repo, Number: i.IID},
		Title: i.Title, State: state, Labels: append([]string(nil), i.Labels...), UpdatedAt: i.UpdatedAt,
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

// maxNotePages bounds how many pages of notes Thread reads (100 notes each).
const maxNotePages = 10

// Thread implements forge.Forge. The body's last editor is the author of the last system note about the
// description (V21). When the notes run past maxNotePages, a later one may hide, so the editor is unknown.
func (c *Client) Thread(ctx context.Context, ref forge.IssueRef) (*forge.Issue, []forge.Comment, error) {
	base := project(ref.Owner, ref.Repo) + "/issues/" + strconv.Itoa(ref.Number)
	var gi glIssue
	if _, err := c.do(ctx, http.MethodGet, c.url(base, nil), nil, &gi); err != nil {
		return nil, nil, err
	}
	is := &forge.Issue{IssueSummary: gi.summary(ref.Owner, ref.Repo), Body: gi.Description, Author: gi.Author.user(), CreatedAt: gi.CreatedAt}
	var comments []forge.Comment
	var editor *forge.User
	more, err := pages(ctx, c, c.url(base+"/notes", url.Values{"sort": {"asc"}, "order_by": {"created_at"}, "per_page": {"100"}}), maxNotePages,
		func(p []glNote) {
			for _, n := range p {
				if n.System {
					if strings.Contains(n.Body, "description") {
						u := n.Author.user()
						editor = &u
					}
					continue
				}
				comments = append(comments, forge.Comment{ID: n.ID, Body: n.Body, Author: n.Author.user(), CreatedAt: n.CreatedAt})
			}
		})
	if err != nil {
		return nil, nil, err
	}
	switch {
	case more:
		is.BodyEdit = forge.Edit{Edited: true, Unknown: true}
	case editor != nil:
		is.BodyEdit = forge.Edit{Edited: true, Editor: editor}
	}
	if err := c.addEdits(ctx, ref, comments); err != nil {
		return nil, nil, err
	}
	return is, comments, nil
}

// editQuery asks for the newest notes, the ones a reply is in.
const editQuery = `query($path: ID!, $iid: String!) {
  project(fullPath: $path) {
    issue(iid: $iid) {
      notes(last: 100) { nodes { id lastEditedBy { id username } } }
    }
  }
}`

type gqlUser struct {
	ID       string `json:"id"` // gid://gitlab/User/<n>
	Username string `json:"username"`
}

// gid returns the number at the end of a GraphQL global ID such as gid://gitlab/Note/81.
func gid(s string) int64 {
	n, _ := strconv.ParseInt(s[strings.LastIndex(s, "/")+1:], 10, 64)
	return n
}

// addEdits fills in who last edited each comment. lastEditedBy is set only for an edited note (V21). A comment
// the query did not return is marked Unknown, so that trust decisions treat it as edited by an untrusted user.
func (c *Client) addEdits(ctx context.Context, ref forge.IssueRef, comments []forge.Comment) error {
	in := map[string]any{"query": editQuery, "variables": map[string]any{"path": ref.Project(), "iid": strconv.Itoa(ref.Number)}}
	var out struct {
		Data struct {
			Project struct {
				Issue struct {
					Notes struct {
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
		return err
	}
	if len(out.Errors) > 0 {
		return fmt.Errorf("gitlab: graphql: %s", out.Errors[0].Message)
	}
	byID := map[int64]forge.Edit{}
	for _, n := range out.Data.Project.Issue.Notes.Nodes {
		e := forge.Edit{}
		if n.LastEditedBy != nil {
			e = forge.Edit{Edited: true, Editor: &forge.User{Login: n.LastEditedBy.Username, ID: gid(n.LastEditedBy.ID)}}
			if e.Editor.ID == 0 {
				e = forge.Edit{Edited: true, Unknown: true}
			}
		}
		byID[gid(n.ID)] = e
	}
	for i := range comments {
		e, ok := byID[comments[i].ID]
		if !ok {
			e = forge.Edit{Edited: true, Unknown: true}
		}
		comments[i].Edit = e
	}
	return nil
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

type glProject struct {
	ID         int64  `json:"id"`
	Visibility string `json:"visibility"`
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

// SearchOpenIssuesWithLabel implements forge.Forge. On GitLab the scope is one project's path ("group/project"):
// the inbox reads the configured projects one by one.
func (c *Client) SearchOpenIssuesWithLabel(ctx context.Context, scope, label string) ([]forge.IssueSummary, error) {
	i := strings.LastIndex(scope, "/")
	if i < 0 {
		return nil, fmt.Errorf("gitlab: %q is not a project path", scope)
	}
	owner, repo := scope[:i], scope[i+1:]
	var out []forge.IssueSummary
	_, err := pages(ctx, c, c.url(project(owner, repo)+"/issues", url.Values{"labels": {label}, "state": {"opened"}, "per_page": {"100"}}), 0,
		func(p []glIssue) {
			for _, gi := range p {
				out = append(out, gi.summary(owner, repo))
			}
		})
	return out, err
}

var _ forge.Forge = (*Client)(nil)
