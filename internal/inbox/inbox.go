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
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/marker"
)

// Inbox reads the forge for the configured projects.
type Inbox struct {
	Config *config.Config
	Forge  forge.Forge
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

// Item is one waiting issue. Host, WaitingSince and Reason come only from the marker of the agent's latest post;
// "?" (or null) when there is none.
type Item struct {
	N            int        `json:"n"`
	Ref          string     `json:"ref"`
	Project      string     `json:"project"`
	Number       int        `json:"number"`
	Title        string     `json:"title"`
	Host         string     `json:"host"`
	WaitingSince *time.Time `json:"waiting_since"`
	Reason       string     `json:"reason"`
	Question     string     `json:"question"`
}

func refString(r forge.IssueRef) string { return r.String() }

func (b *Inbox) configured(owner, repo string) bool {
	for _, p := range b.Config.Projects {
		if p.Owner == owner && p.Repo == repo {
			return true
		}
	}
	return false
}

func (b *Inbox) agentID(ctx context.Context) (int64, error) {
	u, err := b.Forge.User(ctx, b.Config.Forges.GitHub.AgentIdentity)
	if err != nil {
		return 0, fmt.Errorf("resolve agent_identity %s: %w", b.Config.Forges.GitHub.AgentIdentity, err)
	}
	return u.ID, nil
}

// latestPost returns the agent's latest post: authored by the agent and ending in a marker.
func latestPost(comments []forge.Comment, agentID int64) (*forge.Comment, *marker.Marker) {
	for i := len(comments) - 1; i >= 0; i-- {
		if comments[i].Author.ID != agentID {
			continue
		}
		if m, ok := marker.Parse(comments[i].Body); ok {
			return &comments[i], m
		}
	}
	return nil, nil
}

// List collects the waiting issues of the configured projects, oldest wait first, and records the row numbers.
func (b *Inbox) List(ctx context.Context) ([]Item, error) {
	agentID, err := b.agentID(ctx)
	if err != nil {
		return nil, err
	}
	found, err := b.Forge.SearchOpenIssuesWithLabel(ctx, b.Config.Forges.GitHub.SearchScope, b.Config.Labels.Waiting)
	if err != nil {
		return nil, err
	}
	var items []Item
	for _, s := range found {
		if !b.configured(s.Ref.Owner, s.Ref.Repo) {
			continue
		}
		_, comments, err := b.Forge.Thread(ctx, s.Ref)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Ref, err)
		}
		it := Item{Ref: refString(s.Ref), Project: s.Ref.Owner + "/" + s.Ref.Repo, Number: s.Ref.Number, Title: s.Title, Host: "?", Reason: "?", Question: "?"}
		if c, m := latestPost(comments, agentID); c != nil {
			it.Host, it.Reason, it.Question = orQ(m.Host), orQ(m.Reason), Excerpt(c.Body)
			if !m.At.IsZero() {
				at := m.At
				it.WaitingSince = &at
			}
		}
		items = append(items, it)
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, c := items[i].WaitingSince, items[j].WaitingSince
		switch {
		case a == nil || c == nil:
			return a != nil && c == nil
		case !a.Equal(*c):
			return a.Before(*c)
		}
		return items[i].Ref < items[j].Ref
	})
	rows := make([]string, len(items))
	for i := range items {
		items[i].N = i + 1
		rows[i] = items[i].Ref
	}
	if err := b.saveRows(rows); err != nil {
		return nil, err
	}
	return items, nil
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
	text := body(post)
	for _, l := range agent.QuestionLabels() {
		if i := strings.LastIndex(text, l+"\n"); i >= 0 {
			text = text[i+len(l):]
			break
		}
	}
	for _, para := range strings.Split(text, "\n\n") {
		p := strings.TrimSpace(para)
		if p == "" || strings.HasPrefix(p, "#") || (strings.HasPrefix(p, "**") && strings.HasSuffix(p, "**")) {
			continue
		}
		return clip(strings.Join(strings.Fields(p), " "), 72)
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

// Print writes the listing as aligned columns.
func Print(w io.Writer, items []Item, now time.Time) {
	if len(items) == 0 {
		fmt.Fprintln(w, "No issue is waiting.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "#\tISSUE\tHOST\tWAITING\tREASON\tQUESTION")
	for _, it := range items {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n", it.N, it.Ref, it.Host, age(it.WaitingSince, now), it.Reason, it.Question)
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

var issueRef = regexp.MustCompile(`^gh:([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)#([0-9]+)$`)

// Resolve turns a row number of the last listing, or gh:owner/repo#n, into an issue of a configured project.
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
	}
	m := issueRef.FindStringSubmatch(arg)
	if m == nil {
		return forge.IssueRef{}, fmt.Errorf("%q is neither a row number nor of the form gh:owner/repo#123", arg)
	}
	num, _ := strconv.Atoi(m[3])
	if !b.configured(m[1], m[2]) {
		return forge.IssueRef{}, fmt.Errorf("%s/%s is not a configured project", m[1], m[2])
	}
	return forge.IssueRef{Forge: forge.GitHub, Owner: m[1], Repo: m[2], Number: num}, nil
}

// Show writes the agent's latest post on the issue, without its marker.
func (b *Inbox) Show(ctx context.Context, ref forge.IssueRef, w io.Writer) error {
	agentID, err := b.agentID(ctx)
	if err != nil {
		return err
	}
	is, comments, err := b.Forge.Thread(ctx, ref)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%s  %s\n", refString(ref), is.Title)
	c, m := latestPost(comments, agentID)
	if c == nil {
		return fmt.Errorf("%s has no post from the agent", refString(ref))
	}
	since := "?"
	if !m.At.IsZero() {
		since = m.At.Local().Format(time.RFC3339)
	}
	fmt.Fprintf(w, "host %s, %s (%s), since %s\n\n%s\n", orQ(m.Host), orQ(m.State), orQ(m.Reason), since, body(c.Body))
	return nil
}

// Reply posts text on the issue. Unless force is set, the issue must still be waiting.
func (b *Inbox) Reply(ctx context.Context, ref forge.IssueRef, text string, force bool) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("the reply is empty")
	}
	if _, ok := marker.Parse(text); ok {
		return errors.New("the reply ends in a switchboard marker line; remove it")
	}
	is, _, err := b.Forge.Thread(ctx, ref)
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
		return fmt.Errorf("%s is not waiting (%s); nothing was posted. Use --force to post anyway", refString(ref), now)
	}
	_, err = b.Forge.PostComment(ctx, ref, text)
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
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %s: %w", editor, err)
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
