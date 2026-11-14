// Package github implements forge.Forge over the GitHub REST API, plus one GraphQL query for edit history, which
// the REST API does not expose.
package github

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
	"slices"
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
	gh         string // the gh binary AttachComment runs; "gh" when empty

	mu   sync.Mutex
	etag map[string]cached // conditional-request cache for UpdatedIssues, keyed by URL
	// commentTags is the conditional-request cache for Comments, keyed by the URL of the first page.
	commentTags map[string]cachedComments
	now         func() time.Time // time.Now when nil; for the tests of rate limits
}

type cached struct {
	etag  string
	items []forge.IssueSummary
}

type cachedComments struct {
	etag     string
	comments []forge.Comment
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL points the REST API at base (tests, GitHub Enterprise).
func WithBaseURL(base string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(base, "/") }
}

// WithGraphQLURL points the GraphQL API at u.
func WithGraphQLURL(u string) Option { return func(c *Client) { c.graphqlURL = u } }

// WithHTTPClient replaces the HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// New returns a client for github.com authenticated with token.
func New(token string, opts ...Option) *Client {
	c := &Client{
		token:       token,
		baseURL:     "https://api.github.com",
		graphqlURL:  "https://api.github.com/graphql",
		http:        &http.Client{Timeout: 30 * time.Second},
		etag:        map[string]cached{},
		commentTags: map[string]cachedComments{},
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
	details      string
}

func (e *apiError) Error() string {
	s := fmt.Sprintf("github: %s %s: %d %s", e.method, e.path, e.status, e.message)
	if e.details != "" {
		s += ": " + e.details
	}
	return s
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
			Errors  []struct {
				Code    string `json:"code"`
				Field   string `json:"field"`
				Message string `json:"message"`
			} `json:"errors"`
		}
		_ = json.Unmarshal(data, &m)
		// The reason of a 422 is in errors, not in message ("Validation Failed").
		var details []string
		for _, e := range m.Errors {
			if e.Message != "" {
				details = append(details, e.Message)
			} else {
				details = append(details, strings.TrimSpace(e.Code+" "+e.Field))
			}
		}
		var err error = &apiError{method: method, path: req.URL.Path, status: resp.StatusCode, message: m.Message, details: strings.Join(details, "; ")}
		if reset, ok := c.rateLimitReset(resp); ok {
			err = &forge.RateLimitError{Reset: reset, Err: err}
		}
		return nil, err
	}
	if raw, ok := out.(*[]byte); ok {
		*raw = data
		return resp, nil
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("github: %s %s: decode: %w", method, req.URL.Path, err)
		}
	}
	return resp, nil
}

// noResetWait is how long a rate limit is assumed to last when the answer does not say.
const noResetWait = time.Minute

// rateLimitReset tells whether resp refuses for a rate limit, and when the limit resets (V61). GitHub answers 403
// or 429: a primary limit with x-ratelimit-remaining 0 and x-ratelimit-reset (Unix seconds), a secondary limit with
// retry-after (seconds), or neither, when noResetWait is assumed. A 403 with neither is no rate limit.
func (c *Client) rateLimitReset(resp *http.Response) (time.Time, bool) {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return time.Time{}, false
	}
	now := time.Now()
	if c.now != nil {
		now = c.now()
	}
	if s := resp.Header.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return now.Add(time.Duration(n) * time.Second), true
		}
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		if n, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			return time.Unix(n, 0), true
		}
		return now.Add(noResetWait), true
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return now.Add(noResetWait), true
	}
	return time.Time{}, false
}

// TokenOwner returns the login of whoever the client's token belongs to.
func (c *Client) TokenOwner(ctx context.Context) (string, error) {
	var u struct {
		Login string `json:"login"`
	}
	_, err := c.do(ctx, http.MethodGet, c.url("/user", nil), nil, &u, nil)
	return u.Login, err
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
	Comments      int             `json:"comments"`
	PullRequest   json.RawMessage `json:"pull_request"`
	RepositoryURL string          `json:"repository_url"`
	HTMLURL       string          `json:"html_url"`
}

func (i ghIssue) summary(owner, repo string) forge.IssueSummary {
	s := forge.IssueSummary{
		Ref:   forge.IssueRef{Forge: forge.GitHub, Owner: owner, Repo: repo, Number: i.Number},
		Title: i.Title, State: i.State, URL: i.HTMLURL, UpdatedAt: i.UpdatedAt, Notes: i.Comments,
	}
	for _, l := range i.Labels {
		s.Labels = append(s.Labels, l.Name)
	}
	return s
}

