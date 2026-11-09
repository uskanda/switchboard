package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/workspace"
)

// PlanSchema is the JSON schema of the declaration that ends a run in the plan stage; PlanSkipSchema that of a plan
// run that may skip the questions and the approval, which may also declare implement.
const (
	PlanSchema     = planSchemaHead + `"enum":["questions","plan"]` + planSchemaTail
	PlanSkipSchema = planSchemaHead + `"enum":["questions","plan","implement"]` + planSchemaTail
)

const (
	planSchemaHead = `{"type":"object","additionalProperties":false,"required":["status","title"],` +
		`"properties":{"status":{"type":"string",`
	planSchemaTail = `},"title":{"type":"string"},"summary":{"type":"string"},` +
		`"plan":{"type":"string"},"questions":{"type":"array","minItems":1,"items":{"type":"object",` +
		`"additionalProperties":false,"required":["title","body","options","recommended","reason","weight"],"properties":{` +
		`"title":{"type":"string"},"body":{"type":"string"},` +
		`"options":{"type":"array","minItems":2,"maxItems":6,"items":{"type":"string"}},` +
		`"recommended":{"type":"string","pattern":"^[A-F]$"},"reason":{"type":"string"},` +
		`"weight":{"type":"string","enum":["decision","check"]},"diagram":{"type":"string"}}}}}}`
)

// planImplement is the status of a plan run that skips the questions and the approval.
const planImplement = "implement"

// PlanQuestion is one question of a plan round: its options, which switchboard labels A, B, C… in order, the label of
// the option the run recommends and why, its weight, and a Mermaid diagram of what it asks, if any.
type PlanQuestion struct {
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Options     []string `json:"options"`
	Recommended string   `json:"recommended"`
	Reason      string   `json:"reason"`
	Weight      string   `json:"weight"`
	Diagram     string   `json:"diagram,omitempty"`
}

// The weights of a question: one that needs the owner's decision, and one that only asks to confirm the recommended
// answer, which the question comment puts last, under a heading of its own.
const (
	weightDecision = "decision"
	weightCheck    = "check"
)

// weighedOrder is qs with the questions that need a decision first and those that only confirm after them, each in
// the order declared: the order they are numbered and posted in.
func weighedOrder(qs []PlanQuestion) []PlanQuestion {
	out := make([]PlanQuestion, 0, len(qs))
	for _, w := range []string{weightDecision, weightCheck} {
		for _, q := range qs {
			if q.Weight == w {
				out = append(out, q)
			}
		}
	}
	return out
}

// Options of a question: at least two, at most six, labeled A to F.
const (
	minOptions = 2
	maxOptions = 6
)

// optionLabel is the label of the i-th option (from 0): A, B, C…
func optionLabel(i int) string { return string(rune('A' + i)) }

// PlanDeclaration is how a run in the plan stage ends: with questions, with the plan, or, when it may skip, with the
// plan to implement at once.
type PlanDeclaration struct {
	Status    string         `json:"status"`
	Title     string         `json:"title"` // the plan, or the plan the questions lead to, on one short line
	Summary   string         `json:"summary,omitempty"`
	Questions []PlanQuestion `json:"questions,omitempty"`
	Plan      string         `json:"plan,omitempty"`
}

// planDeclaration reads the declaration of a plan run from res, or says why the run ended abnormally. Only a run
// that may skip (skippable) may declare implement.
func planDeclaration(res *claudecode.Result, skippable bool) (*PlanDeclaration, string) {
	raw, problem := structured(res)
	if problem != "" {
		return nil, problem
	}
	var d PlanDeclaration
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, "the structured output does not match the schema: " + err.Error()
	}
	known := d.Status == "questions" || d.Status == "plan" || d.Status == planImplement && skippable
	if known && strings.TrimSpace(d.Title) == "" {
		return nil, "the structured output has no title"
	}
	if !known {
		return nil, fmt.Sprintf("the structured output has the status %q", d.Status)
	}
	switch d.Status {
	case "questions":
		if len(d.Questions) == 0 {
			return nil, "the structured output has no questions"
		}
		for i, q := range d.Questions {
			if problem := questionProblem(q); problem != "" {
				return nil, fmt.Sprintf("question %d %s", i+1, problem)
			}
		}
		d.Questions = weighedOrder(d.Questions)
	default: // plan, implement
		if strings.TrimSpace(d.Plan) == "" {
			return nil, "the structured output has no plan"
		}
	}
	return &d, ""
}

