// Package inbox lists the waiting issues of every configured project, shows the agent's last question on one, and
// posts the user's reply. It reads and writes as the user (inbox_token_ref), not as the agent.
package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/config"
	editorpkg "github.com/uskanda/switchboard/internal/editor"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/marker"
)

// Inbox reads the forges of the configured projects.
type Inbox struct {
	Config *config.Config
	// Forge is the GitHub client; Forges, keyed by forge.GitHub or forge.GitLab, holds the others (and may hold
	// GitHub's too). Each reads and writes with the forge's inbox_token_ref.
	Forge  forge.Forge
	Forges map[string]forge.Forge
	// Only, when set to forge.GitHub or forge.GitLab, lists that forge's issues only.
	Only string
	// Cache is the file the row numbers of the last listing are kept in.
	Cache string
	Now   func() time.Time
}

// DefaultCache is $XDG_CACHE_HOME/switchboard/inbox.json, with ~/.cache when XDG_CACHE_HOME is unset.
func DefaultCache() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	return filepath.Join(base, "switchboard", "inbox.json")
}

// Item is one waiting or running issue. On a waiting issue, Host, WaitingSince and Reason come only from the marker
// of the agent's latest post; "?" (or null) when there is none. On a running issue, Host and RunningSince come from
// the marker of the claim of the run, and Progress is the last line of its progress comment ("?" without one).
// Only waiting issues are numbered.
type Item struct {
	N            int        `json:"n,omitempty"`
	State        string     `json:"state"` // waiting or running
	Ref          string     `json:"ref"`
	Project      string     `json:"project"`
	Number       int        `json:"number"`
	Title        string     `json:"title"`
	URL          string     `json:"url"`
	Host         string     `json:"host"`
	WaitingSince *time.Time `json:"waiting_since"`
	Reason       string     `json:"reason"`
	Question     string     `json:"question"`
	RunningSince *time.Time `json:"running_since,omitempty"`
	Progress     string     `json:"progress,omitempty"`
	// UpdatedAt and Notes are the forge's update time and count of people's comments (GitLab's user_notes_count, 0
	// on GitHub): a page watching one issue reads it again when either moves.
	UpdatedAt *time.Time `json:"updated_at"`
	Notes     int        `json:"notes"`
}

// The states of an Item.
const (
	Waiting = "waiting"
	Running = "running"
)

func refString(r forge.IssueRef) string { return r.String() }

// kindOf is the project's forge; a project built without one (as Phase 1 code did) is on GitHub.
func kindOf(p config.Project) string {
	if p.Forge == "" {
		return forge.GitHub
	}
	return p.Forge
}

// forgeOf is the inbox's client for a forge kind.
func (b *Inbox) forgeOf(kind string) forge.Forge {
	if f, ok := b.Forges[kind]; ok {
		return f
	}
	if kind == forge.GitHub {
		return b.Forge
	}
	return nil
}

// project returns the configured project of an issue.
func (b *Inbox) project(kind, owner, repo string) (config.Project, bool) {
	for _, p := range b.Config.Projects {
		if kindOf(p) == kind && p.Owner == owner && p.Repo == repo {
			return p, true
		}
	}
	return config.Project{}, false
}

// agentID resolves the identity the agent posts as on the issue's project.
func (b *Inbox) agentID(ctx context.Context, ref forge.IssueRef) (int64, error) {
	p, _ := b.project(ref.Kind(), ref.Owner, ref.Repo)
	p.Forge = ref.Kind()
	login := b.Config.AgentIdentity(p)
	u, err := b.forgeOf(ref.Kind()).User(ctx, login)
	if err != nil {
		return 0, fmt.Errorf("resolve agent_identity %s: %w", login, err)
	}
	return u.ID, nil
}

