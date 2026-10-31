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
)

func (k runKind) String() string {
	return [...]string{"direct", "plan", "approved", "conflict"}[k]
}

// modeFor returns the mode a start label asks for: plan for the plan label, direct for the go label, and the
// project's plan_first for the ready label.
func (a *Agent) modeFor(p config.Project, start string) string {
	n := a.Config.Labels
	switch {
	case start == n.Plan:
		return marker.ModePlan
	case start == n.Go:
		return marker.ModeDirect
	case p.PlanFirst:
		return marker.ModePlan
	}
	return marker.ModeDirect
}

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
// cycle. Otherwise the cycle and its mode are read from the agent's posts. word is the approval word.
func (pl *plan) classify(word string) {
	if pl.kind == conflictRun {
		return
	}
	if pl.start != "" {
		pl.kind, pl.qNext = directRun, 1
		if pl.mode == marker.ModePlan {
			pl.kind = planRun
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
	if mode != marker.ModePlan {
		return
	}
	pl.kind, pl.qNext = planRun, 1
	latest := -1
	for i := from + 1; i < len(pl.comments); i++ {
		m := pl.agentMarker(i)
		switch {
		case m == nil || m.State != marker.Waiting:
		case m.Reason == marker.PlanReady:
			latest = i
		case m.Reason == marker.PlanQuestions && m.Q > 0:
			pl.qNext = m.Q + 1
		}
	}
	if latest < 0 {
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
