package agent

import (
	"strings"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/trust"
)

// runKind is what a run is asked to do. A cycle starts at a claim made from a start label; in plan mode, its runs
// plan until a trusted comment approves the latest plan, and implement after that.
type runKind int

const (
	directRun   runKind = iota // implement, as without the plan mode
	planRun                    // ask the open questions or present the plan; change nothing
	approvedRun                // implement the approved plan
	conflictRun                // more than one start label: report it, run nothing
	fixRun                     // fix what failed after a done run: its CI, or a push refused as the branch moved on
	skippedRun                 // implement the plan a plan run posted when it skipped the questions and the approval
)

func (k runKind) String() string {
	return [...]string{"direct", "plan", "approved", "conflict", "fix", "skipped"}[k]
}

// modeFor returns the mode a start label asks for: plan for the plan label, direct for the go label, and for the
// ready label the project's plan_first: plan for always, auto for auto, direct for never.
func (a *Agent) modeFor(p config.Project, start string) string {
	n := a.Config.Labels
	switch {
	case start == n.Plan:
		return marker.ModePlan
	case start == n.Go:
		return marker.ModeDirect
	}
	switch p.PlanFirst.Mode() {
	case config.PlanAlways:
		return marker.ModePlan
	case config.PlanAuto:
		return marker.ModeAuto
	}
	return marker.ModeDirect
}

// planStage reports whether a cycle of mode plans before it implements.
func planStage(mode string) bool { return mode == marker.ModePlan || mode == marker.ModeAuto }

// isApproval reports whether the first non-blank line of body starts with word, followed by the end of the line or
// a space.
func isApproval(body, word string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		return f[0] == word
	}
	return false
}

// classify sets the kind of the run of pl. For a start, pl.start and pl.mode are set already: the run begins a new
// cycle. Otherwise the cycle and its mode are read from the agent's posts. word is the approval word. A plan run of
// an auto cycle may skip the questions and the approval only while the cycle has neither: it is its first plan run,
// or one after a run that ended without posting either.
func (pl *plan) classify(word string) {
	pl.skippable = false
	if pl.kind == conflictRun {
		return
	}
	if pl.start != "" {
		pl.kind, pl.qNext = directRun, 1
		if planStage(pl.mode) {
			pl.kind, pl.skippable = planRun, pl.mode == marker.ModeAuto
		}
		return
	}
	pl.kind = directRun
	from, mode := -1, ""
	for i := len(pl.comments) - 1; i >= 0 && from < 0; i-- {
		if m := pl.agentMarker(i); m != nil && m.State == marker.Running && m.Mode != "" {
			from, mode = i, m.Mode
		}
	}
	if !planStage(mode) {
		return
	}
	pl.kind, pl.qNext = planRun, 1
	latest, asked := -1, false
	for i := from + 1; i < len(pl.comments); i++ {
		m := pl.agentMarker(i)
		switch {
		case m == nil || m.State != marker.Waiting:
		case m.Reason == marker.PlanReady:
			latest = i
		case m.Reason == marker.PlanQuestions:
			asked = true
			if m.Q > 0 {
				pl.qNext = m.Q + 1
			}
		}
	}
	if latest < 0 {
		pl.skippable = mode == marker.ModeAuto && !asked
		return
	}
	for i := latest + 1; i < len(pl.comments); i++ {
		if pl.kinds[i] == trust.Instruction && isApproval(pl.comments[i].Body, word) {
			pl.kind, pl.planID, pl.okID = approvedRun, pl.comments[latest].ID, pl.comments[i].ID
			return
		}
	}
}

// agentMarker returns the marker of the i-th comment when it is the agent's post, or nil.
func (pl *plan) agentMarker(i int) *marker.Marker {
	if pl.kinds[i] != trust.AgentPost {
		return nil
	}
	m, _ := marker.Parse(pl.comments[i].Body)
	return m
}
