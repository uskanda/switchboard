package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/trust"
	"github.com/uskanda/switchboard/internal/workspace"
)

// Reload hands the agent the projects, agent.max_parallel and agent.prefer_plan of a new configuration file. The
// agent applies them before its next poll: it stops taking up the projects that are gone (their runs finish), takes the changed
// settings of the others, and accepts the new ones of this host once they are cloned and have the state labels. A
// lower limit stops no run; the poll's drain starts queued issues under the new limits.
func (a *Agent) Reload(ch *config.Change) {
	var mine []config.Project
	for _, p := range ch.Projects {
		if p.Host == a.Config.Host.Name {
			mine = append(mine, p)
		}
	}
	a.pmu.Lock()
	a.desired, a.desiredMax, a.desiredPrefer, a.reloaded = mine, ch.MaxParallel, ch.PreferPlan, true
	a.pmu.Unlock()
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// applyProjects applies a pending reload and tries the projects still waiting for acceptance. It runs in turn with
// the polls, so that a project is never cloned while it is polled.
func (a *Agent) applyProjects(ctx context.Context) {
	a.pollMu.Lock()
	defer a.pollMu.Unlock()
	a.pmu.Lock()
	if a.reloaded {
		a.reloaded = false
		a.applyDesired()
		a.mu.Lock()
		if a.desiredMax > 0 {
			a.maxParallel = a.desiredMax
		}
		a.preferPlan = a.desiredPrefer
		a.mu.Unlock()
	}
	waiting := make([]config.Project, 0, len(a.accepting))
	for _, p := range a.accepting {
		waiting = append(waiting, p)
	}
	dropped := a.dropped
	a.dropped = nil
	a.pmu.Unlock()
	for _, ref := range dropped {
		a.unqueue(ctx, ref)
	}
	for _, p := range waiting {
		if ctx.Err() != nil {
			return
		}
		a.accept(ctx, p)
	}
}

// applyDesired replaces the projects with desired, keeping the new ones for acceptance. Call it with pmu held.
func (a *Agent) applyDesired() {
	want := map[string]config.Project{}
	for _, p := range a.desired {
		want[p.Ref] = p
	}
	var kept []config.Project
	for _, p := range a.projects {
		if np, ok := want[p.Ref]; ok {
			kept = append(kept, np)
			delete(want, p.Ref)
			continue
		}
		a.logf("%s: removed from the configuration; its running runs finish, and its issues are no longer taken up", p.Ref)
		delete(a.private, p.Ref)
		a.dropped = append(a.dropped, a.dropPending(p)...)
	}
	a.projects = kept
	for ref := range a.accepting {
		if _, ok := want[ref]; !ok {
			delete(a.accepting, ref)
			delete(a.failures, ref)
		}
	}
	for ref, p := range want {
		a.accepting[ref] = p
	}
}

// dropPending forgets the queued issues of a removed project and the issues it would have evaluated again, and returns the queued ones, so that their queued comments can
// be removed once pmu is released.
func (a *Agent) dropPending(p config.Project) []forge.IssueRef {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []forge.IssueRef
	for ref := range a.pending {
		if inProject(ref, p) {
			delete(a.pending, ref)
			out = append(out, ref)
		}
	}
	for ref := range a.unsettled {
		if inProject(ref, p) {
			delete(a.unsettled, ref)
		}
	}
	return out
}

// accept takes on a project of a reload: its route, the repository's visibility, the clone and the state labels.
// A failure keeps the project waiting for the next poll; a project refused for its untrusted input is dropped.
func (a *Agent) accept(ctx context.Context, p config.Project) {
	r, err := a.newRoute(p)
	if err == nil {
		var private bool
		private, err = r.Forge.RepoPrivate(ctx, p.Owner, p.Repo)
		if err == nil && !a.allowed(p, private) {
			a.pmu.Lock()
			delete(a.accepting, p.Ref)
			delete(a.failures, p.Ref)
			a.pmu.Unlock()
			return
		}
		if err == nil {
			err = a.prepareClone(ctx, p, r)
		}
		if err == nil {
			_, err = labels.Sync(ctx, r.Forge, []config.Project{p}, a.Config.Labels)
		}
		if err == nil {
			a.pmu.Lock()
			if a.Routes == nil {
				a.Routes = map[string]Route{}
			}
			a.Routes[p.Ref] = r
			a.private[p.Ref] = private
			a.projects = append(a.projects, p)
			delete(a.accepting, p.Ref)
			delete(a.failures, p.Ref)
			a.pmu.Unlock()
			a.logf("%s: added from the configuration; cloned at %s with the state labels", p.Ref, p.Path)
			a.Audit.Log("project.added", map[string]any{"project": p.Ref})
			return
		}
	}
	if ctx.Err() != nil {
		return
	}
	msg := err.Error()
	a.pmu.Lock()
	again := a.failures[p.Ref] == msg
	a.failures[p.Ref] = msg
	a.pmu.Unlock()
	if !again {
		a.logf("%s: not taken up yet: %s; tried again on the next poll", p.Ref, msg)
	}
}

// newRoute is the route of a project that appears on a reload.
func (a *Agent) newRoute(p config.Project) (Route, error) {
	a.pmu.RLock()
	r, ok := a.Routes[p.Ref]
	a.pmu.RUnlock()
	if ok {
		return r, nil
	}
	if a.NewRoute == nil {
		return Route{Forge: a.Forge, Judge: a.Judge, GitToken: a.GitToken}, nil
	}
	return a.NewRoute(p)
}

// prepareClone clones the project to its path, or checks that the clone already there is of the project.
func (a *Agent) prepareClone(ctx context.Context, p config.Project, r Route) error {
	url := a.cloneURL(p)
	if _, err := os.Stat(p.Path); errors.Is(err, os.ErrNotExist) {
		user := ""
		if p.Forge == forge.GitLab {
			user = "oauth2"
		}
		if err := workspace.Clone(ctx, a.Env, url, p.Path, r.GitToken, user); err != nil {
			return fmt.Errorf("clone %s into %s: %w", url, p.Path, err)
		}
		return nil
	} else if err != nil {
		return err
	}
	origin, err := workspace.Origin(ctx, a.Env, p.Path)
	if err != nil {
		return fmt.Errorf("%s is there but is not a clone: %w", p.Path, err)
	}
	if !workspace.SameRepo(origin, url) {
		return fmt.Errorf("%s is a clone of %s, not of %s (its origin differs)", p.Path, origin, url)
	}
	return nil
}

// cloneURL is where a project is cloned from: the forge's https URL unless CloneURL says otherwise.
func (a *Agent) cloneURL(p config.Project) string {
	if a.CloneURL != nil {
		return a.CloneURL(p)
	}
	if p.Forge == forge.GitLab {
		return strings.TrimRight(a.Config.Forges.GitLab.BaseURL, "/") + "/" + p.Owner + "/" + p.Repo + ".git"
	}
	return "https://github.com/" + p.Owner + "/" + p.Repo + ".git"
}

// allowed applies the untrusted-input policy to a project: a public repository may not take untrusted input. A
// refused project is logged and audited.
func (a *Agent) allowed(p config.Project, private bool) bool {
	if _, err := trust.UntrustedPolicy(p.IncludeUntrusted, private); err != nil {
		a.logf("%s: %v; its issues are not run", p.FullName(), err)
		a.Audit.Log("project.refused", map[string]any{"project": p.FullName(), "reason": err.Error()})
		return false
	}
	return true
}