// questionProblem says what is wrong with a declared question, or "" when nothing is: a blank title, body or reason,
// a weight that is neither decision nor check, too few or too many options, a blank option, or a recommended label
// that is not one of the options'.
func questionProblem(q PlanQuestion) string {
	if strings.TrimSpace(q.Title) == "" || strings.TrimSpace(q.Body) == "" || strings.TrimSpace(q.Reason) == "" {
		return "lacks a title, a body or a reason"
	}
	if q.Weight != weightDecision && q.Weight != weightCheck {
		return fmt.Sprintf("has the weight %q, not %q or %q", q.Weight, weightDecision, weightCheck)
	}
	if len(q.Options) < minOptions || len(q.Options) > maxOptions {
		return fmt.Sprintf("has %d options, not %d to %d", len(q.Options), minOptions, maxOptions)
	}
	for _, o := range q.Options {
		if strings.TrimSpace(o) == "" {
			return "has a blank option"
		}
	}
	for i := range q.Options {
		if q.Recommended == optionLabel(i) {
			return ""
		}
	}
	return fmt.Sprintf("recommends %q, which is not one of its %d options", q.Recommended, len(q.Options))
}

// The kinds of diagram a question may have, and the words that would restyle the page at the start of a line
// (ADR-066): a diagram is drawn with inline styles allowed, so it may say what to draw, never how.
var (
	diagramKinds  = []string{"flowchart", "stateDiagram-v2", "sequenceDiagram"}
	diagramStyles = []string{"classDef", "style", "linkStyle", "click"}
)

const fence = "```"

// diagramOf checks the Mermaid diagram of a question and returns it as it is to be written: without the fence it may
// come wrapped in and the blank lines around it. It returns "" and why when the diagram is not to be written: it is
// blank, still holds a fence (it would end the comment's code block early), holds a directive (%%{), is of another
// kind, or has a line that restyles (classDef, style, linkStyle, click).
func diagramOf(src string) (string, string) {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(src, "\r\n", "\n")), "\n")
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), fence) && strings.TrimSpace(lines[len(lines)-1]) == fence {
		lines = lines[1 : len(lines)-1]
	}
	text := strings.TrimSpace(strings.Join(lines, "\n"))
	switch {
	case text == "":
		return "", "the diagram is blank"
	case strings.Contains(text, fence):
		return "", "the diagram holds a code fence"
	case strings.Contains(text, "%%{"):
		return "", "the diagram holds a directive (%%{)"
	}
	kind := strings.Fields(text)[0]
	if !slices.Contains(diagramKinds, kind) {
		return "", fmt.Sprintf("the diagram is %q, not one of %s", kind, strings.Join(diagramKinds, ", "))
	}
	for _, l := range strings.Split(text, "\n") {
		if w := strings.Fields(l); len(w) > 0 && slices.Contains(diagramStyles, w[0]) {
			return "", "the diagram restyles with " + w[0]
		}
	}
	return text, ""
}

// planSettings are what a plan run may do: read, search and look at the history. Nothing that writes. The rules are
// not the boundary; the check after the run is (workspace.Space.Restore).
func planSettings() claudecode.Settings {
	allow := append([]string{"Read", "Glob", "Grep", "WebSearch"}, claudecode.Commands("git status", "git diff", "git log")...)
	deny := append([]string{"Edit", "Write", "NotebookEdit", "WebFetch"},
		claudecode.Commands("git add", "git commit", "git push", "git rebase", "make", "gh")...)
	return claudecode.Settings{Allow: allow, Deny: deny, Unsandboxed: gitUnsandboxed}
}

