package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/prompt"
	"github.com/uskanda/switchboard/internal/trust"
	"github.com/uskanda/switchboard/internal/workspace"
)

// watch is a done issue whose CI the agent watches: the commit pushed, its change request, and when it was pushed.
type watch struct {
	project config.Project
	sha     string
	cr      int
	pushed  time.Time
	// giveUp is the wait written when the CI failed past the fix limit, kept until it is written back, so that a
	// retry posts it once.
	giveUp *outcome
}

const (
	ciAppearWithin = 10 * time.Minute // a commit without any CI by then belongs to a repository without CI
	ciWatchFor     = 6 * time.Hour    // the CI of a commit is given up on after this
	ciLogJobs      = 3                // the failed jobs whose logs a fix run gets
)

func (a *Agent) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// startWatch watches the CI of the commit sha pushed to change request cr of ref at pushed.
func (a *Agent) startWatch(p config.Project, ref forge.IssueRef, sha string, cr int, pushed time.Time) {
	if _, ok := a.forgeOf(ref).(forge.CI); !ok {
		return
	}
	a.mu.Lock()
	a.watches[ref] = &watch{project: p, sha: sha, cr: cr, pushed: pushed}
	delete(a.unwatched, ref)
	a.mu.Unlock()
	a.logf("%s: watching the CI of %s", ref, sha)
	a.retune() // the CI is read every poll_interval, also while the relay is up
}

// unwatch stops watching the CI of ref, for why.
func (a *Agent) unwatch(ref forge.IssueRef, why string) {
	a.mu.Lock()
	_, ok := a.watches[ref]
	delete(a.watches, ref)
	a.mu.Unlock()
	if !ok {
		return
	}
	a.logf("%s: stopped watching the CI: %s", ref, why)
	a.Audit.Log("ci.watch.ended", map[string]any{"issue": ref.String(), "reason": why})
}

// watching reports whether the CI of some issue is watched.
func (a *Agent) watching() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.watches) > 0
}

// searchScope is the scope SearchOpenIssuesWithLabel takes for the issues of p.
func searchScope(p config.Project) string {
	if p.Forge == forge.GitLab {
		return p.FullName() // GitLab searches one project, named by its path
	}
	return "repo:" + p.FullName()
}

// scanWatches finds the done issues of this host's projects whose CI is still to watch: the latest post of the
// agent is a done post naming a commit and a change request, posted within ciWatchFor. An issue found with nothing
// to watch is read again only when it changes.
func (a *Agent) scanWatches(ctx context.Context) {
	a.mu.Lock()
	a.scanned = a.now()
	a.mu.Unlock()
	for _, p := range a.projectList() {
		f := a.routeOf(p).Forge
		if _, ok := f.(forge.CI); !ok {
			continue
		}
		list, err := f.SearchOpenIssuesWithLabel(ctx, searchScope(p), a.Config.Labels.Done)
		if err != nil {
			a.logf("%s: search the done issues: %v", p.Ref, err)
			continue
		}
		for _, s := range list {
			a.mu.Lock()
			_, watched := a.watches[s.Ref]
			at, seen := a.unwatched[s.Ref]
			a.mu.Unlock()
			if !inProject(s.Ref, p) || watched || seen && !s.UpdatedAt.After(at) {
				continue
			}
			a.findWatch(ctx, p, s)
		}
	}
}

func (a *Agent) findWatch(ctx context.Context, p config.Project, s forge.IssueSummary) {
	_, comments, err := a.forgeOf(s.Ref).Thread(ctx, s.Ref)
	if err != nil {
		a.logf("%s: read the issue for its CI: %v", s.Ref, err)
		return
	}
	agentID := a.judgeOf(s.Ref).AgentIDOf(s.Ref)
	var last *marker.Marker
	for i := len(comments) - 1; i >= 0 && last == nil; i-- {
		if m, ok := marker.Parse(comments[i].Body); ok && comments[i].Author.ID == agentID && !m.Transient() {
			last = m
		}
	}
	if last != nil && last.State == marker.Done && last.SHA != "" && last.CR != 0 && a.now().Sub(last.At) < ciWatchFor {
		a.startWatch(p, s.Ref, last.SHA, last.CR, last.At)
		return
	}
	a.mu.Lock()
	a.unwatched[s.Ref] = s.UpdatedAt
	a.mu.Unlock()
}

