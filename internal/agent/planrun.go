package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/marker"
)

// PlanSchema is the JSON schema of the declaration that ends a run in the plan stage.
const PlanSchema = `{"type":"object","additionalProperties":false,"required":["status"],` +
	`"properties":{"status":{"type":"string","enum":["questions","plan"]},"summary":{"type":"string"},` +
	`"plan":{"type":"string"},"questions":{"type":"array","minItems":1,"items":{"type":"object",` +
	`"additionalProperties":false,"required":["title","body","recommendation"],"properties":{` +
	`"title":{"type":"string"},"body":{"type":"string"},"recommendation":{"type":"string"}}}}}}`

// PlanQuestion is one question of a plan round, with the run's recommended answer.
type PlanQuestion struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	Recommendation string `json:"recommendation"`
}

// PlanDeclaration is how a run in the plan stage ends: with questions, or with the plan.
type PlanDeclaration struct {
	Status    string         `json:"status"`
	Summary   string         `json:"summary,omitempty"`
	Questions []PlanQuestion `json:"questions,omitempty"`
	Plan      string         `json:"plan,omitempty"`
}

// planDeclaration reads the declaration of a plan run from res, or says why the run ended abnormally.
func planDeclaration(res *claudecode.Result) (*PlanDeclaration, string) {
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
	switch d.Status {
	case "questions":
		if len(d.Questions) == 0 {
			return nil, "the structured output has no questions"
		}
		for i, q := range d.Questions {
			if strings.TrimSpace(q.Title) == "" || strings.TrimSpace(q.Body) == "" || strings.TrimSpace(q.Recommendation) == "" {
				return nil, fmt.Sprintf("question %d lacks a title, a body or a recommendation", i+1)
			}
		}
	case "plan":
		if strings.TrimSpace(d.Plan) == "" {
			return nil, "the structured output has no plan"
		}
	default:
		return nil, fmt.Sprintf("the structured output has the status %q", d.Status)
	}
	return &d, ""
}

// planSettings are what a plan run may do: read, search and look at the history. Nothing that writes. The rules are
// not the boundary; the check after the run is (workspace.Space.Restore).
func planSettings() claudecode.Settings {
	allow := []string{"Read", "Glob", "Grep"}
	for _, c := range []string{"git status", "git diff", "git log"} {
		allow = append(allow, "Bash("+c+")", "Bash("+c+" *)")
	}
	deny := []string{"Edit", "Write", "NotebookEdit", "WebFetch"}
	for _, c := range []string{"git add", "git commit", "git push", "make", "gh"} {
		deny = append(deny, "Bash("+c+")", "Bash("+c+" *)")
	}
	return claudecode.Settings{Allow: allow, Deny: deny}
}

// planQuestions writes back the questions of a plan round, numbered from first.
func (o *outcome) planQuestions(d *PlanDeclaration, first int, denials []claudecode.Denial, discarded bool, word string) *outcome {
	var parts []string
	if s := strings.TrimSpace(d.Summary); s != "" {
		parts = append(parts, s)
	}
	var qs []string
	for i, q := range d.Questions {
		qs = append(qs, fmt.Sprintf("❓ **Q%d** - **%s**\n\n%s\n\n➡️ %s", first+i, strings.TrimSpace(q.Title),
			strings.TrimSpace(q.Body), strings.TrimSpace(q.Recommendation)))
	}
	parts = append(parts, strings.Join(qs, "\n\n---\n\n"))
	parts = append(parts, o.planTail(denials, discarded, fmt.Sprintf(o.msg.planQuestionsHint, word, word))...)
	o.set("waiting", marker.PlanQuestions, o.msg.planQuestionsTitle, parts...)
	o.Q = first + len(d.Questions) - 1
	return o
}

// planText writes back the plan of a plan run.
func (o *outcome) planText(d *PlanDeclaration, denials []claudecode.Denial, discarded bool, word string) *outcome {
	parts := []string{strings.TrimSpace(d.Plan)}
	parts = append(parts, o.planTail(denials, discarded, fmt.Sprintf(o.msg.planHint, word))...)
	return o.set("waiting", marker.PlanReady, o.msg.planTitle, parts...)
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
