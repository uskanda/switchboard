package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
)

func TestProgressLineShape(t *testing.T) {
	start := time.Date(2026, 10, 31, 13, 34, 0, 0, time.UTC)
	now := start.Add(7*time.Minute + 20*time.Second)
	long := strings.Repeat("あ", 130)
	l := &progressLog{msg: japanese, started: start}
	if !l.add(now, map[string]int{"Edit": 5, "Bash": 12}, "DESIGN.mdのバナーの記述を、実装に合わせて直している\n次の行は使わない") {
		t.Fatal("the first line was not added")
	}
	if got, want := l.lines[0], "13:41 7分 Bash 12・Edit 5 ｜ DESIGN.mdのバナーの記述を、実装に合わせて直している"; got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	// The same tools and purpose a minute later add nothing.
	if l.add(now.Add(time.Minute), map[string]int{"Edit": 5, "Bash": 12}, "DESIGN.mdのバナーの記述を、実装に合わせて直している") || len(l.lines) != 1 {
		t.Fatalf("an unchanged minute added a line: %q", l.lines)
	}
	if !l.add(start.Add(75*time.Minute), map[string]int{"Edit": 6, "Bash": 12}, "  "+long) {
		t.Fatal("a changed count added nothing")
	}
	got := l.lines[1]
	if !strings.HasPrefix(got, "14:49 1時間15分 Bash 12・Edit 6 ｜ ") || len([]rune(strings.TrimPrefix(got, "14:49 1時間15分 Bash 12・Edit 6 ｜ "))) != 120 {
		t.Fatalf("line = %q", got)
	}

	en := &progressLog{msg: english, started: start}
	en.add(start.Add(30*time.Second), map[string]int{}, "")
	if en.lines[0] != "13:34 0m no tools | ?" {
		t.Fatalf("english line = %q", en.lines[0])
	}
}

// asst is a step fragment that prints an assistant message with text and one call of tool, whose input must
// never reach the forge.
func asst(text, tool string) string {
	return fmt.Sprintf("printf '%%s\\n' '{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"%s\"},{\"type\":\"tool_use\",\"id\":\"t\",\"name\":\"%s\",\"input\":{\"command\":\"cat /SECRETCMD\"}}]}}'\n", text, tool)
}

// progressOf returns the agent's progress comments on ref.
func progressOf(f *fake.Forge, agentID int64, ref forge.IssueRef) []forge.Comment {
	var out []forge.Comment
	for _, c := range f.CommentsOf(ref) {
		if m, ok := marker.Parse(c.Body); ok && c.Author.ID == agentID && m.State == "progress" {
			out = append(out, c)
		}
	}
	return out
}

func progressLines(body string) int { return strings.Count(body, "\n- ") }

func fast(a *Agent) { a.ProgressEvery = 40 * time.Millisecond }

