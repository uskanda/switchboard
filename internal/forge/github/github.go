// Package github implements forge.Forge over the GitHub REST API, plus one GraphQL query for edit history, which
// the REST API does not expose.
package github

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
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// Client talks to one GitHub instance with one token.
type Client struct {
	token      string
	baseURL    string
	graphqlURL string
	http       *http.Client

	mu   sync.Mutex
	etag map[string]cached // conditional-request cache for UpdatedIssues, keyed by URL
}

type cached struct {
	etag  string
	items []forge.IssueSummary
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL points the REST API at base (tests, GitHub Enterprise).
func WithBaseURL(base string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(base, "/") } }

// WithGraphQLURL points the GraphQL API at u.
func WithGraphQLURL(u string) Option { return func(c *Client) { c.graphqlURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// New returns a client for github.com authenticated with token.
func New(token string, opts ...Option) *Client {
	c := &Client{
		token:      token,
		baseURL:    "https://api.github.com",
		graphqlURL: "https://api.github.com/graphql",
		http:       &http.Client{Timeout: 30 * time.Second},
		etag:       map[string]cached{},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// apiError is a non-2xx response. The message comes from GitHub's JSON body; the token is never part of it.
type apiError struct {
	method, path string
	status       int
	message      string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("github: %s %s: %d %s", e.method, e.path, e.status, e.message)
}

func (e *apiError) Unwrap() error {
	if e.status == http.StatusNotFound {
		return forge.ErrNotFound
	}
	return nil
}

// do sends one request. in, when non-nil, is sent as JSON; out, when non-nil, receives the JSON response.
func (c *Client) do(ctx context.Context, method, rawURL string, in, out any, header http.Header) (*http.Response, error) {
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
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "switchboard")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: %s %s: %w", method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotModified {
		return resp, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var m struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &m)
		return nil, &apiError{method: method, path: req.URL.Path, status: resp.StatusCode, message: m.Message}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("github: %s %s: decode: %w", method, req.URL.Path, err)
		}
	}
	return resp, nil
}

