package agent

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/workspace"
)

// purposeLen is how many characters of the assistant's latest text a progress line keeps.
const purposeLen = 120

// progressLog is the lines of a run's progress comment: one a minute, unless nothing changed.
type progressLog struct {
	msg     texts
	started time.Time
	lines   []string
	last    string // the tools and purpose of the last line, which a new line must differ from
}

// add appends the line for now, given the tool counts and the assistant's latest text, and reports whether it did.
// A line with the same tools and purpose as the last one is not added. A line that looks like it holds a secret is
// replaced by one that says so; it is checked before the purpose is cut, so that a cut secret is still found.
func (l *progressLog) add(now time.Time, tools map[string]int, text string) bool {
	full := firstLine(text)
	what := l.tools(tools) + l.msg.purposeSep + cut(full)
	if len(l.lines) > 0 && what == l.last {
		return false
	}
	l.last = what
	line := now.Format("15:04") + " " + l.elapsed(now.Sub(l.started)) + " " + what
	if workspace.HasSecret(line) || workspace.HasSecret(full) {
		line = l.msg.progressWithheld
	}
	l.lines = append(l.lines, line)
	return true
}

func (l *progressLog) tools(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for n, c := range counts {
		if c > 0 {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return l.msg.noTools
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf("%s %d", n, counts[n])
	}
	return strings.Join(parts, l.msg.toolSep)
}

func (l *progressLog) elapsed(d time.Duration) string {
	m := int(d.Minutes())
	if m >= 60 {
		return fmt.Sprintf(l.msg.hours, m/60, m%60)
	}
	return fmt.Sprintf(l.msg.minutes, m)
}

// firstLine is the first non-blank line of text; "?" when there is none.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "?"
}

// cut keeps the first purposeLen characters of a purpose.
func cut(line string) string {
	if r := []rune(line); len(r) > purposeLen {
		return string(r[:purposeLen])
	}
	return line
}

// followProgress posts and keeps up the progress comment of a run while claude writes stream, every ProgressEvery
// (a minute by default). The returned function stops it and waits until it has.
func (a *Agent) followProgress(ctx context.Context, ref forge.IssueRef, rec record, stream string) func() {
	every := a.ProgressEvery
	if every <= 0 {
		every = time.Minute
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pc := &progressComment{rec: rec, read: claudecode.NewProgress(stream), log: progressLog{msg: a.msg, started: rec.Started}}
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case now := <-t.C:
				a.progressTick(ctx, ref, pc, now)
			}
		}
	}()
	return func() {
		close(stop)
		wg.Wait()
	}
}

// progressComment is the progress comment of one run: what was read of claude's output and the comment's ID once
// it is posted.
type progressComment struct {
	rec  record
	read *claudecode.Progress
	log  progressLog
	id   int64
}

// progressTick reads what claude wrote since the last tick and, when that adds a line, posts or edits the progress
// comment. A failure is recorded; the run goes on.
func (a *Agent) progressTick(ctx context.Context, ref forge.IssueRef, pc *progressComment, now time.Time) {
	fail := func(what string, err error) {
		a.logf("%s: %s the progress comment of run %s: %v", ref, what, pc.rec.Run, err)
		a.Audit.Log("progress.failed", map[string]any{"issue": ref.String(), "run": pc.rec.Run, "action": what, "error": err.Error()})
	}
	if err := pc.read.Read(); err != nil {
		fail("read", err)
		return
	}
	if len(pc.read.Tools) == 0 && strings.TrimSpace(pc.read.Text) == "" {
		return // claude has done nothing yet
	}
	if !pc.log.add(now, pc.read.Tools, pc.read.Text) {
		return
	}
	body := a.progressBody(pc)
	a.checkComment(ref, pc.rec.Run, "progress.md", body)
	if pc.id != 0 {
		err := a.forgeOf(ref).EditComment(ctx, ref, pc.id, body)
		if err == nil {
			return
		}
		if !errors.Is(err, forge.ErrNotFound) {
			fail("edit", err)
			return
		}
		pc.id = 0 // someone removed the comment: post it again
	}
	c, err := a.forgeOf(ref).PostComment(ctx, ref, body)
	if err != nil {
		fail("post", err)
		return
	}
	pc.id = c.ID
	a.Audit.Log("progress.posted", map[string]any{"issue": ref.String(), "run": pc.rec.Run, "comment": c.ID})
}

// progressBody is the progress comment: a heading, one list item per line, and the marker.
func (a *Agent) progressBody(pc *progressComment) string {
	var b strings.Builder
	b.WriteString("### " + a.msg.progressTitle + "\n\n")
	for _, l := range pc.log.lines {
		b.WriteString("- " + l + "\n")
	}
	m := marker.Marker{Host: a.Config.Host.Name, Session: pc.rec.Session, Run: pc.rec.Run, State: marker.Progress, Ack: pc.rec.Ack, At: pc.rec.Started}
	b.WriteString("\n" + m.Format() + "\n")
	return b.String()
}

// dropProgress removes the agent's progress comments among comments that belong to run, or to another run of this
// host that no longer goes on (left by an agent that stopped mid-run). A failure is recorded and changes nothing else.
func (a *Agent) dropProgress(ctx context.Context, ref forge.IssueRef, run string, comments []forge.Comment) {
	agentID := a.judgeOf(ref).AgentIDOf(ref)
	for _, c := range comments {
		m, ok := marker.Parse(c.Body)
		if !ok || c.Author.ID != agentID || m.State != marker.Progress {
			continue
		}
		if m.Run != run && (m.Host != a.Config.Host.Name || a.runs(m.Run)) {
			continue
		}
		if err := a.forgeOf(ref).DeleteComment(ctx, ref, c.ID); err != nil && !errors.Is(err, forge.ErrNotFound) {
			a.logf("%s: remove the progress comment %d: %v", ref, c.ID, err)
			a.Audit.Log("progress.failed", map[string]any{"issue": ref.String(), "run": m.Run, "action": "delete", "error": err.Error()})
			continue
		}
		a.Audit.Log("progress.removed", map[string]any{"issue": ref.String(), "run": m.Run, "comment": c.ID})
	}
}

// runs reports whether run goes on on this host.
func (a *Agent) runs(run string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, act := range a.active {
		if act.run == run {
			return true
		}
	}
	return false
}

// keepNotes records the note count of a GitLab issue that now waits, as read after its progress note went away,
// so that the next poll does not take the removal for a reply.
func (a *Agent) keepNotes(ctx context.Context, p config.Project, ref forge.IssueRef) {
	list, err := a.forgeOf(ref).SearchOpenIssuesWithLabel(ctx, p.FullName(), a.Config.Labels.Waiting)
	if err != nil {
		return // the next poll evaluates the issue again, which only costs a read
	}
	for _, s := range list {
		if s.Ref == ref {
			a.mu.Lock()
			a.notes[ref] = s.Notes
			a.mu.Unlock()
		}
	}
}
