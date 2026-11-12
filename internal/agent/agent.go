// Package agent is the loop behind `switchboard agent run`. It finds the issues of this host's projects that are
// ready or answered, runs claude on each in its own worktree, and writes the outcome back to the forge. Every
// write to the forge and every push is done here, never by claude (ADR-011, ADR-012).
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
	"github.com/uskanda/switchboard/internal/workspace"
)

// Agent runs claude on the issues of the projects assigned to this host.
type Agent struct {
	Config *config.Config
	// Forge, Judge and GitToken serve the projects without a route.
	Forge forge.Forge
	Judge *trust.Judge
	// Routes gives a project, keyed by its ref ("gl:nas/sb"), its own forge client, judge and push token: another
	// forge, or the token of an overridden agent_identity.
	Routes map[string]Route
	State  state.Dir
	Audit  state.Auditor
	// Env is the environment git and claude are started from; claude gets only its allowlisted part.
	// os.Environ() when nil.
	Env []string
	// GitToken is offered to git push for https remotes. It never reaches claude.
	GitToken string
	// Log receives one line per notable event; io.Discard when nil.
	Log io.Writer
	// IntGrace and TermGrace override how long a stopping run gets after SIGINT and after SIGTERM.
	IntGrace, TermGrace time.Duration
	// Relay, when set, is the relay's event stream: an event makes the agent fetch its project again, and while
	// connected the agent polls every reconcile_interval instead of every poll_interval.
	Relay Subscriber
	// Debounce is how long the events of one project are gathered before it is fetched; 3s when zero.
	Debounce time.Duration
	// ProgressEvery is how often the progress comment of a run is brought up to date; a minute when zero.
	ProgressEvery time.Duration
	// Notifier, when set, is told each time the agent posts a waiting or a done comment.
	Notifier Notifier
	// NewRoute, when set, builds the route of a project that appears on a reload; without it, such a project uses
	// the agent's defaults (Forge, Judge, GitToken).
	NewRoute func(config.Project) (Route, error)
	// CloneURL is the URL a project that appears on a reload is cloned from; the forge's https URL when nil.
	CloneURL func(config.Project) string
	// PushDelays are the waits between the tries of a push that failed for a moment; 5s and 20s when nil.
	PushDelays []time.Duration
	// Now is the clock of the CI watch; time.Now when nil.
	Now func() time.Time
	// CheckSandbox reports whether the sandbox of a run can work; it is called before every start of claude, and a
	// run whose check fails ends in an error. claudecode.CheckSandbox with bwrap from PATH when nil.
	CheckSandbox func(context.Context) error
	// FindBrowser finds the browser switchboard shot runs for a plan run's pictures; shot.FindBrowser with
	// agent.browser when nil. A plan run is told about pictures only when it finds one (ADR-069).
	FindBrowser func() (string, error)
	// Executable is the switchboard binary a plan run calls for its pictures; os.Executable when empty.
	Executable string
	// GHAttaches reports whether gh has --attach, which a GitHub plan run's pictures need (ADR-071); when nil, gh from
	// PATH is asked for its version once.
	GHAttaches func() bool
	ghOnce     sync.Once
	ghAttaches bool

	ctx    context.Context // runs end when it does
	unlock func()
	msg    texts

	// pmu guards the projects and routes, which a reload changes while runs and relay refreshes read them.
	pmu           sync.RWMutex
	projects      []config.Project
	private       map[string]bool           // by project ref
	desired       []config.Project          // this host's projects of the last reload, until applied
	desiredMax    int                       // agent.max_parallel of the last reload, until applied
	desiredPrefer bool                      // agent.prefer_plan of the last reload, until applied
	reloaded      bool                      // desired holds a reload not applied yet
	dropped       []forge.IssueRef          // queued issues of removed projects, whose queued comments are to go
	accepting     map[string]config.Project // by ref: projects of a reload not yet cloned and labeled
	failures      map[string]string         // by ref: the last acceptance failure logged

	mu          sync.Mutex
	maxParallel int  // agent.max_parallel, which a reload changes
	preferPlan  bool // agent.prefer_plan, which a reload changes
	active      map[forge.IssueRef]*active
	queued      map[forge.IssueRef]int64 // the queued comment of each queued issue; 0 while it is being posted
	pending     map[forge.IssueRef]queuedRun
	seen        map[forge.IssueRef]time.Time
	// unsettled holds the issues whose evaluation did not finish (an error, a queue, a lock), so that since stays
	// at or before their update time and they are evaluated again (#27).
	unsettled map[forge.IssueRef]unsettledIssue
	// writebacks holds the runs of this host whose write-back did not finish; each poll tries them again (#26).
	writebacks map[forge.IssueRef]*pendingWriteBack
	notes      map[forge.IssueRef]int  // GitLab: the comment count of each waiting issue at its last evaluation
	closed     map[forge.IssueRef]bool // closed issues whose worktree waits for their run to stop
	// watches holds the done issues whose CI the agent watches; they are found again from the done posts.
	watches map[forge.IssueRef]*watch
	// unwatched is the update time of each done issue a scan found nothing to watch on, so that it is not read again.
	unwatched map[forge.IssueRef]time.Time
	scanned   time.Time       // when the done issues were last searched for watches
	forbidden map[string]bool // by project ref: the CI could not be read, already logged
	// paused, when set, is the pause of this host on a usage limit or its API quota (ADR-053); pauseTimer takes the
	// queue up again when it ends. cost is what the runs cost today and this month.
	paused     *pause
	pauseTimer *time.Timer
	cost       costs
	// resumes marks the queued issues whose queued comment is the resume mark of a run that hit a usage limit: the
	// comment stays over a restart, and goes when the issue starts or leaves the queue.
	resumes map[forge.IssueRef]bool
	wg      sync.WaitGroup

	pollMu     sync.Mutex // one poll or relay refresh at a time
	rmu        sync.Mutex
	connected  bool                   // the relay stream is up
	timers     map[string]*time.Timer // by project ref: the pending refresh of a project with events
	wake       chan struct{}          // the relay connected or was lost
	tune       chan struct{}          // the interval may have changed: something to try again appeared
	reconciled time.Time              // when every project was last fetched; guarded by rmu
}