func (c *Client) url(path string, q url.Values) string {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

var nextLink = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// pages fetches every page of a list starting at first, decoding each into a fresh slice and passing it to each.
func pages[T any](ctx context.Context, c *Client, first string, each func([]T)) error {
	for u := first; u != ""; {
		var page []T
		resp, err := c.do(ctx, http.MethodGet, u, nil, &page, nil)
		if err != nil {
			return err
		}
		each(page)
		u = ""
		if m := nextLink.FindStringSubmatch(resp.Header.Get("Link")); m != nil {
			u = m[1]
		}
	}
	return nil
}

type ghUser struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

func (u ghUser) user() forge.User { return forge.User{Login: u.Login, ID: u.ID} }

type ghLabel struct {
	Name string `json:"name"`
}

type ghIssue struct {
	Number        int             `json:"number"`
	Title         string          `json:"title"`
	Body          string          `json:"body"`
	State         string          `json:"state"`
	User          ghUser          `json:"user"`
	Labels        []ghLabel       `json:"labels"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	PullRequest   json.RawMessage `json:"pull_request"`
	RepositoryURL string          `json:"repository_url"`
}

func (i ghIssue) summary(owner, repo string) forge.IssueSummary {
	s := forge.IssueSummary{
		Ref:   forge.IssueRef{Owner: owner, Repo: repo, Number: i.Number},
		Title: i.Title, State: i.State, UpdatedAt: i.UpdatedAt,
	}
	for _, l := range i.Labels {
		s.Labels = append(s.Labels, l.Name)
	}
	return s
}

// UpdatedIssues implements forge.Forge. While a repeated request returns 304, the cached list is returned, and
// GitHub does not count the request against the rate limit.
func (c *Client) UpdatedIssues(ctx context.Context, owner, repo string, since time.Time) ([]forge.IssueSummary, error) {
	first := c.url(fmt.Sprintf("/repos/%s/%s/issues", owner, repo), url.Values{
		"state": {"all"}, "since": {since.UTC().Format(time.RFC3339)}, "per_page": {"100"}, "sort": {"updated"},
	})
	c.mu.Lock()
	prev, ok := c.etag[first]
	c.mu.Unlock()
	h := http.Header{}
	if ok {
		h.Set("If-None-Match", prev.etag)
	}
	var page []ghIssue
	resp, err := c.do(ctx, http.MethodGet, first, nil, &page, h)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotModified {
		return append([]forge.IssueSummary(nil), prev.items...), nil
	}
	var out []forge.IssueSummary
	add := func(p []ghIssue) {
		for _, i := range p {
			if len(i.PullRequest) == 0 {
				out = append(out, i.summary(owner, repo))
			}
		}
	}
	add(page)
	if m := nextLink.FindStringSubmatch(resp.Header.Get("Link")); m != nil {
		if err := pages(ctx, c, m[1], add); err != nil {
			return nil, err
		}
	}
	if e := resp.Header.Get("ETag"); e != "" {
		c.mu.Lock()
		c.etag[first] = cached{etag: e, items: out}
		c.mu.Unlock()
	}
	return out, nil
}

type ghComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	User      ghUser    `json:"user"`
	CreatedAt time.Time `json:"created_at"`
}

// Thread implements forge.Forge.
func (c *Client) Thread(ctx context.Context, ref forge.IssueRef) (*forge.Issue, []forge.Comment, error) {
	var gi ghIssue
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/issues/%d", ref.Owner, ref.Repo, ref.Number), nil), nil, &gi, nil); err != nil {
		return nil, nil, err
	}
	is := &forge.Issue{IssueSummary: gi.summary(ref.Owner, ref.Repo), Body: gi.Body, Author: gi.User.user(), CreatedAt: gi.CreatedAt}
	var comments []forge.Comment
	err := pages(ctx, c, c.url(fmt.Sprintf("/repos/%s/%s/issues/%d/comments", ref.Owner, ref.Repo, ref.Number), url.Values{"per_page": {"100"}}),
		func(p []ghComment) {
			for _, gc := range p {
				comments = append(comments, forge.Comment{ID: gc.ID, Body: gc.Body, Author: gc.User.user(), CreatedAt: gc.CreatedAt})
			}
		})
	if err != nil {
		return nil, nil, err
	}
	if err := c.addEdits(ctx, ref, is, comments); err != nil {
		return nil, nil, err
	}
	return is, comments, nil
}

const editQuery = `query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      lastEditedAt
      editor { login ... on User { databaseId } ... on Bot { databaseId } }
      comments(first: 100) {
        pageInfo { hasNextPage }
        nodes { databaseId lastEditedAt editor { login ... on User { databaseId } ... on Bot { databaseId } } }
      }
    }
  }
}`

type gqlActor struct {
	Login      string `json:"login"`
	DatabaseID int64  `json:"databaseId"`
}

type gqlEdited struct {
	DatabaseID   int64     `json:"databaseId"`
	LastEditedAt *string   `json:"lastEditedAt"`
	Editor       *gqlActor `json:"editor"`
}

func (e gqlEdited) edit() forge.Edit {
	if e.LastEditedAt == nil {
		return forge.Edit{}
	}
	if e.Editor == nil || e.Editor.DatabaseID == 0 {
		return forge.Edit{Edited: true, Unknown: true}
	}
	return forge.Edit{Edited: true, Editor: &forge.User{Login: e.Editor.Login, ID: e.Editor.DatabaseID}}
}

// addEdits fills in who last edited the body and each comment. A comment the query did not return (beyond its
// first page) is marked Unknown, so that trust decisions treat it as edited by an untrusted user.
func (c *Client) addEdits(ctx context.Context, ref forge.IssueRef, is *forge.Issue, comments []forge.Comment) error {
	in := map[string]any{"query": editQuery, "variables": map[string]any{"owner": ref.Owner, "repo": ref.Repo, "number": ref.Number}}
	var out struct {
		Data struct {
			Repository struct {
				Issue struct {
					gqlEdited
					Comments struct {
						Nodes []gqlEdited `json:"nodes"`
					} `json:"comments"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if _, err := c.do(ctx, http.MethodPost, c.graphqlURL, in, &out, nil); err != nil {
		return err
	}
	if len(out.Errors) > 0 {
		return fmt.Errorf("github: graphql: %s", out.Errors[0].Message)
	}
	gi := out.Data.Repository.Issue
	is.BodyEdit = gi.edit()
	byID := map[int64]forge.Edit{}
	for _, n := range gi.Comments.Nodes {
		byID[n.DatabaseID] = n.edit()
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

type ghEvent struct {
	Event string  `json:"event"`
	Label ghLabel `json:"label"`
	Actor ghUser  `json:"actor"`
}

// LabelActor implements forge.Forge.
func (c *Client) LabelActor(ctx context.Context, ref forge.IssueRef, label string) (*forge.User, error) {
	var last *forge.User
	err := pages(ctx, c, c.url(fmt.Sprintf("/repos/%s/%s/issues/%d/events", ref.Owner, ref.Repo, ref.Number), url.Values{"per_page": {"100"}}),
		func(p []ghEvent) {
			for _, e := range p {
				if e.Event == "labeled" && e.Label.Name == label {
					u := e.Actor.user()
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

// SetLabels implements forge.Forge.
func (c *Client) SetLabels(ctx context.Context, ref forge.IssueRef, labels []string) error {
	if labels == nil {
		labels = []string{}
	}
	_, err := c.do(ctx, http.MethodPut, c.url(fmt.Sprintf("/repos/%s/%s/issues/%d/labels", ref.Owner, ref.Repo, ref.Number), nil),
		map[string]any{"labels": labels}, nil, nil)
	return err
}

// PostComment implements forge.Forge.
func (c *Client) PostComment(ctx context.Context, ref forge.IssueRef, body string) (*forge.Comment, error) {
	var gc ghComment
	if _, err := c.do(ctx, http.MethodPost, c.url(fmt.Sprintf("/repos/%s/%s/issues/%d/comments", ref.Owner, ref.Repo, ref.Number), nil),
		map[string]any{"body": body}, &gc, nil); err != nil {
		return nil, err
	}
	return &forge.Comment{ID: gc.ID, Body: gc.Body, Author: gc.User.user(), CreatedAt: gc.CreatedAt}, nil
}

// EnsureLabel implements forge.Forge.
func (c *Client) EnsureLabel(ctx context.Context, owner, repo, name, color, description string) (bool, error) {
	_, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/labels/%s", owner, repo, url.PathEscape(name)), nil), nil, nil, nil)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, forge.ErrNotFound) {
		return false, err
	}
	_, err = c.do(ctx, http.MethodPost, c.url(fmt.Sprintf("/repos/%s/%s/labels", owner, repo), nil),
		map[string]any{"name": name, "color": color, "description": description}, nil, nil)
	return err == nil, err
}

// CreateChangeRequest implements forge.Forge.
func (c *Client) CreateChangeRequest(ctx context.Context, cr forge.ChangeRequest) (string, error) {
	var out struct {
		HTMLURL string `json:"html_url"`
	}
	_, err := c.do(ctx, http.MethodPost, c.url(fmt.Sprintf("/repos/%s/%s/pulls", cr.Owner, cr.Repo), nil),
		map[string]any{"title": cr.Title, "body": cr.Body, "head": cr.Head, "base": cr.Base, "draft": cr.Draft}, &out, nil)
	return out.HTMLURL, err
}

// RepoPrivate implements forge.Forge.
func (c *Client) RepoPrivate(ctx context.Context, owner, repo string) (bool, error) {
	var out struct {
		Private bool `json:"private"`
	}
	_, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s", owner, repo), nil), nil, &out, nil)
	return out.Private, err
}

// User implements forge.Forge.
func (c *Client) User(ctx context.Context, login string) (*forge.User, error) {
	var u ghUser
	if _, err := c.do(ctx, http.MethodGet, c.url("/users/"+url.PathEscape(login), nil), nil, &u, nil); err != nil {
		return nil, err
	}
	out := u.user()
	return &out, nil
}

var repoFromURL = regexp.MustCompile(`/repos/([^/]+)/([^/]+)$`)

// SearchOpenIssuesWithLabel implements forge.Forge. It reads the first 100 results, which covers any realistic
// number of waiting issues.
func (c *Client) SearchOpenIssuesWithLabel(ctx context.Context, scope, label string) ([]forge.IssueSummary, error) {
	q := fmt.Sprintf(`is:issue is:open label:%s %s`, strconv.Quote(label), scope)
	var out struct {
		Items []ghIssue `json:"items"`
	}
	if _, err := c.do(ctx, http.MethodGet, c.url("/search/issues", url.Values{"q": {strings.TrimSpace(q)}, "per_page": {"100"}}), nil, &out, nil); err != nil {
		return nil, err
	}
	var res []forge.IssueSummary
	for _, i := range out.Items {
		m := repoFromURL.FindStringSubmatch(i.RepositoryURL)
		if m == nil {
			continue
		}
		res = append(res, i.summary(m[1], m[2]))
	}
	return res, nil
}

var _ forge.Forge = (*Client)(nil)