// latestPost returns the agent's latest post: authored by the agent and ending in a marker. Claims of a run
// (state=running) and its progress comments (state=progress) say nothing about why the issue waits, so they are
// passed over.
func latestPost(comments []forge.Comment, agentID int64) (*forge.Comment, *marker.Marker) {
	for i := len(comments) - 1; i >= 0; i-- {
		if comments[i].Author.ID != agentID {
			continue
		}
		if m, ok := marker.Parse(comments[i].Body); ok && !m.Transient() {
			return &comments[i], m
		}
	}
	return nil, nil
}

// List collects the waiting issues of the configured projects on every forge (or on Only), oldest wait first, and
// records the row numbers. When a forge cannot be read, List still returns the other forges' issues, with an error
// naming the forge that failed.
func (b *Inbox) List(ctx context.Context) ([]Item, error) {
	items, ferrs := b.Collect(ctx)
	var rows []string
	for _, it := range items {
		if it.N > 0 {
			rows = append(rows, it.Ref)
		}
	}
	if err := b.saveRows(rows); err != nil {
		return nil, err
	}
	var errs []error
	for _, e := range ferrs {
		errs = append(errs, e)
	}
	return items, errors.Join(errs...)
}

// ForgeError is a forge that could not be read while listing.
type ForgeError struct {
	Forge string // "github" or "gitlab"
	Err   error
}

func (e ForgeError) Error() string { return e.Forge + " could not be read: " + e.Err.Error() }
func (e ForgeError) Unwrap() error { return e.Err }

// Collect lists the waiting issues as List does, numbered, and after them the running issues, without recording the
// rows. The forges that could not
// be read are returned beside what the others listed.
func (b *Inbox) Collect(ctx context.Context) ([]Item, []ForgeError) {
	var kinds []string
	for _, k := range []string{forge.GitHub, forge.GitLab} {
		if (b.Only == "" || b.Only == k) && b.uses(k) {
			kinds = append(kinds, k)
		}
	}
	type result struct {
		items []Item
		err   error
	}
	results := make([]result, len(kinds))
	var wg sync.WaitGroup
	for i, k := range kinds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := b.list(ctx, k)
			results[i] = result{items, err}
		}()
	}
	wg.Wait()
	var items []Item
	var errs []ForgeError
	for i, r := range results {
		items = append(items, r.items...)
		if r.err != nil {
			errs = append(errs, ForgeError{Forge: forgeName(kinds[i]), Err: r.err})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].State != items[j].State {
			return items[i].State == Waiting
		}
		a, c := items[i].WaitingSince, items[j].WaitingSince
		if items[i].State == Running {
			a, c = items[i].RunningSince, items[j].RunningSince
		}
		switch {
		case a == nil || c == nil:
			return a != nil && c == nil
		case !a.Equal(*c):
			return a.Before(*c)
		}
		return items[i].Ref < items[j].Ref
	})
	for i := range items {
		if items[i].State == Waiting {
			items[i].N = i + 1
		}
	}
	return items, errs
}

func forgeName(kind string) string {
	if kind == forge.GitLab {
		return "gitlab"
	}
	return "github"
}

func (b *Inbox) uses(kind string) bool {
	for _, p := range b.Config.Projects {
		if kindOf(p) == kind {
			return true
		}
	}
	return false
}

