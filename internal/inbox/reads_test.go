package inbox

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// counting wraps a forge and records what the listing asks of it.
type counting struct {
	forge.Forge
	mu                    sync.Mutex
	searches, threads     int
	reads                 map[forge.IssueRef]int
	inflight, maxInflight int
	hold                  time.Duration
	// extra, when set, is appended to the comments Comments returns for that issue: a change the search does not
	// show, as an edited comment is.
	extra map[forge.IssueRef]forge.Comment
}

func count(f forge.Forge) *counting {
	return &counting{Forge: f, reads: map[forge.IssueRef]int{}, extra: map[forge.IssueRef]forge.Comment{}}
}

func (c *counting) SearchOpenIssuesWithLabel(ctx context.Context, scope forge.Scope, labels ...string) ([]forge.IssueSummary, error) {
	c.mu.Lock()
	c.searches++
	c.mu.Unlock()
	return c.Forge.SearchOpenIssuesWithLabel(ctx, scope, labels...)
}

func (c *counting) Thread(ctx context.Context, ref forge.IssueRef) (*forge.Issue, []forge.Comment, error) {
	c.mu.Lock()
	c.threads++
	c.mu.Unlock()
	return c.Forge.Thread(ctx, ref)
}

func (c *counting) Comments(ctx context.Context, ref forge.IssueRef) ([]forge.Comment, error) {
	c.mu.Lock()
	c.reads[ref]++
	c.inflight++
	c.maxInflight = max(c.maxInflight, c.inflight)
	hold := c.hold
	c.mu.Unlock()
	time.Sleep(hold)
	cs, err := c.Forge.Comments(ctx, ref)
	c.mu.Lock()
	c.inflight--
	if x, ok := c.extra[ref]; ok && err == nil {
		cs = append(cs, x)
	}
	c.mu.Unlock()
	return cs, err
}

func (c *counting) readsOf(ref forge.IssueRef) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads[ref]
}

// counted is a world whose listing goes through a counting forge, with the inbox's intervals set.
func counted(t *testing.T, mode string) (*world, *counting) {
	t.Helper()
	w := newWorld(t)
	c := count(w.f)
	w.b.Forge = c
	w.b.Config.Inbox = config.Inbox{RefreshInterval: 30 * time.Second, IdleInterval: 30 * time.Minute, ReadMode: mode}
	return w, c
}

func collect(t *testing.T, w *world) ([]Item, []ForgeError) {
	t.Helper()
	return w.b.Collect(ctx)
}

// The listing reads the comments of each issue alone, never the whole thread, and no more than four at a time.
func TestListingReadsCommentsAloneFourAtATime(t *testing.T) {
	w, c := counted(t, config.ReadAlways)
	for n := 30; n < 40; n++ {
		ref := w.f.AddIssue("o", "r", n, "Queued", "body", w.me)
		w.f.AddLabel(ref, "agent:waiting", w.bot)
	}
	c.hold = 20 * time.Millisecond
	items, errs := collect(t, w)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if len(items) != 12 {
		t.Fatalf("items = %d, want 12", len(items))
	}
	if c.threads != 0 {
		t.Errorf("Thread called %d times, want none", c.threads)
	}
	if c.readsOf(w.a) != 1 || c.maxInflight > 4 || c.maxInflight < 2 {
		t.Errorf("reads of a = %d, at most %d at once; want 1, and 2 to 4 at once", c.readsOf(w.a), c.maxInflight)
	}
}

// readTimes lists the listings, every 30 seconds for span, that read the comments of ref, as offsets from the
// first.
func readTimes(t *testing.T, w *world, c *counting, ref forge.IssueRef, span time.Duration, at func(d time.Duration)) []time.Duration {
	t.Helper()
	start := w.now
	var out []time.Duration
	for d := time.Duration(0); d <= span; d += 30 * time.Second {
		w.now = start.Add(d)
		if at != nil {
			at(d)
		}
		before := c.readsOf(ref)
		if _, errs := collect(t, w); len(errs) > 0 {
			t.Fatal(errs)
		}
		if c.readsOf(ref) > before {
			out = append(out, d)
		}
	}
	return out
}