// Route is what one project uses to reach its forge.
type Route struct {
	Forge    forge.Forge
	Judge    *trust.Judge
	GitToken string // offered to git push for https remotes; never given to claude
}

// route is the issue's route: its project's, or the agent's defaults.
func (a *Agent) route(ref forge.IssueRef) Route {
	a.pmu.RLock()
	r, ok := a.Routes[ref.Kind()+":"+ref.Project()]
	a.pmu.RUnlock()
	if ok {
		return r
	}
	return Route{Forge: a.Forge, Judge: a.Judge, GitToken: a.GitToken}
}

// projectList is a copy of the projects the agent handles now.
func (a *Agent) projectList() []config.Project {
	a.pmu.RLock()
	defer a.pmu.RUnlock()
	return slices.Clone(a.projects)
}

// handles reports whether p is still one of the projects the agent takes issues of.
func (a *Agent) handles(p config.Project) bool {
	a.pmu.RLock()
	defer a.pmu.RUnlock()
	for _, q := range a.projects {
		if q.Ref == p.Ref {
			return true
		}
	}
	return false
}

// isPrivate reports whether the project's repository was private when the agent took the project on.
func (a *Agent) isPrivate(ref string) bool {
	a.pmu.RLock()
	defer a.pmu.RUnlock()
	return a.private[ref]
}

func (a *Agent) routeOf(p config.Project) Route {
	return a.route(forge.IssueRef{Forge: p.Forge, Owner: p.Owner, Repo: p.Repo})
}

func (a *Agent) forgeOf(ref forge.IssueRef) forge.Forge  { return a.route(ref).Forge }
func (a *Agent) judgeOf(ref forge.IssueRef) *trust.Judge { return a.route(ref).Judge }

type active struct {
	cancel  context.CancelFunc
	run     string
	started time.Time
}

// unsettledIssue is an issue to evaluate again: its update time when last seen, and whether it only waits for a
// slot (drain takes it up; it does not shorten the poll interval).
type unsettledIssue struct {
	at     time.Time
	queued bool
}

// errQueued reports that an issue waits for a free slot; it is evaluated again when one frees up.
var errQueued = errors.New("queued")

// lockName joins the forge, the owner, the repository and the number with "~", which neither forge allows in a
// name, and replaces the slashes of GitLab subgroups so that the lock stays one file. A repository name has no
// slash, so the last two parts are always the repository and the number, and two issues never share a name.
func lockName(ref forge.IssueRef) string {
	return fmt.Sprintf("%s~%s~%s~%d", ref.Kind(), strings.ReplaceAll(ref.Owner, "/", "~"), ref.Repo, ref.Number)
}

func (a *Agent) logf(format string, args ...any) {
	fmt.Fprintf(a.Log, "switchboard: "+format+"\n", args...)
}

func (a *Agent) swapper(ref forge.IssueRef) labels.Swapper {
	return labels.Swapper{Forge: a.forgeOf(ref), Names: a.Config.Labels, Audit: a.Audit}
}