func TestProgressCommentGrowsInPlaceAndGoesAway(t *testing.T) {
	checks := t.TempDir()
	script := filepath.Join(checks, "check.sh")
	if err := os.WriteFile(script, []byte(`cat "$1" >> "$(dirname "$0")/checked"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	x := newH(t, "", func(c *config.Config) { c.Comments.CheckCommand = "sh " + script })
	x.agentWith(fast)
	x.step(1, asst("目的A", "Edit")+block(1, asst("目的B ghp_abcdefghijklmnopqrstuvwxyz0123", "Bash")+block(2, waiting("s", "q"))))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitFor("the first progress line", func() bool {
		ps := progressOf(x.f, x.bot.ID, x.ref)
		return len(ps) == 1 && strings.Contains(ps[0].Body, "Edit 1 | 目的A")
	})
	claim := x.claims(x.ref)
	if len(claim) != 1 || !strings.Contains(claim[0].Body, "Started on host") {
		t.Fatalf("claims = %+v", claim)
	}
	cm, _ := marker.Parse(claim[0].Body)
	pm, _ := marker.Parse(progressOf(x.f, x.bot.ID, x.ref)[0].Body)
	if pm.Run != cm.Run || pm.Host != "test" {
		t.Fatalf("progress marker = %+v, claim marker = %+v", pm, cm)
	}
	x.release(1)
	x.waitFor("the second progress line", func() bool {
		ps := progressOf(x.f, x.bot.ID, x.ref)
		return len(ps) == 1 && progressLines(ps[0].Body) == 2
	})
	time.Sleep(200 * time.Millisecond) // a few minutes in which nothing changes
	ps := progressOf(x.f, x.bot.ID, x.ref)
	if len(ps) != 1 || progressLines(ps[0].Body) != 2 || x.f.Edits != 1 {
		t.Fatalf("progress = %+v, edits %d", ps, x.f.Edits)
	}
	body := ps[0].Body
	if strings.Contains(body, "ghp_") || !strings.Contains(body, "withheld") || strings.Contains(body, "SECRETCMD") {
		t.Fatalf("progress body:\n%s", body)
	}
	checked, err := os.ReadFile(filepath.Join(checks, "checked"))
	if err != nil || !strings.Contains(string(checked), "目的A") || strings.Contains(string(checked), "ghp_") {
		t.Fatalf("check_command saw %q, %v", checked, err)
	}
	x.release(2)
	x.settle()
	if ps := progressOf(x.f, x.bot.ID, x.ref); len(ps) != 0 {
		t.Fatalf("the progress comment stayed: %+v", ps)
	}
	if len(x.claims(x.ref)) != 1 || len(x.posts(x.ref)) != 1 || x.lastPost(x.ref).Marker.State != "waiting" {
		t.Fatalf("claims %d, posts %+v", len(x.claims(x.ref)), x.posts(x.ref))
	}
}

func TestARunShorterThanAMinuteHasNoProgressComment(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, asst("目的A", "Edit")+waiting("s", "q"))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.f.Comments != 2 || x.f.Edits != 0 {
		t.Fatalf("comments posted %d, edits %d", x.f.Comments, x.f.Edits)
	}
}

// failing refuses to post or edit progress comments.
type failing struct{ forge.Forge }

func (f failing) PostComment(ctx context.Context, ref forge.IssueRef, body string) (*forge.Comment, error) {
	if strings.Contains(body, "state=progress") {
		return nil, errors.New("boom")
	}
	return f.Forge.PostComment(ctx, ref, body)
}

func (f failing) EditComment(context.Context, forge.IssueRef, int64, string) error {
	return errors.New("boom")
}

func TestAFailingProgressCommentDoesNotStopTheRun(t *testing.T) {
	x := newH(t, "", nil)
	x.agentWith(func(a *Agent) { fast(a); a.Forge = failing{x.f} })
	x.step(1, asst("目的A", "Edit")+"sleep 0.3\n"+waiting("s", "q"))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:waiting" || x.lastPost(x.ref).Marker.State != "waiting" {
		t.Fatalf("labels = %q", x.labels(x.ref))
	}
	if !x.audit.has("progress.failed", nil) {
		t.Fatal("the failure was not audited")
	}
}

func TestProgressCommentGoesAwayWhenDone(t *testing.T) {
	x := newH(t, "", nil)
	x.agentWith(fast)
	x.step(1, asst("目的A", "Edit")+"sleep 0.3\n"+commit("flag.txt", "x")+done("Added the flag."))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.labels(x.ref) != "agent:done" || x.f.Comments != 3 || len(progressOf(x.f, x.bot.ID, x.ref)) != 0 {
		t.Fatalf("labels %q, posted %d, progress %+v", x.labels(x.ref), x.f.Comments, progressOf(x.f, x.bot.ID, x.ref))
	}
}

func TestProgressCommentGoesAwayWhenTheRunIsCanceled(t *testing.T) {
	x := newH(t, "", nil)
	x.agentWith(fast)
	x.step(1, asst("目的A", "Edit")+block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitFor("the progress comment", func() bool { return len(progressOf(x.f, x.bot.ID, x.ref)) == 1 })
	x.f.RemoveLabel(x.ref, "agent:running")
	x.poll()
	x.settle()
	if ps := progressOf(x.f, x.bot.ID, x.ref); len(ps) != 0 {
		t.Fatalf("the progress comment stayed: %+v", ps)
	}
}

func TestRecoveryRemovesALeftProgressComment(t *testing.T) {
	x := newH(t, "", nil)
	x.agentWith(fast)
	x.step(1, asst("目的A", "Edit")+block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitFor("the progress comment", func() bool { return len(progressOf(x.f, x.bot.ID, x.ref)) == 1 })
	x.stop() // the agent dies mid-run
	x.a.Wait()
	x.a.Close()
	x.release(1)
	x.a = nil
	x.ctx, x.stop = context.WithCancel(context.Background())
	x.agent()
	if x.labels(x.ref) != "agent:waiting" || len(progressOf(x.f, x.bot.ID, x.ref)) != 0 {
		t.Fatalf("labels %q, progress %+v", x.labels(x.ref), progressOf(x.f, x.bot.ID, x.ref))
	}
	if ps := x.posts(x.ref); len(ps) != 1 || ps[0].Marker.Reason != "error" {
		t.Fatalf("posts after recovery = %+v", ps)
	}
}

// On GitLab, removing the progress note lowers the note count; the count kept for the waiting issue is the one
// after the removal, so the next poll does not take it for a reply.
func TestGitLabNoteCountIsTakenAfterTheProgressNoteGoes(t *testing.T) {
	x := newGL(t, nil)
	x.agent()
	fast(x.a)
	x.step(0, asst("目的A", "Edit")+"sleep 0.3\n"+waiting("s", "q"))
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if len(progressOf(x.g, x.bot.ID, x.gref)) != 0 || x.g.Edits+x.g.Comments < 3 {
		t.Fatalf("progress %+v, posted %d", progressOf(x.g, x.bot.ID, x.gref), x.g.Comments)
	}
	x.a.mu.Lock()
	n := x.a.notes[x.gref]
	x.a.mu.Unlock()
	if want := len(x.g.CommentsOf(x.gref)); n != want {
		t.Fatalf("notes kept = %d, want %d", n, want)
	}
}
