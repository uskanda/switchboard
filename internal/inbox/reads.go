package inbox

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// concurrentReads is how many issues a listing reads at once: few enough to stay clear of GitHub's secondary rate
// limit on concurrent requests.
const concurrentReads = 4

// recentChange is how long after a change an issue is read on every listing.
const recentChange = 10 * time.Minute

// policy is how a listing decides which issues to read (config.Inbox).
type policy struct {
	always        bool
	refresh, idle time.Duration
}

// readPolicy is the configured policy, with the defaults of an unloaded configuration (tests build one by hand).
func (b *Inbox) readPolicy() policy {
	in := b.Config.Inbox
	p := policy{always: in.ReadMode == config.ReadAlways, refresh: in.RefreshInterval, idle: in.IdleInterval}
	if p.refresh <= 0 {
		p.refresh = 30 * time.Second
	}
	if p.idle < p.refresh {
		p.idle = max(p.refresh, 30*time.Minute)
	}
	return p
}

// issueState is what the listings know of one issue.
type issueState struct {
	key      string          // the search result's update time, comment count and labels when last seen
	changed  time.Time       // when a change was last seen
	read     time.Time       // when the comments were last read
	gap      time.Duration   // how long after read the next read is due, once the change is no longer recent
	comments []forge.Comment // the comments last read
	ok       bool            // the comments were read at least once
}

// forgeState is what the listings know of one forge.
type forgeState struct {
	issues map[forge.IssueRef]*issueState
	items  []Item    // the rows of the last listing
	until  time.Time // the forge is not asked before this, after a rate limit
}

// state is the record of a forge kind, made on first use. The listing of one kind runs once at a time (Collect
// runs one per kind), so the record itself needs no lock.
func (b *Inbox) state(kind string) *forgeState {
	b.stMu.Lock()
	defer b.stMu.Unlock()
	if b.states == nil {
		b.states = map[string]*forgeState{}
	}
	st, ok := b.states[kind]
	if !ok {
		st = &forgeState{issues: map[forge.IssueRef]*issueState{}}
		b.states[kind] = st
	}
	return st
}

func (st *forgeState) limitedUntil() time.Time { return st.until }

func (st *forgeState) limit(until time.Time) {
	if until.After(st.until) {
		st.until = until
	}
}

func (st *forgeState) lastItems() []Item { return slices.Clone(st.items) }

// keyOf is what the search tells of an issue's changes.
func keyOf(s forge.IssueSummary) string {
	ls := slices.Clone(s.Labels)
	slices.Sort(ls)
	return s.UpdatedAt.UTC().Format(time.RFC3339Nano) + "|" + strconv.Itoa(s.Notes) + "|" + strings.Join(ls, ",")
}

// plan says which listed issues to read now: one new to the listings, one whose search result changed, a running
// one, one changed within recentChange, and one whose gap has passed; every one under ReadAlways.
func (st *forgeState) plan(listed []forge.IssueSummary, now time.Time, p policy, running func(forge.IssueSummary) bool) []bool {
	out := make([]bool, len(listed))
	for i, s := range listed {
		is := st.issues[s.Ref]
		switch {
		case p.always, is == nil, !is.ok, is.key != keyOf(s), running(s), now.Sub(is.changed) < recentChange:
			out[i] = true
		default:
			// Past the recent change, the first gap is twice the refresh interval. Half an interval of slack keeps a
			// listing that comes a little early from putting the read off by a whole interval.
			is.gap = max(is.gap, 2*p.refresh)
			out[i] = now.Sub(is.read) >= is.gap-p.refresh/2
		}
	}
	return out
}

// record takes what the listing did with one issue and returns the comments its row is built from, false when it
// was never read. A change seen in the search or in the comments brings the issue back to every listing; a read of
// a quiet issue that found nothing new doubles its gap, up to the idle interval.
func (st *forgeState) record(s forge.IssueSummary, now time.Time, p policy, planned bool, comments []forge.Comment, err error) ([]forge.Comment, bool) {
	is := st.issues[s.Ref]
	if is == nil {
		is = &issueState{changed: now, gap: p.refresh}
		st.issues[s.Ref] = is
	}
	key := keyOf(s)
	if is.key != key {
		if is.key != "" {
			is.changed, is.gap = now, p.refresh
		}
		is.key = key
	}
	if !planned || err != nil {
		return is.comments, is.ok
	}
	switch {
	case is.ok && !sameComments(is.comments, comments):
		is.changed, is.gap = now, p.refresh
	case is.gap >= 2*p.refresh: // a read of a quiet issue (plan raised its gap) that found nothing new
		is.gap = min(is.gap*2, p.idle)
	}
	is.read, is.comments, is.ok = now, comments, true
	return is.comments, true
}

// keep records the rows of a listing and forgets the issues the search no longer finds.
func (st *forgeState) keep(listed []forge.IssueSummary, items []Item) {
	seen := map[forge.IssueRef]bool{}
	for _, s := range listed {
		seen[s.Ref] = true
	}
	for ref := range st.issues {
		if !seen[ref] {
			delete(st.issues, ref)
		}
	}
	st.items = slices.Clone(items)
}

func sameComments(a, b []forge.Comment) bool {
	return slices.EqualFunc(a, b, func(x, y forge.Comment) bool {
		return x.ID == y.ID && x.Body == y.Body && x.Author.ID == y.Author.ID
	})
}