// Start takes the agent lock of the state directory, checks the projects of this host, and recovers the runs an
// earlier agent left behind. Runs started later end when ctx ends.
func (a *Agent) Start(ctx context.Context) error {
	if a.Log == nil {
		a.Log = io.Discard
	}
	if a.Env == nil {
		a.Env = os.Environ()
	}
	if err := a.State.Ensure(); err != nil {
		return err
	}
	unlock, err := a.State.Lock("agent")
	if errors.Is(err, state.ErrLocked) {
		return fmt.Errorf("another switchboard agent is running with the state directory %s", a.State.Path)
	}
	if err != nil {
		return err
	}
	a.unlock = unlock
	a.ctx = ctx
	a.msg = textsFor(a.Config.Comments.Language)
	a.maxParallel = a.Config.Agent.MaxParallel
	a.preferPlan = a.Config.Agent.PreferPlan
	a.active = map[forge.IssueRef]*active{}
	a.queued = map[forge.IssueRef]int64{}
	a.pending = map[forge.IssueRef]queuedRun{}
	a.seen = map[forge.IssueRef]time.Time{}
	a.unsettled = map[forge.IssueRef]unsettledIssue{}
	a.writebacks = map[forge.IssueRef]*pendingWriteBack{}
	a.notes = map[forge.IssueRef]int{}
	a.closed = map[forge.IssueRef]bool{}
	a.watches = map[forge.IssueRef]*watch{}
	a.unwatched = map[forge.IssueRef]time.Time{}
	a.forbidden = map[string]bool{}
	a.resumes = map[forge.IssueRef]bool{}
	a.paused, a.cost = nil, costs{}
	a.scanned = time.Time{}
	a.wake = make(chan struct{}, 1)
	a.tune = make(chan struct{}, 1)
	a.pmu.Lock()
	a.private = map[string]bool{}
	a.accepting = map[string]config.Project{}
	a.failures = map[string]string{}
	a.projects = nil
	a.pmu.Unlock()
	if err := a.State.WriteJSON("agent.json", agentInfo{PID: os.Getpid(), Host: a.Config.Host.Name, Started: time.Now().UTC()}); err != nil {
		return err
	}
	for _, p := range a.Config.ProjectsFor(a.Config.Host.Name) {
		private, err := a.routeOf(p).Forge.RepoPrivate(ctx, p.Owner, p.Repo)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// One project the forge does not answer for keeps the others running: it waits for acceptance, as a
			// project of a reload does, and is tried again on each poll.
			a.logf("%s: read the repository's visibility: %v; tried again on the next poll", p.FullName(), err)
			a.pmu.Lock()
			a.accepting[p.Ref] = p
			a.failures[p.Ref] = err.Error()
			a.pmu.Unlock()
			continue
		}
		if !a.allowed(p, private) {
			continue
		}
		a.pmu.Lock()
		a.private[p.Ref] = private
		a.projects = append(a.projects, p)
		a.pmu.Unlock()
		// No run goes yet, so a sandbox placeholder for the clone's config.lock is one a killed run left (#128).
		if removed, err := workspace.ClearConfigLock(ctx, p.Path); err != nil {
			a.logf("%s: check the config.lock of %s: %v", p.FullName(), p.Path, err)
		} else if removed {
			a.logf("%s: removed the sandbox placeholder config.lock that a killed run left in %s", p.FullName(), p.Path)
		}
	}
	a.loadLimits()
	a.clearQueued(ctx)
	if err := a.recover(ctx); err != nil {
		return err
	}
	a.scanWatches(ctx)
	a.pruneRuns(time.Now())
	return nil
}

// Close releases the agent lock. Call it after Wait.
func (a *Agent) Close() {
	a.stopPauseTimer()
	if a.unlock != nil {
		_ = os.Remove(filepath.Join(a.State.Path, "agent.json"))
		a.unlock()
		a.unlock = nil
	}
}

// Wait waits for every run, including the runs that finishing runs start.
func (a *Agent) Wait() { a.wg.Wait() }

// Run starts the agent and polls every poll_interval until ctx ends; then it waits for the runs to stop.
func (a *Agent) Run(ctx context.Context) error {
	if err := a.Start(ctx); err != nil {
		return err
	}
	defer a.Close()
	defer a.Wait()
	a.logf("polling %d project(s) every %s", len(a.projectList()), a.Config.Agent.PollInterval)
	if a.Relay != nil {
		defer a.stopRefreshes()
		go a.Relay.Run(ctx, a.onEvent, a.onRelayState)
	}
	cur := a.interval()
	t := time.NewTicker(cur)
	defer t.Stop()
	// retune resets the ticker when the interval changed: something to try again appeared or was done.
	retune := func() {
		if d := a.interval(); d != cur {
			cur = d
			t.Reset(d)
		}
	}
	prune := time.NewTicker(pruneEvery)
	defer prune.Stop()
	next := a.projectList // what the coming poll fetches, decided after a reload is applied
	for {
		a.applyProjects(ctx)
		if err := a.poll(ctx, next()); err != nil && ctx.Err() == nil {
			a.logf("poll: %v", err)
		}
		retune()
	wait:
		for {
			select {
			case <-ctx.Done():
				return nil
			case now := <-t.C:
				next = func() []config.Project { return a.due(now) }
				break wait
			case <-a.wake: // poll every project at once, then at the interval of the new state
				cur = a.interval()
				t.Reset(cur)
				next = a.projectList
				break wait
			case <-a.tune:
				retune()
			case now := <-prune.C:
				a.pruneRuns(now)
			}
		}
	}
}

// Poll applies a pending reload, evaluates the issues updated since the last poll, then starts queued issues while
// slots are free.
func (a *Agent) Poll(ctx context.Context) error {
	a.applyProjects(ctx)
	return a.poll(ctx, a.projectList())
}

// poll is Poll for the projects ps. Fetching every project counts as a reconcile.
func (a *Agent) poll(ctx context.Context, ps []config.Project) error {
	a.pollMu.Lock()
	defer a.pollMu.Unlock()
	a.retryWriteBacks(ctx)
	a.startFixes(ctx) // before a newly ready issue takes a free slot
	var errs []error
	for _, p := range ps {
		errs = append(errs, a.pollProject(ctx, p)...)
	}
	if len(ps) == len(a.projectList()) {
		a.rmu.Lock()
		a.reconciled = time.Now()
		a.rmu.Unlock()
	}
	a.cleanClosed(ctx)
	a.drain(ctx)
	a.checkWatches(ctx)
	return errors.Join(errs...)
}