// quietReads are the reads of an issue whose search result never changes, from its first listing: every listing for
// 10 minutes, then after 1, 2, 4, 8 and 16 minutes, then every 30.
func quietReads() []time.Duration {
	var out []time.Duration
	for d := time.Duration(0); d < 10*time.Minute; d += 30 * time.Second {
		out = append(out, d)
	}
	at := 9*time.Minute + 30*time.Second
	for _, gap := range []time.Duration{1, 2, 4, 8, 16, 30, 30} {
		at += gap * time.Minute
		out = append(out, at)
	}
	return out
}

func sameDurations(a, b []time.Duration) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestQuietIssueIsReadLessAndLessOften(t *testing.T) {
	w, c := counted(t, config.ReadAdaptive)
	got := readTimes(t, w, c, w.a, 2*time.Hour, nil)
	if want := quietReads(); !sameDurations(got, want) {
		t.Fatalf("reads at %v, want %v", got, want)
	}
}

// A reply moves the issue in the search: the next listing reads it, and it is read on every listing for 10 minutes.
func TestReplyBringsTheIssueBack(t *testing.T) {
	w, c := counted(t, config.ReadAdaptive)
	got := readTimes(t, w, c, w.a, 40*time.Minute, func(d time.Duration) {
		if d == 30*time.Minute { // between the reads at 24.5m and 40.5m
			w.f.AddComment(w.a, "Go with B.", w.me)
		}
	})
	var want []time.Duration
	for _, d := range quietReads() {
		if d < 30*time.Minute {
			want = append(want, d)
		}
	}
	for d := 30 * time.Minute; d < 40*time.Minute; d += 30 * time.Second {
		want = append(want, d)
	}
	if !sameDurations(got, want) {
		t.Fatalf("reads at %v, want %v", got, want)
	}
}

// A change the search does not show, found by reading the comments, brings the issue back as well.
func TestChangedCommentsBringTheIssueBack(t *testing.T) {
	w, c := counted(t, config.ReadAdaptive)
	got := readTimes(t, w, c, w.a, 30*time.Minute, func(d time.Duration) {
		if d == 24*time.Minute+30*time.Second {
			c.mu.Lock()
			c.extra[w.a] = forge.Comment{ID: 999, Body: "edited", Author: w.me}
			c.mu.Unlock()
		}
	})
	var want []time.Duration
	for _, d := range quietReads() {
		if d <= 24*time.Minute+30*time.Second {
			want = append(want, d)
		}
	}
	for d := 25 * time.Minute; d <= 30*time.Minute; d += 30 * time.Second {
		want = append(want, d)
	}
	if !sameDurations(got, want) {
		t.Fatalf("reads at %v, want %v", got, want)
	}
}

func TestRunningIssueIsReadEveryTime(t *testing.T) {
	w, c := counted(t, config.ReadAdaptive)
	w.f.RemoveLabel(w.a, "agent:waiting")
	w.f.AddLabel(w.a, "agent:running", w.bot)
	got := readTimes(t, w, c, w.a, 20*time.Minute, nil)
	if len(got) != 41 {
		t.Fatalf("reads at %v, want every listing (41)", got)
	}
}

func TestAlwaysReadsEveryTime(t *testing.T) {
	w, c := counted(t, config.ReadAlways)
	got := readTimes(t, w, c, w.a, 60*time.Minute, nil)
	if len(got) != 121 {
		t.Fatalf("reads at %v, want every listing (121)", got)
	}
}