// UpdatedIssues implements forge.Forge. While a repeated request returns 304, the cached list is returned, and
// GitHub does not count the request against the rate limit.
func (c *Client) UpdatedIssues(ctx context.Context, owner, repo string, since time.Time) ([]forge.IssueSummary, error) {
	q := url.Values{"state": {"all"}, "per_page": {"100"}, "sort": {"updated"}}
	if !since.IsZero() {
		// GitHub refuses the zero time with 422, so the first poll asks for every issue instead.
		q.Set("since", since.UTC().Format(time.RFC3339))
	}
	first := c.url(fmt.Sprintf("/repos/%s/%s/issues", owner, repo), q)
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

// Comments implements forge.Forge. The first page is asked for with the ETag of the last answer; while it comes
// back 304, the comments of that answer are returned, and GitHub does not count the request against the rate limit
// (V60). Only when the first page changed are the pages after it read.
func (c *Client) Comments(ctx context.Context, ref forge.IssueRef) ([]forge.Comment, error) {
	first := c.url(fmt.Sprintf("/repos/%s/%s/issues/%d/comments", ref.Owner, ref.Repo, ref.Number), url.Values{"per_page": {"100"}})
	c.mu.Lock()
	prev, ok := c.commentTags[first]
	c.mu.Unlock()
	h := http.Header{}
	if ok {
		h.Set("If-None-Match", prev.etag)
	}
	var page []ghComment
	resp, err := c.do(ctx, http.MethodGet, first, nil, &page, h)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotModified {
		return slices.Clone(prev.comments), nil
	}
	var out []forge.Comment
	add := func(p []ghComment) {
		for _, gc := range p {
			out = append(out, gc.comment())
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
		c.commentTags[first] = cachedComments{etag: e, comments: out}
		c.mu.Unlock()
	}
	return slices.Clone(out), nil
}

func (gc ghComment) comment() forge.Comment {
	return forge.Comment{ID: gc.ID, Body: gc.Body, Author: gc.User.user(), CreatedAt: gc.CreatedAt}
}

// Thread implements forge.Forge.
// The issue, its comments and who edited them are three requests that need nothing of one another, so they are sent
// at once.
func (c *Client) Thread(ctx context.Context, ref forge.IssueRef) (*forge.Issue, []forge.Comment, error) {
	var (
		wg                         sync.WaitGroup
		gi                         ghIssue
		comments                   []forge.Comment
		ed                         *edits
		issueErr, commentErr, eErr error
	)
	wg.Add(3)
	go func() {
		defer wg.Done()
		_, issueErr = c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/issues/%d", ref.Owner, ref.Repo, ref.Number), nil), nil, &gi, nil)
	}()
	go func() {
		defer wg.Done()
		commentErr = pages(ctx, c, c.url(fmt.Sprintf("/repos/%s/%s/issues/%d/comments", ref.Owner, ref.Repo, ref.Number), url.Values{"per_page": {"100"}}),
			func(p []ghComment) {
				for _, gc := range p {
					comments = append(comments, gc.comment())
				}
			})
	}()
	go func() {
		defer wg.Done()
		ed, eErr = c.edits(ctx, ref)
	}()
	wg.Wait()
	if err := cmp.Or(issueErr, commentErr, eErr); err != nil {
		return nil, nil, err
	}
	is := &forge.Issue{IssueSummary: gi.summary(ref.Owner, ref.Repo), Body: gi.Body, Author: gi.User.user(), CreatedAt: gi.CreatedAt}
	ed.apply(is, comments)
	return is, comments, nil
}

// editQuery reads who last edited the body, who last renamed the title, and one page of comments with who last
// edited each. $after is the cursor of the page before; null for the first.
const editQuery = `query($owner: String!, $repo: String!, $number: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      lastEditedAt
      editor { login ... on User { databaseId } ... on Bot { databaseId } }
      timelineItems(last: 1, itemTypes: [RENAMED_TITLE_EVENT]) {
        nodes { ... on RenamedTitleEvent { actor { login ... on User { databaseId } ... on Bot { databaseId } } } }
      }
      comments(first: 100, after: $after) {
        pageInfo { hasNextPage endCursor }
        nodes { databaseId lastEditedAt editor { login ... on User { databaseId } ... on Bot { databaseId } } }
      }
    }
  }
}`

// maxEditPages bounds how many pages of comments the edit query reads (10,000 comments). A comment beyond them is
// marked Unknown.
const maxEditPages = 100

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

// titleEdit is the edit of the title by the last RenamedTitleEvent: none when the title was never renamed, Unknown
// when the event names no actor (a deleted account).
func titleEdit(events []struct {
	Actor *gqlActor `json:"actor"`
}) forge.Edit {
	if len(events) == 0 {
		return forge.Edit{}
	}
	a := events[len(events)-1].Actor
	if a == nil || a.DatabaseID == 0 {
		return forge.Edit{Edited: true, Unknown: true}
	}
	return forge.Edit{Edited: true, Editor: &forge.User{Login: a.Login, ID: a.DatabaseID}}
}

// edits is who last edited the body and the title of an issue and each comment the edit query returned, by comment
// ID.
type edits struct {
	body, title forge.Edit
	comments    map[int64]forge.Edit
}

// apply fills in who last edited the body, the title and each comment. A comment the query did not return (posted
// while it read, or beyond maxEditPages) is marked Unknown, so that trust decisions treat it as edited by an
// untrusted user.
func (e *edits) apply(is *forge.Issue, comments []forge.Comment) {
	is.BodyEdit = e.body
	is.TitleEdit = e.title
	for i := range comments {
		ed, ok := e.comments[comments[i].ID]
		if !ok {
			ed = forge.Edit{Edited: true, Unknown: true}
		}
		comments[i].Edit = ed
	}
}

// edits reads who last edited the body and the title of an issue, and every page of its comments, oldest first.
func (c *Client) edits(ctx context.Context, ref forge.IssueRef) (*edits, error) {
	var e *edits
	var after any
	for page := 0; page < maxEditPages; page++ {
		in := map[string]any{"query": editQuery, "variables": map[string]any{"owner": ref.Owner, "repo": ref.Repo, "number": ref.Number, "after": after}}
		var out struct {
			Data struct {
				Repository struct {
					Issue struct {
						gqlEdited
						TimelineItems struct {
							Nodes []struct {
								Actor *gqlActor `json:"actor"`
							} `json:"nodes"`
						} `json:"timelineItems"`
						Comments struct {
							PageInfo struct {
								HasNextPage bool   `json:"hasNextPage"`
								EndCursor   string `json:"endCursor"`
							} `json:"pageInfo"`
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
			return nil, err
		}
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("github: graphql: %s", out.Errors[0].Message)
		}
		gi := out.Data.Repository.Issue
		if e == nil {
			e = &edits{body: gi.edit(), title: titleEdit(gi.TimelineItems.Nodes), comments: map[int64]forge.Edit{}}
		}
		for _, n := range gi.Comments.Nodes {
			e.comments[n.DatabaseID] = n.edit()
		}
		info := gi.Comments.PageInfo
		if !info.HasNextPage || info.EndCursor == "" {
			break
		}
		after = info.EndCursor
	}
	return e, nil
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

// EditComment implements forge.Forge.
func (c *Client) EditComment(ctx context.Context, ref forge.IssueRef, id int64, body string) error {
	_, err := c.do(ctx, http.MethodPatch, c.url(fmt.Sprintf("/repos/%s/%s/issues/comments/%d", ref.Owner, ref.Repo, id), nil),
		map[string]any{"body": body}, nil, nil)
	return err
}

// DeleteComment implements forge.Forge.
func (c *Client) DeleteComment(ctx context.Context, ref forge.IssueRef, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, c.url(fmt.Sprintf("/repos/%s/%s/issues/comments/%d", ref.Owner, ref.Repo, id), nil), nil, nil, nil)
	return err
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

// OpenChangeRequest implements forge.Forge. switchboard pushes to the repository itself, so the head's owner is
// the repository's.
func (c *Client) OpenChangeRequest(ctx context.Context, owner, repo, head, base string) (string, error) {
	var out []struct {
		HTMLURL string `json:"html_url"`
	}
	q := url.Values{"state": {"open"}, "head": {owner + ":" + head}, "base": {base}}
	if _, err := c.do(ctx, http.MethodGet, c.url(fmt.Sprintf("/repos/%s/%s/pulls", owner, repo), q), nil, &out, nil); err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "", forge.ErrNotFound
	}
	return out[0].HTMLURL, nil
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

// reposPerSearch is how many repo: qualifiers one search holds, well within its limit of 256 characters.
const reposPerSearch = 4

// searchQueries are the search qualifiers that cover scope: its query when set, else its projects as repo:
// qualifiers, a few to a search. A scope with neither has no query: an empty one would search the whole of GitHub.
func searchQueries(scope forge.Scope) []string {
	if scope.Query != "" {
		return []string{scope.Query}
	}
	var repos []string
	for _, p := range scope.Projects {
		if q := "repo:" + p; !slices.Contains(repos, q) {
			repos = append(repos, q)
		}
	}
	var out []string
	for len(repos) > 0 {
		n := min(reposPerSearch, len(repos))
		out = append(out, strings.Join(repos[:n], " "))
		repos = repos[n:]
	}
	return out
}

// SearchOpenIssuesWithLabel implements forge.Forge. Each search reads the first 100 results, which covers any
// realistic number of waiting issues.
func (c *Client) SearchOpenIssuesWithLabel(ctx context.Context, scope forge.Scope, labels ...string) ([]forge.IssueSummary, error) {
	var res []forge.IssueSummary
	seen := map[forge.IssueRef]bool{}
	for _, q := range searchQueries(scope) {
		found, err := c.search(ctx, q, labels)
		if err != nil {
			return nil, err
		}
		for _, s := range found {
			if !seen[s.Ref] {
				seen[s.Ref] = true
				res = append(res, s)
			}
		}
	}
	return res, nil
}

// search runs one issue search within the qualifiers scope. Several labels go into one qualifier, label:"a","b",
// which matches any of them (V28).
func (c *Client) search(ctx context.Context, scope string, labels []string) ([]forge.IssueSummary, error) {
	quoted := make([]string, len(labels))
	for i, l := range labels {
		quoted[i] = strconv.Quote(l)
	}
	q := fmt.Sprintf(`is:issue is:open label:%s %s`, strings.Join(quoted, ","), scope)
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