// pollProject evaluates the issues of p updated since the last poll and, on GitLab, the waiting issues with new notes.
func (a *Agent) pollProject(ctx context.Context, p config.Project) []error {
	f := a.routeOf(p).Forge
	list, err := f.UpdatedIssues(ctx, p.Owner, p.Repo, a.since(p))
	if err != nil {
		return []error{fmt.Errorf("%s: %w", p.Ref, err)}
	}
	var errs []error
	listed := map[forge.IssueRef]bool{}
	for _, s := range list {
		listed[s.Ref] = true
		a.mu.Lock()
		last, ok := a.seen[s.Ref]
		_, again := a.unsettled[s.Ref]
		a.mu.Unlock()
		if ok && !s.UpdatedAt.After(last) && !again {
			continue
		}
		if err := a.evaluate(ctx, p, s); err != nil {
			errs = append(errs, err)
		}
	}
	a.forgetMissing(p, listed)
	if p.Forge == forge.GitLab {
		errs = append(errs, a.pollReplies(ctx, p, f)...)
	}
	return errs
}

// pollReplies evaluates the waiting and done issues of a GitLab project whose comment count changed. GitLab does not
// move an issue's update time for a new comment (V22), so UpdatedIssues alone misses replies.
func (a *Agent) pollReplies(ctx context.Context, p config.Project, f forge.Forge) []error {
	waiting, err := f.SearchOpenIssuesWithLabel(ctx, forge.Projects(p.FullName()), a.Config.Labels.Waiting, a.Config.Labels.Done)
	if err != nil {
		return []error{fmt.Errorf("%s: %w", p.Ref, err)}
	}
	var errs []error
	for _, s := range waiting {
		a.mu.Lock()
		n, ok := a.notes[s.Ref]
		a.mu.Unlock()
		if ok && n == s.Notes {
			continue
		}
		if err := a.evaluate(ctx, p, s); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// evaluate decides what an updated issue needs and records it as seen, unless it has to be evaluated again later:
// then it is kept unsettled, which holds since back for it.
func (a *Agent) evaluate(ctx context.Context, p config.Project, s forge.IssueSummary) error {
	err := a.evaluateOnce(ctx, p, s)
	gone := errors.Is(err, forge.ErrNotFound)
	if err == nil || gone {
		a.mu.Lock()
		if prev, ok := a.seen[s.Ref]; !ok || s.UpdatedAt.After(prev) {
			a.seen[s.Ref] = s.UpdatedAt
		}
		a.notes[s.Ref] = s.Notes
		delete(a.unsettled, s.Ref)
		a.mu.Unlock()
		if gone {
			a.logf("%s: the forge no longer has the issue; it is not evaluated again", s.Ref)
		}
		return nil
	}
	a.unsettle(s.Ref, s.UpdatedAt, errors.Is(err, errQueued))
	if errors.Is(err, errQueued) || errors.Is(err, state.ErrLocked) {
		return nil // evaluated again later
	}
	return fmt.Errorf("%s: %w", s.Ref, err)
}

// unsettle keeps ref to be evaluated again, as updated at at. queued says it only waits for a slot.
func (a *Agent) unsettle(ref forge.IssueRef, at time.Time, queued bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if u, ok := a.unsettled[ref]; ok && u.at.After(at) {
		at = u.at
	}
	a.unsettled[ref] = unsettledIssue{at: at, queued: queued}
}

// forgetMissing stops evaluating again the unsettled issues of p that the forge no longer lists: since never passed
// their update time, so a listing without them means they were deleted or moved.
func (a *Agent) forgetMissing(p config.Project, listed map[forge.IssueRef]bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for ref := range a.unsettled {
		if inProject(ref, p) && !listed[ref] && a.active[ref] == nil {
			delete(a.unsettled, ref)
			delete(a.pending, ref)
			a.logf("%s: the forge no longer lists the issue; it is not evaluated again", ref)
		}
	}
}

// inProject reports whether ref is an issue of p. The forges treat owner and repository names without regard to
// case, and may return them in another case than the configuration's.
func inProject(ref forge.IssueRef, p config.Project) bool {
	return ref.Kind() == p.Forge && strings.EqualFold(ref.Owner, p.Owner) && strings.EqualFold(ref.Repo, p.Repo)
}

// since is a minute before the latest update seen in the project, so that a clock skew between the forge and
// this host loses nothing. It never passes an unsettled issue, which is then listed again. The value moves only when
// something changes, which keeps conditional requests cheap.
func (a *Agent) since(p config.Project) time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	var latest time.Time
	for ref, t := range a.seen {
		if inProject(ref, p) && t.After(latest) {
			latest = t
		}
	}
	if latest.IsZero() {
		return latest
	}
	for ref, u := range a.unsettled {
		if inProject(ref, p) && u.at.Before(latest) {
			latest = u.at
		}
	}
	return latest.Add(-time.Minute)
}

func (a *Agent) isActive(ref forge.IssueRef) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.active[ref] != nil
}