// list reads one forge's waiting and running issues. GitHub is searched once over search_scope; GitLab is read
// project by project, since it has no search over the configured projects alone.
func (b *Inbox) list(ctx context.Context, kind string) ([]Item, error) {
	f := b.forgeOf(kind)
	if f == nil {
		return nil, errors.New("no client")
	}
	var found []forge.IssueSummary
	if kind == forge.GitHub {
		s, err := f.SearchOpenIssuesWithLabel(ctx, b.Config.Forges.GitHub.SearchScope, b.Config.Labels.Waiting, b.Config.Labels.Running)
		if err != nil {
			return nil, err
		}
		found = s
	} else {
		for _, p := range b.Config.Projects {
			if kindOf(p) != kind {
				continue
			}
			s, err := f.SearchOpenIssuesWithLabel(ctx, p.FullName(), b.Config.Labels.Waiting, b.Config.Labels.Running)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", p.Ref, err)
			}
			found = append(found, s...)
		}
	}
	ids := map[string]int64{}
	var items []Item
	for _, s := range found {
		if _, ok := b.project(s.Ref.Kind(), s.Ref.Owner, s.Ref.Repo); !ok {
			continue
		}
		agentID, ok := ids[s.Ref.Project()]
		if !ok {
			id, err := b.agentID(ctx, s.Ref)
			if err != nil {
				return nil, err
			}
			agentID, ids[s.Ref.Project()] = id, id
		}
		_, comments, err := f.Thread(ctx, s.Ref)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Ref, err)
		}
		if labels.StateOf(s.Labels, b.Config.Labels) == b.Config.Labels.Running {
			items = append(items, withUpdate(runningItem(s, comments, agentID), s))
			continue
		}
		it := withUpdate(Item{State: Waiting, Ref: refString(s.Ref), Project: s.Ref.Project(), Number: s.Ref.Number, Title: s.Title, URL: s.URL, Host: "?", Reason: "?", Question: "?"}, s)
		if c, m := latestPost(comments, agentID); c != nil {
			it.Host, it.Reason, it.Question = orQ(m.Host), orQ(m.Reason), Excerpt(c.Body)
			if !m.At.IsZero() {
				at := m.At
				it.WaitingSince = &at
			}
		}
		items = append(items, it)
	}
	return items, nil
}

// withUpdate sets what tells a watcher that the issue changed.
func withUpdate(it Item, s forge.IssueSummary) Item {
	if !s.UpdatedAt.IsZero() {
		at := s.UpdatedAt
		it.UpdatedAt = &at
	}
	it.Notes = s.Notes
	return it
}

// runningItem is the row of a running issue: where and since when it runs, from the claim of the run, and the last
// line of the run's progress comment.
func runningItem(s forge.IssueSummary, comments []forge.Comment, agentID int64) Item {
	it := Item{State: Running, Ref: refString(s.Ref), Project: s.Ref.Project(), Number: s.Ref.Number, Title: s.Title, URL: s.URL, Host: "?", Progress: "?"}
	claim, cm := latestOf(comments, agentID, marker.Running)
	if claim == nil {
		return it
	}
	it.Host = orQ(cm.Host)
	if !cm.At.IsZero() {
		at := cm.At
		it.RunningSince = &at
	}
	if p, pm := latestOf(comments, agentID, marker.Progress); p != nil && pm.Run == cm.Run {
		it.Progress = lastLine(body(p.Body))
	}
	return it
}

// latestOf returns the agent's latest post whose marker is in state.
func latestOf(comments []forge.Comment, agentID int64, state string) (*forge.Comment, *marker.Marker) {
	for i := len(comments) - 1; i >= 0; i-- {
		if comments[i].Author.ID != agentID {
			continue
		}
		if m, ok := marker.Parse(comments[i].Body); ok && m.State == state {
			return &comments[i], m
		}
	}
	return nil, nil
}

// lastLine is the last item of a progress comment's list, "?" when it has none.
func lastLine(text string) string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l, ok := strings.CutPrefix(strings.TrimSpace(lines[i]), "- "); ok {
			return l
		}
	}
	return "?"
}

