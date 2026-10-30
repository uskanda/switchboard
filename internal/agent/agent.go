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
	"sort"
	"strings"
	"sync"
	"time"

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

	ctx      context.Context // runs end when it does
	unlock   func()
	projects []config.Project
	private  map[string]bool // by project full name
	msg      texts

	mu      sync.Mutex
	active  map[forge.IssueRef]*active
	pending map[forge.IssueRef]config.Project
	seen    map[forge.IssueRef]time.Time
	notes   map[forge.IssueRef]int  // GitLab: the comment count of each waiting issue at its last evaluation
	closed  map[forge.IssueRef]bool // closed issues whose worktree waits for their run to stop
	wg      sync.WaitGroup
}

// Route is what one project uses to reach its forge.
type Route struct {
	Forge    forge.Forge
	Judge    *trust.Judge
	GitToken string // offered to git push for https remotes; never given to claude
}

// route is the issue's route: its project's, or the agent's defaults.
func (a *Agent) route(ref forge.IssueRef) Route {
	if r, ok := a.Routes[ref.Kind()+":"+ref.Project()]; ok {
		return r
	}
	return Route{Forge: a.Forge, Judge: a.Judge, GitToken: a.GitToken}
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

// errQueued reports that an issue waits for a free slot; it is evaluated again when one frees up.
var errQueued = errors.New("queued")

// lockName keeps the Phase 1 name for GitHub. GitLab names start with "@", which no GitHub owner can, and replace the
// slashes of subgroups so that the lock stays one file.
func lockName(ref forge.IssueRef) string {
	if ref.Kind() == forge.GitLab {
		return fmt.Sprintf("@gitlab-%s-%s-%d", strings.ReplaceAll(ref.Owner, "/", "~"), ref.Repo, ref.Number)
	}
	return fmt.Sprintf("%s-%s-%d", ref.Owner, ref.Repo, ref.Number)
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
	a.active = map[forge.IssueRef]*active{}
	a.pending = map[forge.IssueRef]config.Project{}
	a.seen = map[forge.IssueRef]time.Time{}
	a.notes = map[forge.IssueRef]int{}
	a.closed = map[forge.IssueRef]bool{}
	a.private = map[string]bool{}
	if err := a.State.WriteJSON("agent.json", agentInfo{PID: os.Getpid(), Host: a.Config.Host.Name, Started: time.Now().UTC()}); err != nil {
		return err
	}
	a.projects = nil
	for _, p := range a.Config.ProjectsFor(a.Config.Host.Name) {
		private, err := a.routeOf(p).Forge.RepoPrivate(ctx, p.Owner, p.Repo)
		if err != nil {
			return fmt.Errorf("%s: %w", p.FullName(), err)
		}
		if _, err := trust.UntrustedPolicy(p.IncludeUntrusted, private); err != nil {
			a.logf("%s: %v; its issues are not run", p.FullName(), err)
			a.Audit.Log("project.refused", map[string]any{"project": p.FullName(), "reason": err.Error()})
			continue
		}
		a.private[p.Ref] = private
		a.projects = append(a.projects, p)
	}
	return a.recover(ctx)
}

// Close releases the agent lock. Call it after Wait.
func (a *Agent) Close() {
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
	a.logf("polling %d project(s) every %s", len(a.projects), a.Config.Agent.PollInterval)
	t := time.NewTicker(a.Config.Agent.PollInterval)
	defer t.Stop()
	for {
		if err := a.Poll(ctx); err != nil && ctx.Err() == nil {
			a.logf("poll: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// Poll evaluates the issues updated since the last poll, then starts queued issues while slots are free.
func (a *Agent) Poll(ctx context.Context) error {
	var errs []error
	for _, p := range a.projects {
		f := a.routeOf(p).Forge
		list, err := f.UpdatedIssues(ctx, p.Owner, p.Repo, a.since(p))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.Ref, err))
			continue
		}
		for _, s := range list {
			a.mu.Lock()
			last, ok := a.seen[s.Ref]
			a.mu.Unlock()
			if ok && !s.UpdatedAt.After(last) {
				continue
			}
			if err := a.evaluate(ctx, p, s); err != nil {
				errs = append(errs, err)
			}
		}
		if p.Forge == forge.GitLab {
			errs = append(errs, a.pollReplies(ctx, p, f)...)
		}
	}
	a.cleanClosed(ctx)
	a.drain(ctx)
	return errors.Join(errs...)
}

// pollReplies evaluates the waiting issues of a GitLab project whose comment count changed. GitLab does not move an
// issue's update time for a new comment (V22), so UpdatedIssues alone misses replies.
func (a *Agent) pollReplies(ctx context.Context, p config.Project, f forge.Forge) []error {
	waiting, err := f.SearchOpenIssuesWithLabel(ctx, p.FullName(), a.Config.Labels.Waiting)
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

// evaluate decides what an updated issue needs and records it as seen, unless it has to be evaluated again later.
func (a *Agent) evaluate(ctx context.Context, p config.Project, s forge.IssueSummary) error {
	err := a.evaluateOnce(ctx, p, s)
	switch {
	case err == nil:
		a.mu.Lock()
		if prev, ok := a.seen[s.Ref]; !ok || s.UpdatedAt.After(prev) {
			a.seen[s.Ref] = s.UpdatedAt
		}
		a.notes[s.Ref] = s.Notes
		a.mu.Unlock()
		return nil
	case errors.Is(err, errQueued), errors.Is(err, state.ErrLocked):
		return nil // evaluated again later
	}
	return fmt.Errorf("%s: %w", s.Ref, err)
}

// since is a minute before the latest update seen in the project, so that a clock skew between the forge and
// this host loses nothing. The value moves only when something changes, which keeps conditional requests cheap.
func (a *Agent) since(p config.Project) time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	var latest time.Time
	for ref, t := range a.seen {
		if ref.Kind() == p.Forge && ref.Owner == p.Owner && ref.Repo == p.Repo && t.After(latest) {
			latest = t
		}
	}
	if latest.IsZero() {
		return latest
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
	if st != n.Ready && st != n.Waiting {
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
	a.mu.Unlock()
	if act == nil {
		return nil
	}
	a.logf("%s: stopping run %s: %s", ref, act.run, reason)
	a.Audit.Log("run.cancel", map[string]any{"issue": ref.String(), "run": act.run, "reason": reason})
	act.cancel()
	return nil
}

// consider reads the issue again and starts a run when it should run.
func (a *Agent) consider(ctx context.Context, p config.Project, ref forge.IssueRef) error {
	pl, err := a.decide(ctx, p, ref)
	if err != nil || pl == nil {
		return err
	}
	return a.launch(ctx, pl)
}

// plan is an issue that should run, as read when that was decided.
type plan struct {
	project  config.Project
	issue    *forge.Issue
	comments []forge.Comment
	kinds    []trust.Kind
}

// ack is the last comment the run takes in, or 0.
func (pl *plan) ack() int64 {
	if len(pl.comments) == 0 {
		return 0
	}
	return pl.comments[len(pl.comments)-1].ID
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
	switch st {
	case n.Ready:
		actor, err := a.forgeOf(ref).LabelActor(ctx, ref, n.Ready)
		if errors.Is(err, forge.ErrNotFound) {
			actor = nil
		} else if err != nil {
			return nil, err
		}
		if !j.ReadyLabel(ref, actor) {
			return nil, nil
		}
	case n.Waiting:
	default:
		return nil, nil
	}
	pl := &plan{project: p, issue: is, comments: comments}
	for _, c := range comments {
		pl.kinds = append(pl.kinds, j.Comment(ref, c))
	}
	if st == n.Waiting && !pl.hasReply() {
		return nil, nil
	}
	if !j.Body(is) {
		return nil, nil
	}
	return pl, nil
}

// hasReply reports whether a trusted comment follows the comment acknowledged by the agent's latest post, and that
// post is a question (the issue is waiting because the agent made it wait).
func (pl *plan) hasReply() bool {
	var m *marker.Marker
	for i := len(pl.comments) - 1; i >= 0 && m == nil; i-- {
		if pl.kinds[i] == trust.AgentPost {
			m, _ = marker.Parse(pl.comments[i].Body)
		}
	}
	if m == nil || m.State != "waiting" {
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
// holds the issue's lock (state.ErrLocked).
func (a *Agent) launch(ctx context.Context, pl *plan) error {
	ref := pl.issue.Ref
	a.mu.Lock()
	if a.active[ref] != nil {
		a.mu.Unlock()
		return nil
	}
	if len(a.active) >= a.Config.Agent.MaxParallel {
		a.pending[ref] = pl.project
		a.mu.Unlock()
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
	if err := a.swapper(ref).Swap(ctx, ref, a.Config.Labels.Running); err != nil {
		finish()
		a.wg.Done()
		return err
	}
	a.logf("%s: run %s started", ref, act.run)
	go func() {
		defer a.wg.Done()
		state := a.execute(runCtx, pl, act)
		finish()
		if state == "waiting" {
			// A reply that arrived during the run starts the next run now.
			if err := a.consider(a.ctx, pl.project, ref); err != nil && !errors.Is(err, errQueued) && a.ctx.Err() == nil {
				a.logf("%s: %v", ref, err)
			}
		}
		a.drain(a.ctx)
	}()
	return nil
}

// drain starts queued issues while slots are free.
func (a *Agent) drain(ctx context.Context) {
	a.mu.Lock()
	refs := make([]forge.IssueRef, 0, len(a.pending))
	for ref := range a.pending {
		refs = append(refs, ref)
	}
	a.mu.Unlock()
	sort.Slice(refs, func(i, j int) bool { return refs[i].String() < refs[j].String() })
	for _, ref := range refs {
		a.mu.Lock()
		full := len(a.active) >= a.Config.Agent.MaxParallel
		p, ok := a.pending[ref]
		if !full && ok {
			delete(a.pending, ref)
		}
		a.mu.Unlock()
		if full || ctx.Err() != nil {
			return
		}
		if !ok {
			continue
		}
		if err := a.consider(ctx, p, ref); err != nil && !errors.Is(err, errQueued) {
			a.logf("%s: %v", ref, err)
		}
	}
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
		for _, p := range a.projects {
			if ref.Kind() == p.Forge && ref.Owner == p.Owner && ref.Repo == p.Repo {
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