// evaluateOnce decides what an updated issue needs: cancel its run, start a run, clean up its worktree, or nothing.
func (a *Agent) evaluateOnce(ctx context.Context, p config.Project, s forge.IssueSummary) error {
	n := a.Config.Labels
	st := labels.StateOf(s.Labels, n)
	if s.State != "open" || st != n.Done {
		a.unwatch(s.Ref, "the issue is no longer done")
	}
	if a.isActive(s.Ref) {
		if s.State == "open" && st == n.Running {
			return nil
		}
		if s.State != "open" {
			a.mu.Lock()
			a.closed[s.Ref] = true
			a.mu.Unlock()
		}
		return a.maybeCancel(ctx, s.Ref)
	}
	if s.State != "open" {
		a.leaveQueue(ctx, s.Ref)
		a.cleanUp(ctx, p, s.Ref)
		return nil
	}
	if fixed, changed := labels.Normalize(s.Labels, n); changed {
		if err := a.forgeOf(s.Ref).SetLabels(ctx, s.Ref, fixed); err != nil {
			return err
		}
		a.Audit.Log("labels.normalized", map[string]any{"issue": s.Ref.String(), "found": s.Labels, "kept": labels.StateOf(fixed, n)})
		st = labels.StateOf(fixed, n)
	}
	if !labels.IsStart(st, n) && st != n.Waiting && st != n.Done {
		a.leaveQueue(ctx, s.Ref)
		return nil
	}
	return a.consider(ctx, p, s.Ref)
}

// maybeCancel stops the run of ref when the issue, read again, is closed or no longer carries the running label.
func (a *Agent) maybeCancel(ctx context.Context, ref forge.IssueRef) error {
	is, _, err := a.forgeOf(ref).Thread(ctx, ref)
	if err != nil {
		return err
	}
	reason := ""
	switch {
	case is.State != "open":
		reason = "the issue was closed"
	case labels.StateOf(is.Labels, a.Config.Labels) != a.Config.Labels.Running:
		reason = "the running label was removed"
	default:
		return nil
	}
	a.mu.Lock()
	act := a.active[ref]
	run := ""
	if act != nil {
		run = act.run // a fix run that takes over a run changes it
	}
	a.mu.Unlock()
	if act == nil {
		return nil
	}
	a.logf("%s: stopping run %s: %s", ref, run, reason)
	a.Audit.Log("run.cancel", map[string]any{"issue": ref.String(), "run": run, "reason": reason})
	act.cancel()
	return nil
}

// consider reads the issue again and starts a run when it should run.
func (a *Agent) consider(ctx context.Context, p config.Project, ref forge.IssueRef) error {
	pl, err := a.decide(ctx, p, ref)
	if err != nil {
		return err
	}
	if pl == nil {
		a.leaveQueue(ctx, ref)
		return nil
	}
	return a.launch(ctx, pl)
}

// plan is an issue that should run, as read when that was decided.
type plan struct {
	project  config.Project
	issue    *forge.Issue
	comments []forge.Comment
	kinds    []trust.Kind

	start        string   // the start label the run starts from, or "" when it resumes on a reply or after a limit
	mode         string   // with start: the mode its claim records
	resumeNote   int64    // the queued comment that marks the run to resume after a usage limit, or 0
	kind         runKind  // set by classify
	planID, okID int64    // for approvedRun: the plan and the comment that approved it; for skippedRun: the plan
	qNext        int      // for planRun: the number of its first question
	skippable    bool     // for planRun: it may skip the questions and the approval (the first plan run of an auto cycle)
	fix          *fixInfo // for fixRun: what it fixes
}

// ack is the last comment the run takes in, or 0. The agent's transient posts (claims, progress, queued comments,
// attachments) are passed over: two hosts that read the issue on either side of the other's claim still name the same
// trigger, so the claim rule finds one winner.
func (pl *plan) ack() int64 {
	for i := len(pl.comments) - 1; i >= 0; i-- {
		if i < len(pl.kinds) && pl.kinds[i] == trust.AgentPost {
			if m, _ := marker.Parse(pl.comments[i].Body); m != nil && m.Transient() {
				continue
			}
		}
		return pl.comments[i].ID
	}
	return 0
}