// planQuestions writes back the questions of a plan round, numbered from first, in the order planDeclaration left
// them: the first question that only confirms the recommended answer starts under a heading of its own.
func (o *outcome) planQuestions(d *PlanDeclaration, first int, denials []claudecode.Denial, discarded bool, word string) *outcome {
	parts := []string{o.planTitleLine(d)}
	if s := strings.TrimSpace(d.Summary); s != "" {
		parts = append(parts, s)
	}
	var qs []string
	headed := false
	for i, q := range d.Questions {
		text := o.questionText(first+i, q)
		if q.Weight == weightCheck && !headed {
			headed = true
			text = "#### " + o.msg.planChecks + "\n\n" + text
		}
		qs = append(qs, text)
	}
	parts = append(parts, strings.Join(qs, "\n\n---\n\n"))
	parts = append(parts, o.planTail(denials, discarded, fmt.Sprintf(o.msg.planQuestionsHint, word, word))...)
	o.set("waiting", marker.PlanQuestions, o.msg.planQuestionsTitle, parts...)
	o.Q = first + len(d.Questions) - 1
	return o
}

// dropDiagrams checks the diagram of every question of d, numbered from first: one that passes is kept as it is to be
// written, one that does not is dropped, and the drop is logged with the issue, the run and why (the question is
// posted without it).
func (a *Agent) dropDiagrams(ref forge.IssueRef, run string, d *PlanDeclaration, first int) {
	for i := range d.Questions {
		q := &d.Questions[i]
		if strings.TrimSpace(q.Diagram) == "" {
			q.Diagram = ""
			continue
		}
		text, problem := diagramOf(q.Diagram)
		q.Diagram = text
		if problem != "" {
			a.Audit.Log("plan.diagram_dropped", map[string]any{"issue": ref.String(), "run": run, "q": first + i, "reason": problem})
		}
	}
}

// questionText is one question of a question comment: its title line, its body, its diagram (checked already, see
// dropDiagrams) in a mermaid code block, its options a line each, and the paragraph with the recommended label and
// the reason.
func (o *outcome) questionText(n int, q PlanQuestion) string {
	parts := []string{fmt.Sprintf("❓ **Q%d** - **%s**", n, strings.TrimSpace(q.Title)), strings.TrimSpace(q.Body)}
	if q.Diagram != "" {
		parts = append(parts, fence+"mermaid\n"+q.Diagram+"\n"+fence)
	}
	var options []string
	for i, opt := range q.Options {
		options = append(options, optionLabel(i)+": "+strings.Join(strings.Fields(opt), " "))
	}
	parts = append(parts, strings.Join(options, "\n"))
	parts = append(parts, "➡️ **"+fmt.Sprintf(o.msg.planRecommended, q.Recommended)+"** "+strings.TrimSpace(q.Reason))
	return strings.Join(parts, "\n\n")
}

// planText writes back the plan of a plan run.
func (o *outcome) planText(d *PlanDeclaration, denials []claudecode.Denial, discarded bool, word string) *outcome {
	parts := []string{o.planTitleLine(d), strings.TrimSpace(d.Plan)}
	parts = append(parts, o.planTail(denials, discarded, fmt.Sprintf(o.msg.planHint, word))...)
	return o.set("waiting", marker.PlanReady, o.msg.planTitle, parts...)
}

// planSkip ends a plan run that skipped the questions and the approval: its comment, with the plan, is posted as the
// claim of the run that implements it (skipAgain), and nothing is written back for the plan run.
func (o *outcome) planSkip(d *PlanDeclaration, denials []claudecode.Denial, discarded bool) *outcome {
	parts := []string{o.planTitleLine(d), o.msg.planSkipped, strings.TrimSpace(d.Plan)}
	if discarded {
		parts = append(parts, o.msg.planDiscarded)
	}
	if len(denials) > 0 {
		parts = append(parts, "<details>\n<summary>"+o.msg.denied+"</summary>\n\n"+deniedList(denials)+"\n\n</details>")
	}
	o.skip = "### " + o.msg.planSkippedTitle + "\n\n" + strings.Join(parts, "\n\n")
	return o
}

