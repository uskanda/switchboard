package agent

import (
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/trust"
)

// thread builds a plan from posts, oldest first. An agent post is written as its marker; anything else is a comment
// of the given kind.
type entry struct {
	kind trust.Kind
	m    *marker.Marker
	body string
}

func agentPost(m marker.Marker) entry { return entry{kind: trust.AgentPost, m: &m} }
func reply(body string) entry         { return entry{kind: trust.Instruction, body: body} }
func untrusted(body string) entry     { return entry{kind: trust.Untrusted, body: body} }
func claimOf(mode string) entry {
	return agentPost(marker.Marker{V: 1, State: marker.Running, Mode: mode})
}
func asked(q int) entry {
	return agentPost(marker.Marker{V: 1, State: marker.Waiting, Reason: marker.PlanQuestions, Q: q})
}
func planned() entry {
	return agentPost(marker.Marker{V: 1, State: marker.Waiting, Reason: marker.PlanReady})
}
func question() entry {
	return agentPost(marker.Marker{V: 1, State: marker.Waiting, Reason: "question"})
}
func doneAt() entry                             { return agentPost(marker.Marker{V: 1, State: marker.Done}) }
func resumed() entry                            { return agentPost(marker.Marker{V: 1, State: marker.Running}) }
func thread(posts ...entry) *plan               { return threadFrom("", "", posts...) }
func started(mode string, posts ...entry) *plan { return threadFrom("agent:ready", mode, posts...) }

func threadFrom(start, mode string, posts ...entry) *plan {
	pl := &plan{start: start, mode: mode}
	for i, p := range posts {
		body := p.body
		if p.m != nil {
			body = "text\n\n" + p.m.Format()
		}
		pl.comments = append(pl.comments, forge.Comment{ID: int64(10 + i), Body: body})
		pl.kinds = append(pl.kinds, p.kind)
	}
	return pl
}

func TestModeOfTheStartLabels(t *testing.T) {
	a := &Agent{Config: &config.Config{Labels: config.Labels{Ready: "agent:ready", Plan: "agent:plan", Go: "agent:go"}}}
	always, auto, never := config.Project{PlanFirst: config.PlanAlways}, config.Project{PlanFirst: config.PlanAuto}, config.Project{PlanFirst: config.PlanNever}
	for _, c := range []struct {
		p     config.Project
		label string
		want  string
	}{
		{always, "agent:ready", marker.ModePlan},
		{auto, "agent:ready", marker.ModeAuto},
		{never, "agent:ready", marker.ModeDirect},
		{config.Project{}, "agent:ready", marker.ModeDirect},
		{config.Project{}, "agent:plan", marker.ModePlan},
		{auto, "agent:plan", marker.ModePlan},
		{never, "agent:plan", marker.ModePlan},
		{always, "agent:go", marker.ModeDirect},
		{auto, "agent:go", marker.ModeDirect},
	} {
		if got := a.modeFor(c.p, c.label); got != c.want {
			t.Errorf("modeFor(plan_first=%q, %s) = %q, want %q", c.p.PlanFirst, c.label, got, c.want)
		}
	}
}

// skipped is the comment a plan run posts when it skips the questions and the approval: a claim in direct mode.
func skipped() entry {
	return agentPost(marker.Marker{V: 1, State: marker.Running, Mode: marker.ModeDirect, Reason: marker.PlanSkipped})
}

func failed() entry {
	return agentPost(marker.Marker{V: 1, State: marker.Waiting, Reason: "error"})
}

