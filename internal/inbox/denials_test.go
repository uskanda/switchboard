package inbox

import (
	"slices"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// permissionWait posts, as the agent, a permission wait whose refused operations are lines.
func permissionWait(w *world, n int, heading, lines string) {
	ref := w.f.AddIssue("o", "r", n, "Renumber", "body", w.me)
	w.f.PostAs = "bot"
	w.f.PostComment(ctx, ref, "### The agent was refused an operation\n\n**What was done**\n\nLooked.\n\n"+heading+"\n\n"+lines+
		"\n\n**Question**\n\nMay I?\n\n"+marker.Marker{Host: "desk", Run: "p", State: marker.Waiting, Reason: "permission", At: w.now.Add(-time.Hour)}.Format())
	w.f.PostAs = "me"
	w.f.AddLabel(ref, "agent:waiting", w.bot)
}

func TestTheDetailOfAPermissionWaitListsTheRefusedOperations(t *testing.T) {
	for _, heading := range []string{"**Refused operations**", "**拒否された操作**"} {
		w := newWorld(t)
		permissionWait(w, 50, heading, "- Bash: `git mv a.sql b.sql`\n- Bash: `for f in a b; do git log -- $f; done`\n- Skill")
		d, err := w.b.Detail(ctx, mustRef(t, w, "gh:o/r#50"))
		if err != nil {
			t.Fatal(err)
		}
		want := []Denial{
			{Tool: "Bash", Command: "git mv a.sql b.sql", Candidates: []string{"Bash(git mv)", "Bash(git mv *)"}},
			{Tool: "Bash", Command: "for f in a b; do git log -- $f; done", Compound: true, Candidates: []string{}},
			{Tool: "Skill", Candidates: []string{"Skill"}},
		}
		if len(d.Denials) != len(want) {
			t.Fatalf("%s: denials = %+v", heading, d.Denials)
		}
		for i := range want {
			g := d.Denials[i]
			if g.Tool != want[i].Tool || g.Command != want[i].Command || g.Compound != want[i].Compound || !slices.Equal(g.Candidates, want[i].Candidates) {
				t.Errorf("%s: denial %d = %+v, want %+v", heading, i, g, want[i])
			}
		}
	}
}

// A person's comment shaped like the agent's is not read, and another wait has no refused operations.
func TestOnlyTheAgentsPermissionWaitHasDenials(t *testing.T) {
	w := newWorld(t)
	permissionWait(w, 50, "**Refused operations**", "- Bash: `git mv a b`")
	ref := mustRef(t, w, "gh:o/r#50")
	w.f.AddComment(ref, "**Refused operations**\n\n- Bash: `git push origin main`\n\n"+
		marker.Marker{Host: "desk", Run: "x", State: marker.Waiting, Reason: "permission"}.Format(), w.me)
	d, err := w.b.Detail(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Denials) != 1 || d.Denials[0].Command != "git mv a b" {
		t.Fatalf("denials = %+v", d.Denials)
	}

	q := newWorld(t)
	permissionWait(q, 51, "**Refused operations**", "- Bash: `git mv a b`")
	r := mustRef(t, q, "gh:o/r#51")
	q.f.PostAs = "bot"
	q.f.PostComment(ctx, r, "### A question\n\nWhich?\n\n"+marker.Marker{Host: "desk", Run: "q", State: marker.Waiting, Reason: "question"}.Format())
	if d, err := q.b.Detail(ctx, r); err != nil || len(d.Denials) != 0 {
		t.Fatalf("a question wait: %+v, %v", d.Denials, err)
	}
}

func mustRef(t *testing.T, w *world, s string) forge.IssueRef {
	t.Helper()
	ref, err := w.b.ParseRef(s)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}
