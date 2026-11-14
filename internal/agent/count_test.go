package agent

import (
	"context"
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
)

// countingForge wraps a forge and counts the agent's calls to it, by method. It hides the optional interfaces of
// the forge it wraps (CI, attachments), so it suits tests that need none of them.
type countingForge struct {
	forge.Forge
	mu sync.Mutex
	n  map[string]int
	// After limitAfter more calls, every call is refused with a rate limit until limitUntil by now, as GitHub
	// answers once a token used up its limit. limitAfter < 0 sets no limit.
	limitAfter int
	limitUntil time.Time
	now        func() time.Time
}

func newCountingForge(f forge.Forge) *countingForge {
	return &countingForge{Forge: f, n: map[string]int{}, limitAfter: -1, now: time.Now}
}

// limit makes the calls after the next n ones refused for a rate limit until until.
func (c *countingForge) limit(n int, until time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.limitAfter, c.limitUntil = n, until
}

// add counts a call of m and returns the rate limit that refuses it, if any.
func (c *countingForge) add(m string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n[m]++
	switch {
	case c.limitAfter > 0:
		c.limitAfter--
	case c.limitAfter == 0 && c.now().Before(c.limitUntil):
		return &forge.RateLimitError{Reset: c.limitUntil, Err: forge.ErrForbidden}
	}
	return nil
}

// take returns the counts since the last take and starts counting again.
func (c *countingForge) take() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.n
	c.n = map[string]int{}
	return out
}

func (c *countingForge) UpdatedIssues(ctx context.Context, o, r string, s time.Time) ([]forge.IssueSummary, error) {
	if err := c.add("UpdatedIssues"); err != nil {
		return nil, err
	}
	return c.Forge.UpdatedIssues(ctx, o, r, s)
}

func (c *countingForge) Thread(ctx context.Context, ref forge.IssueRef) (*forge.Issue, []forge.Comment, error) {
	if err := c.add("Thread"); err != nil {
		return nil, nil, err
	}
	return c.Forge.Thread(ctx, ref)
}

func (c *countingForge) Comments(ctx context.Context, ref forge.IssueRef) ([]forge.Comment, error) {
	if err := c.add("Comments"); err != nil {
		return nil, err
	}
	return c.Forge.Comments(ctx, ref)
}

func (c *countingForge) LabelActor(ctx context.Context, ref forge.IssueRef, l string) (*forge.User, error) {
	if err := c.add("LabelActor"); err != nil {
		return nil, err
	}
	return c.Forge.LabelActor(ctx, ref, l)
}

func (c *countingForge) SetLabels(ctx context.Context, ref forge.IssueRef, l []string) error {
	if err := c.add("SetLabels"); err != nil {
		return err
	}
	return c.Forge.SetLabels(ctx, ref, l)
}

func (c *countingForge) PostComment(ctx context.Context, ref forge.IssueRef, b string) (*forge.Comment, error) {
	if err := c.add("PostComment"); err != nil {
		return nil, err
	}
	return c.Forge.PostComment(ctx, ref, b)
}

func (c *countingForge) EditComment(ctx context.Context, ref forge.IssueRef, id int64, b string) error {
	if err := c.add("EditComment"); err != nil {
		return err
	}
	return c.Forge.EditComment(ctx, ref, id, b)
}

func (c *countingForge) DeleteComment(ctx context.Context, ref forge.IssueRef, id int64) error {
	if err := c.add("DeleteComment"); err != nil {
		return err
	}
	return c.Forge.DeleteComment(ctx, ref, id)
}

func (c *countingForge) SearchOpenIssuesWithLabel(ctx context.Context, s forge.Scope, l ...string) ([]forge.IssueSummary, error) {
	if err := c.add("SearchOpenIssuesWithLabel"); err != nil {
		return nil, err
	}
	return c.Forge.SearchOpenIssuesWithLabel(ctx, s, l...)
}