// checkWatches reads the CI of every watched issue that does not run, and acts on what finished. The done issues
// are searched again once every reconcile_interval. Call it with pollMu held.
func (a *Agent) checkWatches(ctx context.Context) {
	a.mu.Lock()
	scan := a.now().Sub(a.scanned) >= a.Config.Agent.ReconcileInterval
	a.mu.Unlock()
	if scan {
		a.scanWatches(ctx)
	}
	a.mu.Lock()
	refs := make([]forge.IssueRef, 0, len(a.watches))
	for ref := range a.watches {
		if a.active[ref] == nil {
			refs = append(refs, ref)
		}
	}
	a.mu.Unlock()
	sort.Slice(refs, func(i, j int) bool { return refs[i].String() < refs[j].String() })
	for _, ref := range refs {
		if ctx.Err() != nil {
			return
		}
		a.mu.Lock()
		w := a.watches[ref]
		a.mu.Unlock()
		if w != nil {
			a.checkWatch(ctx, ref, w)
		}
	}
}

func (a *Agent) checkWatch(ctx context.Context, ref forge.IssueRef, w *watch) {
	ci, ok := a.forgeOf(ref).(forge.CI)
	if !ok {
		a.unwatch(ref, "the forge client cannot read CI")
		return
	}
	if w.giveUp != nil {
		a.writeGiveUp(ctx, ref, w)
		return
	}
	st, err := ci.ChangeRequestHead(ctx, ref.Owner, ref.Repo, w.cr)
	if err != nil {
		a.ciError(ref, w, err)
		return
	}
	switch {
	case st.State != forge.CROpen:
		a.unwatch(ref, "the change request was "+st.State)
		return
	case st.Head != w.sha:
		a.unwatch(ref, "the change request moved on to another commit")
		return
	}
	checks, err := ci.CommitChecks(ctx, ref.Owner, ref.Repo, w.sha)
	if err != nil {
		a.ciError(ref, w, err)
		return
	}
	age := a.now().Sub(w.pushed)
	if len(checks) == 0 {
		if age >= ciAppearWithin {
			a.unwatch(ref, "no CI ran on the commit")
		}
		return
	}
	var failed []forge.Check
	for _, c := range checks {
		if !c.Done {
			if age >= ciWatchFor {
				a.unwatch(ref, fmt.Sprintf("the CI did not finish within %s", ciWatchFor))
			}
			return
		}
		if c.Failed() {
			failed = append(failed, c)
		}
	}
	if len(failed) == 0 {
		a.unwatch(ref, "the CI passed")
		return
	}
	a.ciFailed(ctx, ref, w, ci, failed)
}

// ciError handles a failed read of the CI: a token that may not read it ends the watch, logged once per project;
// a change request that is gone ends it too; anything else is tried again at the next poll.
func (a *Agent) ciError(ref forge.IssueRef, w *watch, err error) {
	switch {
	case errors.Is(err, forge.ErrForbidden):
		a.unwatch(ref, "the token may not read the CI")
		a.Audit.Log("ci.forbidden", map[string]any{"issue": ref.String(), "error": err.Error()})
		a.mu.Lock()
		logged := a.forbidden[w.project.Ref]
		a.forbidden[w.project.Ref] = true
		a.mu.Unlock()
		if !logged {
			a.logf("%s: the agent's token may not read the CI (%v); give it Actions: Read and Commit statuses: Read on GitHub", w.project.Ref, err)
		}
	case errors.Is(err, forge.ErrNotFound):
		a.unwatch(ref, "the change request is gone")
	default:
		a.logf("%s: read the CI: %v; tried again at the next poll", ref, err)
	}
}