// skipAgain ends the plan run of rec, which skipped the questions and the approval, and starts the run that implements
// its plan in its place: it posts o's comment as the claim of the new run, in direct mode, so that every later run of
// the cycle implements, and the plan run's slot and lock pass to the new run. It returns the new run's outcome; o made
// a failure when the comment could not be posted; or nil when someone took the issue over meanwhile.
func (a *Agent) skipAgain(ctx context.Context, prev *plan, act *active, rec *record, o *outcome) *outcome {
	body := o.skip
	o.skip = ""
	ref := prev.issue.Ref
	if workspace.HasSecret(body) {
		body = fmt.Sprintf(a.msg.withheld, rec.Run)
		a.Audit.Log("comment.withheld", map[string]any{"issue": ref.String(), "run": rec.Run})
	}
	run := newID()
	m := marker.Marker{Host: a.Config.Host.Name, Run: run, State: marker.Running, Mode: marker.ModeDirect,
		Reason: marker.PlanSkipped, Ack: o.Ack, At: time.Now()}
	body += "\n\n" + m.Format() + "\n"
	a.checkComment(ref, rec.Run, "comment.md", body)
	posted, err := a.forgeOf(ref).PostComment(ctx, ref, body)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return o.fail(fmt.Sprintf(a.msg.planSkipFailed, err))
	}
	is, comments, err := a.forgeOf(ref).Thread(ctx, ref)
	switch {
	case ctx.Err() != nil:
		return nil
	case err != nil:
		return o.fail(fmt.Sprintf(a.msg.planSkipFailed, err))
	}
	j := a.judgeOf(ref)
	if is.State != "open" || labels.StateOf(is.Labels, a.Config.Labels) != a.Config.Labels.Running || !j.Body(is) {
		return nil
	}
	pl := &plan{project: prev.project, issue: is, comments: comments, kind: skippedRun, planID: posted.ID}
	for _, c := range comments {
		pl.kinds = append(pl.kinds, j.Comment(ref, c))
	}
	a.dropProgress(ctx, ref, rec.Run, comments)
	rec.State, rec.Ended, rec.Result = "finished", time.Now().UTC(), marker.PlanSkipped
	a.saveRecord(ref, *rec)
	a.mu.Lock()
	act.run = run
	a.mu.Unlock()
	*rec = record{Issue: ref.String(), Run: run, Session: newUUID(), State: "running", Started: time.Now().UTC(), Ack: o.Ack}
	a.saveRecord(ref, *rec)
	a.logf("%s: the plan run skipped the questions and the approval; run %s implements the plan of comment %d", ref, run, posted.ID)
	a.Audit.Log("run.plan_skipped", map[string]any{"issue": ref.String(), "run": run, "plan": posted.ID})
	return a.work(ctx, pl, *rec)
}

// planTitleLine is the paragraph right after the heading of a plan-stage comment: the plan's title on one line. The
// inbox and the notifications quote it as the start of the question (Question).
func (o *outcome) planTitleLine(d *PlanDeclaration) string {
	return "**" + o.msg.planLabel + "** " + strings.Join(strings.Fields(d.Title), " ")
}

// planTail is the end of a plan-stage comment: the discarded changes, the refused operations folded away, and the
// hint on how to answer as the last line.
func (o *outcome) planTail(denials []claudecode.Denial, discarded bool, hint string) []string {
	var parts []string
	if discarded {
		parts = append(parts, o.msg.planDiscarded)
	}
	if len(denials) > 0 {
		parts = append(parts, "<details>\n<summary>"+o.msg.denied+"</summary>\n\n"+deniedList(denials)+"\n\n</details>")
	}
	return append(parts, hint)
}

// deniedList is the refused operations as a Markdown list.
func deniedList(denials []claudecode.Denial) string {
	var lines []string
	for _, dn := range denials {
		what := dn.ToolName
		if c := dn.Command(); c != "" {
			what += ": `" + strings.ReplaceAll(c, "`", "'") + "`"
		}
		lines = append(lines, "- "+what)
	}
	return strings.Join(lines, "\n")
}