// An issue the search no longer finds is forgotten: when it comes back, it is read at once.
func TestGoneIssueIsForgotten(t *testing.T) {
	w, c := counted(t, config.ReadAdaptive)
	readTimes(t, w, c, w.a, 30*time.Minute, nil)
	w.f.RemoveLabel(w.a, "agent:waiting")
	w.now = w.now.Add(30 * time.Second)
	collect(t, w)
	w.b.stMu.Lock()
	_, kept := w.b.states[forge.GitHub].issues[w.a]
	w.b.stMu.Unlock()
	if kept {
		t.Fatal("the record of an issue the search no longer finds is kept")
	}
}

// A comment read that fails keeps the issue's last row; the forge is reported.
func TestFailedReadKeepsTheLastRow(t *testing.T) {
	w, _ := counted(t, config.ReadAdaptive)
	first, _ := collect(t, w)
	w.now = w.now.Add(30 * time.Second)
	w.f.FailNthComments(1, errors.New("502 bad gateway"))
	items, errs := collect(t, w)
	if len(errs) != 1 || errs[0].Forge != "github" || errs[0].RetryAt != nil {
		t.Fatalf("errors = %+v, want github without a retry time", errs)
	}
	if len(items) != len(first) || items[0].Ref != first[0].Ref || items[1].Question != first[1].Question {
		t.Fatalf("items = %+v, want the last rows %+v", items, first)
	}
}

func TestFailedSearchKeepsTheForgesLastRows(t *testing.T) {
	w, _ := counted(t, config.ReadAdaptive)
	first, _ := collect(t, w)
	w.now = w.now.Add(30 * time.Second)
	w.f.FailSearch = errors.New("502 bad gateway")
	items, errs := collect(t, w)
	if len(errs) != 1 || errs[0].Forge != "github" {
		t.Fatalf("errors = %+v", errs)
	}
	if len(items) != len(first) || items[0].Ref != first[0].Ref {
		t.Fatalf("items = %+v, want the last rows %+v", items, first)
	}
}

func TestFailedSearchRightAfterStartShowsNoRows(t *testing.T) {
	w, _ := counted(t, config.ReadAdaptive)
	w.f.FailSearch = errors.New("502 bad gateway")
	items, errs := collect(t, w)
	if len(items) != 0 || len(errs) != 1 {
		t.Fatalf("items = %+v, errors = %+v", items, errs)
	}
}

// After a rate limit, the listing leaves the forge alone until the reset, with the last rows and the reset time;
// a detail and a reply still go to the forge.
func TestRateLimitedForgeIsLeftAloneUntilTheReset(t *testing.T) {
	w, c := counted(t, config.ReadAdaptive)
	first, _ := collect(t, w)
	reset := w.now.Add(20 * time.Minute)
	w.now = w.now.Add(30 * time.Second)
	w.f.FailSearch = &forge.RateLimitError{Reset: reset, Err: forge.ErrForbidden}
	items, errs := collect(t, w)
	if len(errs) != 1 || errs[0].RetryAt == nil || !errs[0].RetryAt.Equal(reset) || len(items) != len(first) {
		t.Fatalf("items = %d, errors = %+v; want %d rows and a retry at %v", len(items), errs, len(first), reset)
	}

	searches := c.searches
	w.now = w.now.Add(10 * time.Minute)
	items, errs = collect(t, w)
	if c.searches != searches || len(items) != len(first) || len(errs) != 1 || errs[0].RetryAt == nil {
		t.Fatalf("while limited: searches %d -> %d, items %d, errors %+v", searches, c.searches, len(items), errs)
	}
	threads := c.threads
	if _, err := w.b.Detail(ctx, w.a); err != nil {
		t.Fatal(err)
	}
	if err := w.b.Reply(ctx, w.a, "Go with B.", false); err != nil {
		t.Fatal(err)
	}
	if c.threads == threads {
		t.Error("a detail and a reply did not go to the forge while limited")
	}

	w.now = reset.Add(time.Second)
	if _, errs := collect(t, w); len(errs) != 0 || c.searches != searches+1 {
		t.Fatalf("after the reset: searches %d -> %d, errors %+v", searches, c.searches, errs)
	}
}