func TestOnlyTheFirstPlanRunOfAnAutoCycleMaySkip(t *testing.T) {
	for _, c := range []struct {
		name      string
		pl        *plan
		kind      runKind
		skippable bool
	}{
		{"a start in auto mode", started(marker.ModeAuto), planRun, true},
		{"a start in plan mode", started(marker.ModePlan), planRun, false},
		{"a start in direct mode", started(marker.ModeDirect), directRun, false},
		{"a reply after a failed first run", thread(claimOf("auto"), failed(), reply("try again")), planRun, true},
		{"a reply to the questions", thread(claimOf("auto"), asked(2), reply("/ok")), planRun, false},
		{"a reply to the plan", thread(claimOf("auto"), planned(), reply("rename it")), planRun, false},
		{"the plan approved", thread(claimOf("auto"), planned(), reply("/ok")), approvedRun, false},
		{"a reply in a plan cycle", thread(claimOf("plan"), failed(), reply("try again")), planRun, false},
		{"questions of an earlier cycle", thread(claimOf("plan"), asked(2), doneAt(), claimOf("auto"), failed(), reply("again")), planRun, true},
		{"a reply after the skip", thread(claimOf("auto"), skipped(), question(), reply("yes")), directRun, false},
		{"a reply after the skip ended done", thread(claimOf("auto"), skipped(), doneAt(), reply("more")), directRun, false},
	} {
		c.pl.classify("/ok")
		if c.pl.kind != c.kind || c.pl.skippable != c.skippable {
			t.Errorf("%s: kind=%v skippable=%v, want %v %v", c.name, c.pl.kind, c.pl.skippable, c.kind, c.skippable)
		}
	}
}

func TestKindOfTheRun(t *testing.T) {
	for _, c := range []struct {
		name string
		pl   *plan
		want runKind
	}{
		{"a start in plan mode", started(marker.ModePlan), planRun},
		{"a start in direct mode", started(marker.ModeDirect), directRun},
		{"a reply to a question of the plan", thread(claimOf("plan"), asked(3), reply("Q2 is B")), planRun},
		{"no claim with a mode", thread(resumed(), question(), reply("yes")), directRun},
		{"a cycle in direct mode", thread(claimOf("direct"), question(), reply("yes")), directRun},
		{"a reply to the plan", thread(claimOf("plan"), planned(), reply("rename the API")), planRun},
		{"the plan approved", thread(claimOf("plan"), planned(), reply("/ok")), approvedRun},
		{"a question after the approval", thread(claimOf("plan"), planned(), reply("/ok"), resumed(), question(), reply("yes")), approvedRun},
		{"a new plan after a reply supersedes the approval", thread(claimOf("plan"), planned(), reply("/ok"), resumed(), planned(), reply("hmm")), planRun},
		{"an approval of an earlier cycle", thread(claimOf("plan"), planned(), reply("/ok"), doneAt(), claimOf("plan"), asked(2), reply("/ok")), planRun},
		{"an untrusted approval", thread(claimOf("plan"), planned(), untrusted("/ok")), planRun},
		{"/ok to the questions", thread(claimOf("plan"), asked(3), reply("/ok")), planRun},
	} {
		c.pl.classify("/ok")
		if c.pl.kind != c.want {
			t.Errorf("%s: kind = %v, want %v", c.name, c.pl.kind, c.want)
		}
	}
}

func TestApprovedRunNamesThePlanAndTheApproval(t *testing.T) {
	pl := thread(claimOf("plan"), asked(2), reply("/ok"), planned(), reply("/ok q2 is B"))
	pl.classify("/ok")
	if pl.kind != approvedRun || pl.planID != 13 || pl.okID != 14 {
		t.Fatalf("kind=%v plan=%d ok=%d, want approvedRun 13 14", pl.kind, pl.planID, pl.okID)
	}
}

func TestQuestionNumbersContinueInTheCycle(t *testing.T) {
	pl := thread(asked(9), doneAt(), claimOf("plan"), asked(3), reply("/ok"))
	pl.classify("/ok")
	if pl.qNext != 4 {
		t.Fatalf("qNext = %d, want 4 (after Q3 of this cycle, not Q9 of an earlier one)", pl.qNext)
	}
	pl = started(marker.ModePlan, asked(9))
	pl.classify("/ok")
	if pl.qNext != 1 {
		t.Fatalf("qNext of a new cycle = %d, want 1", pl.qNext)
	}
}

func TestApprovalWord(t *testing.T) {
	for body, want := range map[string]bool{
		"/ok":                 true,
		"/ok q5はB":            true,
		"\n\n  /ok\nthanks":   true,
		"/ok\tq2 is A":        true,
		"/okay":               false,
		"/ok,":                false,
		"ok":                  false,
		"Looks good.\n/ok":    false,
		"":                    false,
		"> /ok\nquoted reply": false,
	} {
		if got := isApproval(body, "/ok"); got != want {
			t.Errorf("isApproval(%q) = %v, want %v", body, got, want)
		}
	}
}