// decide returns a plan when the issue should run now: ready and labeled by a trusted user, or waiting with a
// trusted reply after the agent's question. The title and body must be trustworthy in both cases.
func (a *Agent) decide(ctx context.Context, p config.Project, ref forge.IssueRef) (*plan, error) {
	n := a.Config.Labels
	j := a.judgeOf(ref)
	is, comments, err := a.forgeOf(ref).Thread(ctx, ref)
	if err != nil {
		return nil, err
	}
	if is.State != "open" {
		return nil, nil
	}
	st := labels.StateOf(is.Labels, n)
	byAgent := false // the agent put the start label back after a run hit a usage limit
	var actor *forge.User
	switch {
	case labels.IsStart(st, n):
		actor, err = a.forgeOf(ref).LabelActor(ctx, ref, st)
		if errors.Is(err, forge.ErrNotFound) {
			actor = nil
		} else if err != nil {
			return nil, err
		}
		byAgent = actor != nil && actor.ID == j.AgentIDOf(ref)
		if !byAgent && !j.ReadyLabel(ref, actor) {
			return nil, nil
		}
	case st == n.Waiting, st == n.Done:
	default:
		return nil, nil
	}
	pl := &plan{project: p, issue: is, comments: comments}
	for _, c := range comments {
		pl.kinds = append(pl.kinds, j.Comment(ref, c))
	}
	if byAgent {
		// Only the agent's own resume mark lets a start label it put start a run. The run goes on with the cycle the
		// limited run was in, as a resume on a reply does (ADR-053). Without the mark, the label is a person's when the
		// agent's account is also a trusted user's, which the forge cannot tell apart (ADR-056). With the web inbox's
		// login on, people and the agent post as different accounts, and the exception does not hold (issue #146).
		if pl.resumeNote = pl.limitResume(); pl.resumeNote == 0 {
			if a.Config.Inbox.Auth != nil || !j.ReadyLabel(ref, actor) {
				return nil, nil
			}
			byAgent = false
		}
	}
	if byAgent {
		pl.classify(a.Config.Plan.ApproveWord)
		if !j.Body(is) {
			return nil, nil
		}
		return pl, nil
	}
	if labels.IsStart(st, n) {
		if labels.MultipleStarts(is.Labels, n) {
			// Report the clash only when every start label came from a trusted user.
			for _, l := range is.Labels {
				if !labels.IsStart(l, n) || l == st {
					continue
				}
				actor, err := a.forgeOf(ref).LabelActor(ctx, ref, l)
				if err != nil && !errors.Is(err, forge.ErrNotFound) {
					return nil, err
				}
				if err != nil || !j.ReadyLabel(ref, actor) {
					return nil, nil
				}
			}
			pl.kind = conflictRun
		} else {
			pl.start, pl.mode = st, a.modeFor(p, st)
		}
	}
	pl.classify(a.Config.Plan.ApproveWord)
	if labels.IsStart(st, n) {
		// A person put a start label on an issue a limit put back in the queue: a new cycle, which still takes the
		// limited run's worktree on and removes its resume mark.
		pl.resumeNote = pl.limitResume()
	}
	if (st == n.Waiting || st == n.Done) && !pl.hasReply() {
		return nil, nil
	}
	if !j.Body(is) {
		return nil, nil
	}
	return pl, nil
}

// hasReply reports whether a trusted comment follows the comment acknowledged by the agent's latest post, and that
// post is a question (the issue is waiting because the agent made it wait) or a done (the issue stays open for more
// work until it is closed, ADR-042).
func (pl *plan) hasReply() bool {
	var m *marker.Marker
	for i := len(pl.comments) - 1; i >= 0 && m == nil; i-- {
		if pl.kinds[i] == trust.AgentPost {
			if pm, _ := marker.Parse(pl.comments[i].Body); pm != nil && !pm.Transient() {
				m = pm // a claim or the progress of a run is not the question the issue waits on
			}
		}
	}
	if m == nil || (m.State != "waiting" && m.State != "done") {
		return false
	}
	at := -1
	for i, c := range pl.comments {
		if c.ID == m.Ack {
			at = i
		}
	}
	for i, c := range pl.comments {
		after := i > at
		if at < 0 {
			after = c.ID > m.Ack
		}
		if after && pl.kinds[i] == trust.Instruction {
			return true
		}
	}
	return false
}

// launch starts the run of pl unless the issue already runs, every slot is taken (errQueued), or another process
// holds the issue's lock (state.ErrLocked). Call it with pollMu held: every caller (a poll, a relay refresh, afterRun) does.
func (a *Agent) launch(ctx context.Context, pl *plan) error {
	ref := pl.issue.Ref
	limit := a.projectLimit(ref)
	a.mu.Lock()
	if a.active[ref] != nil {
		a.mu.Unlock()
		return nil
	}
	if ok, limitKind, value := a.hasSlot(ref, limit); !ok {
		if pl.fix != nil {
			a.mu.Unlock()
			return errQueued // the watch keeps the fix run waiting for startFixes; a done issue is never queued
		}
		// An issue already queued keeps its place; its kind may have changed meanwhile (a plan approved).
		q, ok := a.pending[ref]
		if !ok {
			q.since = time.Now()
		}
		q.project, q.plan = pl.project, pl.kind == planRun
		a.pending[ref] = q
		_, noted := a.queued[ref]
		switch {
		case !noted && pl.resumeNote != 0:
			// The resume mark of a run that hit a limit serves as the queued comment.
			a.queued[ref], a.resumes[ref], noted = pl.resumeNote, true, true
			a.saveQueued()
		case !noted:
			a.queued[ref] = 0
		}
		a.mu.Unlock()
		a.unsettle(ref, pl.issue.UpdatedAt, true)
		if !noted {
			a.noteQueued(ctx, ref, limitKind, value)
		}
		return errQueued
	}
	unlockIssue, err := a.State.Lock(lockName(ref))
	if err != nil {
		a.mu.Unlock()
		if errors.Is(err, state.ErrLocked) {
			a.logf("%s: another process holds the issue; skipped", ref)
		}
		return err
	}
	runCtx, cancel := context.WithCancel(a.ctx)
	act := &active{cancel: cancel, run: newID(), started: time.Now().UTC()}
	a.active[ref] = act
	delete(a.pending, ref)
	a.wg.Add(1)
	a.mu.Unlock()

	finish := func() {
		cancel()
		unlockIssue()
		a.mu.Lock()
		delete(a.active, ref)
		a.mu.Unlock()
	}
	if a.unqueue(ctx, ref) {
		a.logf("%s: taken up from the queue", ref)
		pl.dropQueued()
	} else if pl.resumeNote != 0 {
		a.removeQueued(ctx, ref, pl.resumeNote)
		pl.dropQueued()
	}
	// Claim first: a host that loses to another one leaves without touching the labels.
	won, err := a.claim(ctx, pl, act)
	if err != nil || !won {
		finish()
		a.wg.Done()
		return err
	}
	if err := a.swapper(ref).Swap(ctx, ref, a.Config.Labels.Running); err != nil {
		finish()
		a.wg.Done()
		if ctx.Err() == nil {
			// The label may be set although the swap failed: the next poll reads it again (D3 of the design).
			o := &outcome{Run: act.run, Ack: pl.ack(), msg: a.msg}
			o.fail(a.msg.interrupted)
			rec := record{Issue: ref.String(), Run: act.run, Started: act.started, Ack: pl.ack()}
			a.keepWriteBack(ref, &pendingWriteBack{project: pl.project, o: o, interrupted: true}, rec, err)
		}
		return err
	}
	a.logf("%s: run %s started", ref, act.run)
	go func() {
		defer a.wg.Done()
		state := a.execute(runCtx, pl, act)
		finish()
		a.afterRun(pl.project, ref, state)
	}()
	return nil
}