// ciFailed starts a fix run of the failed CI, or, past the fix limit, makes the issue wait (reason ci). A fix run
// that finds no free slot is tried again at the next poll.
func (a *Agent) ciFailed(ctx context.Context, ref forge.IssueRef, w *watch, ci forge.CI, failed []forge.Check) {
	fix := &fixInfo{cause: prompt.FixCI, checks: checkNames(failed)}
	pl, err := a.fixPlan(ctx, w.project, ref, a.Config.Labels.Done, fix)
	switch {
	case err != nil:
		a.logf("%s: read the issue for its failed CI: %v; tried again at the next poll", ref, err)
		return
	case pl == nil:
		a.unwatch(ref, "the issue no longer allows a fix run")
		return
	}
	limit := a.Config.Agent.FixAttempts()
	if fix.attempt > limit {
		logs, err := a.ciLogs(ctx, ref, ci, failed)
		if err != nil {
			a.ciError(ref, w, err)
			return
		}
		w.giveUp = a.giveUpOutcome(pl, w, failed, logs, fix.attempt-1, limit)
		a.writeGiveUp(ctx, ref, w)
		return
	}
	limitOf := a.projectLimit(ref)
	a.mu.Lock()
	free, _, _ := a.hasSlot(ref, limitOf)
	a.mu.Unlock()
	if !free {
		return
	}
	logs, err := a.ciLogs(ctx, ref, ci, failed)
	if err != nil {
		a.ciError(ref, w, err)
		return
	}
	fix.report = ciReport(w.sha, failed, logs)
	switch err := a.launch(ctx, pl); {
	case err == nil:
		a.unwatch(ref, "the CI failed; a fix run started")
	case errors.Is(err, errQueued):
	default:
		a.logf("%s: start the fix run: %v; tried again at the next poll", ref, err)
	}
}

// ciLogs reads the logs of the first ciLogJobs failed checks that have one, by check index, trimmed.
func (a *Agent) ciLogs(ctx context.Context, ref forge.IssueRef, ci forge.CI, failed []forge.Check) (map[int]string, error) {
	logs := map[int]string{}
	for i, c := range failed {
		if c.Job == 0 || len(logs) == ciLogJobs {
			continue
		}
		raw, err := ci.CheckLog(ctx, ref.Owner, ref.Repo, c)
		if errors.Is(err, forge.ErrForbidden) {
			return nil, err
		}
		if err != nil {
			logs[i] = fmt.Sprintf("(the log could not be read: %v)", err)
			continue
		}
		logs[i] = trimLog(raw)
	}
	return logs, nil
}

// checkNames lists the failed checks as a claim names them.
func checkNames(failed []forge.Check) string {
	var names []string
	for _, c := range failed {
		names = append(names, "`"+strings.ReplaceAll(c.Name, "`", "'")+"`")
	}
	return strings.Join(names, ", ")
}

// ciReport is the failure report of a CI fix run: each failed check with what the forge says about it, and the end
// of its log when there is one. It is redacted.
func ciReport(sha string, failed []forge.Check, logs map[int]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Failed CI checks of commit %s:\n", sha)
	for i, c := range failed {
		fmt.Fprintf(&b, "\n## %s (%s)\n", c.Name, c.Conclusion)
		if c.URL != "" {
			fmt.Fprintf(&b, "URL: %s\n", c.URL)
		}
		if c.Description != "" {
			fmt.Fprintf(&b, "Description: %s\n", c.Description)
		}
		if len(c.FailedSteps) > 0 {
			fmt.Fprintf(&b, "Failed steps: %s\n", strings.Join(c.FailedSteps, ", "))
		}
		if log, ok := logs[i]; ok {
			fmt.Fprintf(&b, "End of the log:\n%s\n", log)
		}
	}
	return workspace.Redact(b.String())
}