func orQ(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

// body returns a post without its marker line.
func body(post string) string {
	lines := strings.Split(strings.TrimRight(post, " \t\r\n"), "\n")
	return strings.TrimRight(strings.Join(lines[:len(lines)-1], "\n"), " \t\r\n")
}

// Excerpt is the start of the question in an agent post, on one line: the paragraph after the question label, or
// else the first paragraph after the heading.
func Excerpt(post string) string {
	if q := agent.Question(body(post)); q != "" {
		return clip(q, 72)
	}
	return "?"
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func age(since *time.Time, now time.Time) string {
	if since == nil {
		return "?"
	}
	d := now.Sub(*since)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// Print writes the listing as aligned columns: the waiting issues, then the running ones under their own heading.
func Print(w io.Writer, items []Item, now time.Time) {
	var waiting, running []Item
	for _, it := range items {
		if it.State == Running {
			running = append(running, it)
		} else {
			waiting = append(waiting, it)
		}
	}
	if len(waiting) == 0 {
		fmt.Fprintln(w, "No issue is waiting.")
	} else {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "#\tISSUE\tHOST\tWAITING\tREASON\tQUESTION")
		for _, it := range waiting {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n", it.N, it.Ref, it.Host, age(it.WaitingSince, now), it.Reason, it.Question)
		}
		tw.Flush()
	}
	if len(running) == 0 {
		return
	}
	fmt.Fprintln(w, "\nRunning")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ISSUE\tHOST\tRUNNING\tPROGRESS")
	for _, it := range running {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", it.Ref, it.Host, age(it.RunningSince, now), it.Progress)
	}
	tw.Flush()
}

// PrintJSON writes the listing as a JSON array.
func PrintJSON(w io.Writer, items []Item) error {
	if items == nil {
		items = []Item{}
	}
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(items)
}

func (b *Inbox) saveRows(rows []string) error {
	data, err := json.Marshal(map[string]any{"rows": rows})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(b.Cache), 0o700); err != nil {
		return err
	}
	return os.WriteFile(b.Cache, append(data, '\n'), 0o600)
}

// The errors of Resolve, Detail and Reply can be told apart with errors.Is.
var (
	ErrInvalid    = errors.New("invalid")     // the request itself is wrong: a malformed ref, an empty reply
	ErrNotFound   = errors.New("not found")   // the issue is not of a configured project, or has no agent post
	ErrNotWaiting = errors.New("not waiting") // the issue no longer waits, so the reply was not posted
)

// wrap marks err with kind for errors.Is and keeps its message.
func wrap(kind, err error) error { return kindError{kind, err} }

type kindError struct{ kind, err error }

func (e kindError) Error() string        { return e.err.Error() }
func (e kindError) Is(target error) bool { return target == e.kind }
func (e kindError) Unwrap() error        { return e.err }

var issueRef = regexp.MustCompile(`^(gh):([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)#([0-9]+)$|^(gl):((?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)#([0-9]+)$`)

// Resolve turns a row number of the last listing, gh:owner/repo#n, or gl:group/project#n into an issue of a
// configured project.
func (b *Inbox) Resolve(arg string) (forge.IssueRef, error) {
	if n, err := strconv.Atoi(arg); err == nil {
		var saved struct {
			Rows []string `json:"rows"`
		}
		data, err := os.ReadFile(b.Cache)
		if errors.Is(err, os.ErrNotExist) {
			return forge.IssueRef{}, fmt.Errorf("there is no listing to take row %d from; run switchboard inbox first", n)
		}
		if err != nil {
			return forge.IssueRef{}, err
		}
		if err := json.Unmarshal(data, &saved); err != nil {
			return forge.IssueRef{}, fmt.Errorf("%s: %w", b.Cache, err)
		}
		if n < 1 || n > len(saved.Rows) {
			return forge.IssueRef{}, fmt.Errorf("the last listing has no row %d; run switchboard inbox again", n)
		}
		arg = saved.Rows[n-1]
	} else if !issueRef.MatchString(arg) {
		return forge.IssueRef{}, wrap(ErrInvalid, fmt.Errorf("%q is neither a row number nor of the form gh:owner/repo#123 or gl:group/project#123", arg))
	}
	return b.ParseRef(arg)
}

// ParseRef turns gh:owner/repo#n or gl:group/project#n into an issue of a configured project. Row numbers are not
// accepted.
func (b *Inbox) ParseRef(arg string) (forge.IssueRef, error) {
	m := issueRef.FindStringSubmatch(arg)
	if m == nil {
		return forge.IssueRef{}, wrap(ErrInvalid, fmt.Errorf("%q is not of the form gh:owner/repo#123 or gl:group/project#123", arg))
	}
	if m[1] == "" {
		m = append([]string{m[0]}, m[5:]...)
	} else {
		m = m[:5]
	}
	num, _ := strconv.Atoi(m[4])
	if _, ok := b.project(m[1], m[2], m[3]); !ok {
		return forge.IssueRef{}, wrap(ErrNotFound, fmt.Errorf("%s:%s/%s is not a configured project", m[1], m[2], m[3]))
	}
	return forge.IssueRef{Forge: m[1], Owner: m[2], Repo: m[3], Number: num}, nil
}

// Detail is one issue as show presents it: the agent's latest post, which holds the question and the refused
// operations, with what its marker says.
type Detail struct {
	Ref     string     `json:"ref"`
	Title   string     `json:"title"`
	URL     string     `json:"url"`
	Waiting bool       `json:"waiting"` // open and labeled waiting
	Host    string     `json:"host"`
	State   string     `json:"state"` // the state of the agent's latest post, or running while the issue runs
	Reason  string     `json:"reason"`
	Since   *time.Time `json:"since"`
	Post    string     `json:"post"` // the agent's latest post, without its marker
	// ApproveWord is the word that approves a plan (plan.approve_word), for a page to say how to approve one.
	ApproveWord string `json:"approve_word,omitempty"`
	// Author, CreatedAt and Body are the issue's own; Comments are all its comments, oldest first.
	Author    string          `json:"author"`
	CreatedAt *time.Time      `json:"created_at"`
	Body      string          `json:"body"`
	Comments  []ThreadComment `json:"comments"`
}

// ThreadComment is one comment of an issue. Agent is set on a post of the agent (its identity, ending in a marker);
// Kind is then the marker's state (running, progress, waiting, done) and Body is without the marker. A person's
// comment is returned as written: it is data, and a marker-shaped line in it does not make it the agent's.
type ThreadComment struct {
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	Body      string    `json:"body"`
	Agent     bool      `json:"agent"`
	Kind      string    `json:"kind,omitempty"`
}

// withThread adds the issue's body and comments to d.
func withThread(d *Detail, is *forge.Issue, comments []forge.Comment, agentID int64) *Detail {
	d.Author, d.Body = is.Author.Login, is.Body
	if !is.CreatedAt.IsZero() {
		at := is.CreatedAt
		d.CreatedAt = &at
	}
	d.Comments = []ThreadComment{}
	for _, c := range comments {
		tc := ThreadComment{Author: c.Author.Login, CreatedAt: c.CreatedAt, Body: c.Body}
		if c.Author.ID == agentID {
			if m, ok := marker.Parse(c.Body); ok {
				tc.Agent, tc.Kind, tc.Body = true, m.State, body(c.Body)
			}
		}
		d.Comments = append(d.Comments, tc)
	}
	return d
}

// Detail reads the issue and the agent's latest post on it.
func (b *Inbox) Detail(ctx context.Context, ref forge.IssueRef) (*Detail, error) {
	agentID, err := b.agentID(ctx, ref)
	if err != nil {
		return nil, err
	}
	is, comments, err := b.forgeOf(ref.Kind()).Thread(ctx, ref)
	if err != nil {
		return nil, err
	}
	if is.State == "open" && labels.StateOf(is.Labels, b.Config.Labels) == b.Config.Labels.Running {
		if d := runningDetail(is, comments, agentID); d != nil {
			return withThread(d, is, comments, agentID), nil
		}
	}
	c, m := latestPost(comments, agentID)
	if c == nil {
		return nil, wrap(ErrNotFound, fmt.Errorf("%s has no post from the agent", refString(ref)))
	}
	d := &Detail{
		Ref: refString(ref), Title: is.Title, URL: is.URL,
		Waiting: is.State == "open" && labels.StateOf(is.Labels, b.Config.Labels) == b.Config.Labels.Waiting,
		Host:    orQ(m.Host), State: orQ(m.State), Reason: orQ(m.Reason), Post: body(c.Body),
		ApproveWord: b.Config.Plan.ApproveWord,
	}
	if !m.At.IsZero() {
		at := m.At
		d.Since = &at
	}
	return withThread(d, is, comments, agentID), nil
}

// runningDetail is a running issue with the progress comment of its run, or the claim while there is none. It is nil
// when the agent has not claimed the issue.
func runningDetail(is *forge.Issue, comments []forge.Comment, agentID int64) *Detail {
	claim, cm := latestOf(comments, agentID, marker.Running)
	if claim == nil {
		return nil
	}
	d := &Detail{Ref: refString(is.Ref), Title: is.Title, URL: is.URL, Host: orQ(cm.Host), State: Running, Reason: "?", Post: body(claim.Body)}
	if p, pm := latestOf(comments, agentID, marker.Progress); p != nil && pm.Run == cm.Run {
		d.Post = body(p.Body)
	}
	if !cm.At.IsZero() {
		at := cm.At
		d.Since = &at
	}
	return d
}

// Show writes the agent's latest post on the issue, without its marker.
func (b *Inbox) Show(ctx context.Context, ref forge.IssueRef, w io.Writer) error {
	d, err := b.Detail(ctx, ref)
	if err != nil {
		return err
	}
	PrintDetail(w, d)
	return nil
}

// PrintDetail writes d for a person to read.
func PrintDetail(w io.Writer, d *Detail) {
	since := "?"
	if d.Since != nil {
		since = d.Since.Local().Format(time.RFC3339)
	}
	fmt.Fprintf(w, "%s  %s\nhost %s, %s (%s), since %s\n\n%s\n", d.Ref, d.Title, d.Host, d.State, d.Reason, since, d.Post)
}

// Reply posts text on the issue. Unless force is set, the issue must still be waiting.
func (b *Inbox) Reply(ctx context.Context, ref forge.IssueRef, text string, force bool) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return wrap(ErrInvalid, errors.New("the reply is empty"))
	}
	if _, ok := marker.Parse(text); ok {
		return wrap(ErrInvalid, errors.New("the reply ends in a switchboard marker line; remove it"))
	}
	f := b.forgeOf(ref.Kind())
	is, _, err := f.Thread(ctx, ref)
	if err != nil {
		return err
	}
	if st := labels.StateOf(is.Labels, b.Config.Labels); !force && (is.State != "open" || st != b.Config.Labels.Waiting) {
		now := st
		if is.State != "open" {
			now = "closed"
		}
		if now == "" {
			now = "no state label"
		}
		return wrap(ErrNotWaiting, fmt.Errorf("%s is not waiting (%s); nothing was posted. Use --force to post anyway", refString(ref), now))
	}
	_, err = f.PostComment(ctx, ref, text)
	return err
}

const scissors = "------------------------ >8 ------------------------"

// EditorText opens editor on a draft and returns what the user wrote above the scissors line.
func EditorText(editor, ref, question string) (string, error) {
	f, err := os.CreateTemp("", "switchboard-reply-*.md")
	if err != nil {
		return "", err
	}
	path := f.Name()
	defer os.Remove(path)
	fmt.Fprintf(f, "\n%s\nWrite the reply to %s above this line. Everything below it is dropped.\n\n%s\n", scissors, ref, question)
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := editorpkg.Run(editor, path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text, _, _ := strings.Cut(string(data), scissors)
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("the reply is empty; nothing was posted")
	}
	return text, nil
}