// afterRun starts what the end of a run makes ready: the next run of a waiting or done issue that got a reply
// meanwhile (ADR-016, ADR-042), then the queued issues. It runs in turn with the polls, so that every launch is
// serialized with them: a poll never sees an issue that is active but not yet labeled running (#25).
func (a *Agent) afterRun(p config.Project, ref forge.IssueRef, state string) {
	a.pollMu.Lock()
	defer a.pollMu.Unlock()
	if state == "waiting" || state == "done" || state == marker.Queued {
		a.resume(p, ref)
	}
	a.drain(a.ctx)
	a.retune() // a queued issue that failed to start is evaluated again within poll_interval
}

// resume starts the next run of an issue just made waiting or done, when a reply already arrived. A project removed from the
// configuration starts no new run.
func (a *Agent) resume(p config.Project, ref forge.IssueRef) {
	if !a.handles(p) {
		return
	}
	if err := a.consider(a.ctx, p, ref); err != nil && !errors.Is(err, errQueued) && a.ctx.Err() == nil {
		a.logf("%s: %v", ref, err)
	}
}

// queuedRun is an issue waiting for a slot: its project, whether its run is a plan run, and when it entered the
// queue.
type queuedRun struct {
	project config.Project
	plan    bool
	since   time.Time
}

// queueOrder is the order in which drain offers the slots (ADR-046): the plan runs first when preferPlan is on, then
// the earliest in the queue, then by ref. The fix runs that wait for a slot are not in the queue: startFixes starts
// them before it (ADR-054). Call it with mu held.
func (a *Agent) queueOrder() []forge.IssueRef {
	refs := make([]forge.IssueRef, 0, len(a.pending))
	for ref := range a.pending {
		refs = append(refs, ref)
	}
	sort.Slice(refs, func(i, j int) bool {
		qi, qj := a.pending[refs[i]], a.pending[refs[j]]
		if a.preferPlan && qi.plan != qj.plan {
			return qi.plan
		}
		if !qi.since.Equal(qj.since) {
			return qi.since.Before(qj.since)
		}
		return refs[i].String() < refs[j].String()
	})
	return refs
}

// drain starts the fix runs that wait for a slot (startFixes), then queued issues while slots are free, in
// queueOrder. An issue whose project is at its limit stays queued and does not hold up the issues of other projects.
func (a *Agent) drain(ctx context.Context) {
	a.startFixes(ctx)
	a.mu.Lock()
	refs := a.queueOrder()
	a.mu.Unlock()
	for _, ref := range refs {
		limit := a.projectLimit(ref)
		a.mu.Lock()
		full := len(a.active) >= a.maxParallel
		q, ok := a.pending[ref]
		p := q.project
		slot, _, _ := a.hasSlot(ref, limit)
		free := ok && !full && slot
		if free {
			delete(a.pending, ref)
		}
		a.mu.Unlock()
		if full || ctx.Err() != nil {
			return
		}
		if !free {
			continue
		}
		switch err := a.consider(ctx, p, ref); {
		case err == nil:
			a.mu.Lock()
			delete(a.unsettled, ref)
			a.mu.Unlock()
		case errors.Is(err, errQueued):
			a.keepPlace(ref, q.since)
		default:
			// The next poll lists the issue again, since it stays unsettled; it now shortens the interval.
			a.mu.Lock()
			if u, ok := a.unsettled[ref]; ok {
				u.queued = false
				a.unsettled[ref] = u
			}
			a.mu.Unlock()
			a.logf("%s: %v", ref, err)
		}
	}
}

// keepPlace gives ref back the time it entered the queue, when drain took it out and launch queued it again.
func (a *Agent) keepPlace(ref forge.IssueRef, since time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if q, ok := a.pending[ref]; ok {
		q.since = since
		a.pending[ref] = q
	}
}