// giveUpOutcome is the wait (reason ci) of a CI that failed past the fix limit: the failed checks, a summary of each
// fix run, and the ends of the logs.
func (a *Agent) giveUpOutcome(pl *plan, w *watch, failed []forge.Check, logs map[int]string, made, limit int) *outcome {
	o := &outcome{Run: newID(), Ack: pl.ack(), msg: a.msg}
	cr := fmt.Sprintf("#%d", w.cr)
	if pl.issue.Ref.Kind() == forge.GitLab {
		cr = fmt.Sprintf("!%d", w.cr)
	}
	lead := fmt.Sprintf(a.msg.ciFailed, cr, made, limit)
	if limit == 0 {
		lead = fmt.Sprintf(a.msg.ciNoFixes, cr)
	}
	var checks []string
	for _, c := range failed {
		line := "- `" + strings.ReplaceAll(c.Name, "`", "'") + "` (" + c.Conclusion + ")"
		if c.URL != "" {
			line += " " + c.URL
		}
		checks = append(checks, line)
	}
	parts := []string{lead, "**" + a.msg.ciChecks + "**\n\n" + workspace.Redact(strings.Join(checks, "\n"))}
	if sums := pl.fixSummaries(); len(sums) > 0 {
		var lines []string
		for i, s := range sums {
			lines = append(lines, "- "+fmt.Sprintf(a.msg.ciAttempt, i+1)+": "+strings.Join(strings.Fields(s), " "))
		}
		parts = append(parts, "**"+a.msg.ciAttempts+"**\n\n"+strings.Join(lines, "\n"))
	}
	for i, c := range failed {
		if log, ok := logs[i]; ok {
			parts = append(parts, "<details><summary>"+fmt.Sprintf(a.msg.ciLog, c.Name)+"</summary>\n\n```\n"+tail(log)+"\n```\n\n</details>")
		}
	}
	parts = append(parts, a.msg.ciReply)
	return o.set("waiting", marker.CI, a.msg.ciTitle, parts...)
}

// writeGiveUp writes back the kept wait of w; a failure is tried again at the next poll.
func (a *Agent) writeGiveUp(ctx context.Context, ref forge.IssueRef, w *watch) {
	if err := a.writeBack(ctx, ref, w.giveUp); err != nil {
		a.logf("%s: write back the failed CI: %v; tried again at the next poll", ref, err)
		return
	}
	a.unwatch(ref, "the CI failed past the limit of fix runs")
	if ref.Kind() == forge.GitLab {
		a.keepNotes(a.ctx, w.project, ref)
	}
}

// fixSummaries are the summaries of the fix runs since the last reset that ended done, oldest first.
func (pl *plan) fixSummaries() []string {
	from := 0
	for i := len(pl.comments) - 1; i >= 0; i-- {
		if m := pl.agentMarker(i); pl.kinds[i] == trust.Instruction || m != nil && m.State == marker.Running && m.Mode != "" {
			from = i + 1
			break
		}
	}
	var out []string
	inFix := false
	for i := from; i < len(pl.comments); i++ {
		m := pl.agentMarker(i)
		switch {
		case m == nil:
		case m.State == marker.Running:
			inFix = m.Reason == marker.Fix
		case m.State == marker.Done && inFix:
			out = append(out, doneSummary(pl.comments[i].Body))
			inFix = false
		}
	}
	return out
}

// doneSummary is the summary of a done post: what follows its heading and the line naming the change request.
func doneSummary(body string) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if n := len(lines); n > 0 && strings.HasPrefix(strings.TrimSpace(lines[n-1]), "<!-- switchboard ") {
		lines = lines[:n-1]
	}
	parts := strings.Split(strings.TrimSpace(strings.Join(lines, "\n")), "\n\n")
	if len(parts) < 3 {
		return ""
	}
	return strings.TrimSpace(strings.Join(parts[2:], "\n\n"))
}