// hasSlot reports whether a run of ref may start now: the host is not paused, the agent is under agent.max_parallel,
// and the issue's project under limit, its own limit (0: none). Without a slot it names the pause
// (marker.QueuedLimit or marker.QueuedQuota) or the limit reached (marker.QueuedHost or marker.QueuedProject) and its
// value. Call it with mu held.
func (a *Agent) hasSlot(ref forge.IssueRef, limit int) (bool, string, int) {
	if reason, _ := a.pausedLocked(); reason != "" {
		return false, reason, 0
	}
	if len(a.active) >= a.maxParallel {
		return false, marker.QueuedHost, a.maxParallel
	}
	if limit == 0 {
		return true, "", 0
	}
	n := 0
	for r := range a.active {
		if r.Kind() == ref.Kind() && strings.EqualFold(r.Owner, ref.Owner) && strings.EqualFold(r.Repo, ref.Repo) {
			n++
		}
	}
	if n >= limit {
		return false, marker.QueuedProject, limit
	}
	return true, "", 0
}

// projectLimit is the limit of ref's project as configured now, or 0 when it has none. Projects are read under pmu,
// which is never taken with mu held.
func (a *Agent) projectLimit(ref forge.IssueRef) int {
	for _, p := range a.projectList() {
		if inProject(ref, p) {
			return p.ParallelLimit()
		}
	}
	return 0
}

// cleanClosed cleans up the closed issues whose run has stopped since they were found closed.
func (a *Agent) cleanClosed(ctx context.Context) {
	a.mu.Lock()
	var refs []forge.IssueRef
	for ref := range a.closed {
		if a.active[ref] == nil {
			refs = append(refs, ref)
		}
	}
	a.mu.Unlock()
	for _, ref := range refs {
		for _, p := range a.projectList() {
			if inProject(ref, p) {
				a.cleanUp(ctx, p, ref)
			}
		}
	}
}

// cleanUp removes the worktree and the local issue branch of a closed issue that is not running. A worktree with
// uncommitted changes stays, and the log says so. The remote branch is never touched.
func (a *Agent) cleanUp(ctx context.Context, p config.Project, ref forge.IssueRef) {
	if a.isActive(ref) {
		a.mu.Lock()
		a.closed[ref] = true
		a.mu.Unlock()
		return
	}
	a.mu.Lock()
	delete(a.closed, ref)
	a.mu.Unlock()
	sp := a.spaceOf(p, ref)
	removed, err := sp.Remove(ctx)
	switch {
	case errors.Is(err, workspace.ErrUncommitted):
		a.logf("%s: kept the worktree %s: it has uncommitted changes", ref, sp.Dir())
	case err != nil:
		a.logf("%s: remove the worktree %s: %v", ref, sp.Dir(), err)
	case removed:
		a.logf("%s: removed the worktree and the local branch %s", ref, sp.IssueBranch())
		a.Audit.Log("worktree.removed", map[string]any{"issue": ref.String(), "branch": sp.IssueBranch()})
	}
}

// claim posts the comment that claims the run of pl for this host, then reads the issue again. Of the claims with
// the same ack (the same trigger), the one with the smallest comment ID wins; IDs are given by the forge, so no
// host clock is involved. A host that loses removes its own claim and reports false. An earlier claim of this host,
// left by an agent that stopped before running, also counts as this host's.
func (a *Agent) claim(ctx context.Context, pl *plan, act *active) (bool, error) {
	ref, f, host := pl.issue.Ref, a.forgeOf(pl.issue.Ref), a.Config.Host.Name
	m := marker.Marker{Host: host, Run: act.run, State: "running", Ack: pl.ack(), At: time.Now()}
	if pl.start != "" {
		m.Mode = pl.mode // the claim starts a cycle; a resumed run's claim does not
	}
	if pl.fix != nil {
		m.Reason = marker.Fix // counted toward agent.max_fix_attempts
	}
	mine, err := f.PostComment(ctx, ref, a.claimText(pl)+"\n\n"+m.Format()+"\n")
	if err != nil {
		return false, fmt.Errorf("post the claim: %w", err)
	}
	_, comments, err := f.Thread(ctx, ref)
	if err != nil {
		return false, err
	}
	agentID := a.judgeOf(ref).AgentIDOf(ref)
	var first *forge.Comment
	var firstHost string
	for i, c := range comments {
		cm, ok := marker.Parse(c.Body)
		if !ok || c.Author.ID != agentID || cm.State != "running" || cm.Ack != m.Ack {
			continue
		}
		if first == nil || c.ID < first.ID {
			first, firstHost = &comments[i], cm.Host
		}
	}
	if first == nil || first.ID == mine.ID || firstHost == host {
		a.Audit.Log("run.claimed", map[string]any{"issue": ref.String(), "run": act.run, "comment": mine.ID})
		return true, nil
	}
	if err := f.DeleteComment(ctx, ref, mine.ID); err != nil && !errors.Is(err, forge.ErrNotFound) {
		a.logf("%s: remove the claim %d: %v", ref, mine.ID, err)
	}
	a.logf("%s: host %s claimed the issue first; this host does not run it", ref, firstHost)
	a.Audit.Log("run.yielded", map[string]any{"issue": ref.String(), "run": act.run, "winner": firstHost, "comment": first.ID})
	return false, nil
}

func (a *Agent) checkSandbox(ctx context.Context) error {
	if a.CheckSandbox != nil {
		return a.CheckSandbox(ctx)
	}
	return claudecode.CheckSandbox(ctx, "")
}
