package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// resumeLine is a part of the sentence a run resumed after a usage limit gets in its prompt.
const resumeLine = "stopped partway because it hit a usage limit"

// limited is a step that ends the way claude ends on a usage limit (V12): a rejected rate_limit_event with the reset
// time, then a result in error with api_error_status 429.
func limited(resets time.Time) string {
	ev, _ := json.Marshal(map[string]any{"type": "rate_limit_event", "rate_limit_info": map[string]any{
		"status": "rejected", "resetsAt": resets.Unix(), "rateLimitType": "five_hour",
	}})
	return "cat <<'JSON'\n" + string(ev) + "\n" + limitResult(0) + "\nJSON\nexit 1\n"
}

// limited429 is a step that ends on a 429 without a rate_limit_event, so without a reset time.
func limited429() string {
	return "cat <<'JSON'\n" + limitResult(0) + "\nJSON\nexit 1\n"
}

func limitResult(cost float64) string {
	line, _ := json.Marshal(map[string]any{
		"type": "result", "subtype": "success", "is_error": true, "num_turns": 1, "total_cost_usd": cost,
		"api_error_status": 429, "api_error": "usage_limit_reached", "terminal_reason": "api_error",
		"result": "You've hit your 5-hour limit · resets 6:30pm (UTC)", "permission_denials": []any{},
	})
	return string(line)
}

// costing is a step that declares waiting and reports what the run cost.
func costing(cost float64) string {
	line, _ := json.Marshal(map[string]any{
		"type": "result", "subtype": "success", "is_error": false, "num_turns": 2, "total_cost_usd": cost,
		"structured_output": map[string]any{"status": "waiting", "summary": "s", "question": "q"}, "permission_denials": []any{},
	})
	return "cat <<'JSON'\n" + string(line) + "\nJSON\n"
}

// limitWorld is h with a clock the agent reads, which the test moves.
type limitWorld struct {
	*h
	offset atomic.Int64
	notes  *notes
}

func newLimit(t *testing.T, yamlExtra string, edit func(*config.Config)) *limitWorld {
	t.Helper()
	w := &limitWorld{h: newH(t, yamlExtra, edit), notes: &notes{}}
	w.start()
	return w
}

// start (re)starts the agent with the world's clock.
func (w *limitWorld) start() {
	w.agentWith(func(a *Agent) { a.Now = w.now; a.Notifier = w.notes })
}

func (w *limitWorld) now() time.Time { return time.Now().Add(time.Duration(w.offset.Load())) }

func (w *limitWorld) later(d time.Duration) { w.offset.Add(int64(d)) }

func (w *limitWorld) only(ref forge.IssueRef) post {
	w.t.Helper()
	qs := w.queuedPosts(ref)
	if len(qs) != 1 {
		w.t.Fatalf("queued comments on %s = %d, want 1", ref, len(qs))
	}
	return qs[0]
}

func TestLimitPutsTheIssueBackInTheQueue(t *testing.T) {
	w := newLimit(t, "", nil)
	resets := w.now().Add(2 * time.Hour).Truncate(time.Second)
	w.step(1, limited(resets))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.settle()
	if got := w.labels(w.ref); got != "agent:ready" {
		t.Fatalf("labels = %q, want agent:ready", got)
	}
	q := w.only(w.ref)
	claims := w.claims(w.ref)
	run, _ := marker.Parse(claims[len(claims)-1].Body)
	if q.Marker.Reason != marker.QueuedLimit || q.Marker.Run != run.Run || !q.Marker.Until.Equal(resets) {
		t.Fatalf("queued marker = %+v, want reason limit, run %s and until %v", q.Marker, run.Run, resets)
	}
	if n := len(w.posts(w.ref)); n != 1 {
		t.Errorf("posts = %d, want only the queued comment", n)
	}
	if len(w.notes.sent()) != 0 {
		t.Errorf("notified %q; a limit is not notified", w.notes.sent())
	}
	w.poll()
	w.settle()
	if w.starts() != 1 || len(w.queuedPosts(w.ref)) != 1 {
		t.Fatalf("starts = %d queued = %d: the paused host must not start the issue again", w.starts(), len(w.queuedPosts(w.ref)))
	}
}

func TestLimitResumesAfterTheResetTime(t *testing.T) {
	w := newLimit(t, "", nil)
	w.step(1, limited(w.now().Add(2*time.Hour)))
	w.step(2, waiting("Went on.", "Which name?"))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.settle()
	w.later(2*time.Hour + time.Minute)
	w.poll()
	w.settle()
	if w.starts() != 2 || w.labels(w.ref) != "agent:waiting" || len(w.queuedPosts(w.ref)) != 0 {
		t.Fatalf("starts = %d labels = %q queued = %d", w.starts(), w.labels(w.ref), len(w.queuedPosts(w.ref)))
	}
	if !strings.Contains(w.file("prompt-2.md"), resumeLine) {
		t.Errorf("the resumed run's prompt lacks the limit sentence:\n%s", w.file("prompt-2.md"))
	}
	if strings.Contains(w.file("prompt-1.md"), resumeLine) {
		t.Error("the first run's prompt has the limit sentence")
	}
	if strings.Contains(w.file("prompt-2.md"), "You've hit") {
		t.Error("the resumed run read its queued comment")
	}
	claims := w.claims(w.ref)
	if m, _ := marker.Parse(claims[len(claims)-1].Body); m.Mode != "" {
		t.Errorf("the resumed run's claim names the mode %q; a resume starts no new cycle", m.Mode)
	}
}

func TestPausedHostQueuesOtherIssuesAndKeepsRunning(t *testing.T) {
	w := newLimit(t, "", func(c *config.Config) { c.Agent.MaxParallel = 2 })
	second := w.f.AddIssue("o", "r", 13, "Second", "Do it.", w.master)
	third := w.f.AddIssue("o", "r", 14, "Third", "Do it too.", w.master)
	w.step(1, block(1, waiting("s", "q")))
	w.step(2, limited(w.now().Add(time.Hour)))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.waitStarted(1)
	w.f.AddLabel(second, "agent:ready", w.master)
	w.poll()
	w.waitFor("the queued comment of the limited run", func() bool { return len(w.queuedPosts(second)) == 1 })
	w.f.AddLabel(third, "agent:ready", w.master)
	w.poll()
	if q := w.only(third); q.Marker.Reason != marker.QueuedLimit || q.Marker.Until.IsZero() || q.Marker.Run != "" {
		t.Fatalf("queued marker of the third issue = %+v, want reason limit with until and no run", q.Marker)
	}
	if w.labels(w.ref) != "agent:running" {
		t.Fatalf("the running issue is %q; a limit stops no running run", w.labels(w.ref))
	}
	w.release(1)
	w.settle()
	if w.starts() != 2 || w.labels(w.ref) != "agent:waiting" {
		t.Fatalf("starts = %d labels = %q", w.starts(), w.labels(w.ref))
	}
	w.step(0, waiting("s", "q"))
	w.later(time.Hour + time.Minute)
	w.poll()
	w.settle()
	if w.starts() != 4 || len(w.queuedPosts(second)) != 0 || len(w.queuedPosts(third)) != 0 {
		t.Fatalf("starts = %d queued = %d, %d; both should start after the reset", w.starts(), len(w.queuedPosts(second)), len(w.queuedPosts(third)))
	}
	if both := w.file("prompt-3.md") + w.file("prompt-4.md"); strings.Count(both, resumeLine) != 1 {
		t.Errorf("the limit sentence is in %d of the two prompts; only the limited issue gets it", strings.Count(both, resumeLine))
	}
}

func TestPauseSurvivesARestart(t *testing.T) {
	w := newLimit(t, "", nil)
	w.step(1, limited(w.now().Add(time.Hour)))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.settle()
	w.stop()
	w.a.Wait()
	w.a.Close()
	w.a = nil
	w.ctx, w.stop = context.WithCancel(context.Background())
	w.start()
	w.poll()
	w.settle()
	if w.starts() != 1 {
		t.Fatalf("starts = %d; the restarted agent must stay paused", w.starts())
	}
	w.only(w.ref) // the resume mark is kept over the restart, not posted again
	w.step(2, waiting("s", "q"))
	w.later(time.Hour + time.Minute)
	w.poll()
	w.settle()
	if w.starts() != 2 || len(w.queuedPosts(w.ref)) != 0 || !strings.Contains(w.file("prompt-2.md"), resumeLine) {
		t.Fatalf("starts = %d queued = %d; the issue should resume after the reset", w.starts(), len(w.queuedPosts(w.ref)))
	}
}

func TestLimitWithoutResetTimeWaitsLimitWait(t *testing.T) {
	w := newLimit(t, "", func(c *config.Config) { c.Agent.LimitWait = 10 * time.Minute })
	w.step(1, limited429())
	w.step(2, waiting("s", "q"))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	before := w.now()
	w.poll()
	w.settle()
	q := w.only(w.ref)
	if q.Marker.Until.Before(before.Add(10*time.Minute-time.Second)) || q.Marker.Until.After(w.now().Add(10*time.Minute+time.Second)) {
		t.Fatalf("until = %v, want about 10m after %v", q.Marker.Until, before)
	}
	w.later(5 * time.Minute)
	w.poll()
	w.settle()
	if w.starts() != 1 {
		t.Fatalf("starts = %d before limit_wait passed", w.starts())
	}
	w.later(6 * time.Minute)
	w.poll()
	w.settle()
	if w.starts() != 2 {
		t.Fatalf("starts = %d after limit_wait passed", w.starts())
	}
}

func TestResumedPlanRunGetsNoLimitSentence(t *testing.T) {
	w := newLimit(t, "", nil)
	w.step(1, limited(w.now().Add(time.Hour)))
	w.step(2, questions([3]string{"Default", "Which default?", "A."}))
	w.f.AddLabel(w.ref, "agent:plan", w.master)
	w.poll()
	w.settle()
	if w.labels(w.ref) != "agent:ready" {
		t.Fatalf("labels = %q, want agent:ready back", w.labels(w.ref))
	}
	w.later(time.Hour + time.Minute)
	w.poll()
	w.settle()
	p := w.lastPost(w.ref)
	if p.Marker.Reason != marker.PlanQuestions {
		t.Fatalf("last post = %+v, want plan questions", p.Marker)
	}
	if strings.Contains(w.file("prompt-2.md"), resumeLine) {
		t.Error("the resumed plan run got the limit sentence")
	}
}

func TestApprovedRunResumesAsApproved(t *testing.T) {
	w := newLimit(t, "", func(c *config.Config) { c.Projects[0].PlanFirst = config.PlanAlways })
	w.step(1, planDecl("1. Add greeting.txt."))
	w.step(2, limited(w.now().Add(time.Hour)))
	w.step(3, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.settle()
	w.f.AddComment(w.ref, "/ok", w.master)
	w.poll()
	w.settle()
	if w.labels(w.ref) != "agent:ready" || w.starts() != 2 {
		t.Fatalf("labels = %q starts = %d after the limit", w.labels(w.ref), w.starts())
	}
	w.later(time.Hour + time.Minute)
	w.poll()
	w.settle()
	d := w.lastPost(w.ref)
	if d.Marker.State != "done" || w.starts() != 3 {
		t.Fatalf("last post = %+v starts = %d, want the approved run done", d.Marker, w.starts())
	}
	if !strings.Contains(w.file("system-3.md"), `status "done"`) || !strings.Contains(w.file("prompt-3.md"), resumeLine) {
		t.Errorf("the resumed run is not the approved implementation:\n%s", w.file("system-3.md"))
	}
}

func TestUntrustedStartLabelOnAQueuedLimitIsNoResume(t *testing.T) {
	// The resume mark is only the agent's own queued comment of a run; a start label put by someone untrusted next
	// to an ordinary queued comment does not start anything.
	pl := thread(agentPost(marker.Marker{V: 1, State: marker.Queued, Reason: marker.QueuedLimit, Until: time.Now()}))
	if pl.limitResume() != 0 {
		t.Fatal("a queued comment without a run is a resume mark")
	}
	pl = thread(agentPost(marker.Marker{V: 1, State: marker.Queued, Reason: marker.QueuedLimit, Run: "r1", Until: time.Now()}))
	if pl.limitResume() == 0 {
		t.Fatal("the queued comment of a limited run is no resume mark")
	}
	pl = thread(agentPost(marker.Marker{V: 1, State: marker.Queued, Reason: marker.QueuedLimit, Run: "r1"}), resumed())
	if pl.limitResume() != 0 {
		t.Fatal("a resume mark followed by a claim still counts")
	}
}

func TestStartLabelOfATrustedAgentAccountStartsARun(t *testing.T) {
	// One account as both the agent and the trusted user: its start label with no resume mark is a person's, as
	// before ADR-053, since the forge cannot tell the two apart.
	x := newH(t, "", func(c *config.Config) { c.TrustedAuthors.GitHub = []string{"master", "agent"} })
	x.f.AddLabel(x.ref, "agent:ready", x.bot)
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("starts = %d, want 1", x.starts())
	}
	claims := x.claims(x.ref)
	if m, _ := marker.Parse(claims[len(claims)-1].Body); m.Mode == "" {
		t.Error("the run's claim names no mode; a person's start label starts a new cycle")
	}
}

func TestWithTheLoginOnAStartLabelOfTheAgentAccountStartsNothing(t *testing.T) {
	// With inbox.auth, ADR-056 does not hold: the agent's start label without a resume mark is never a person's, even
	// when its account is somehow a trusted one (issue #146).
	x := newH(t, "", func(c *config.Config) {
		c.TrustedAuthors.GitHub = []string{"master", "agent"}
		c.Inbox.Auth = &config.InboxAuth{ClientID: "cid", ClientSecretRef: "s", MemberGroups: []string{"team"}}
	})
	x.f.AddLabel(x.ref, "agent:ready", x.bot)
	x.poll()
	x.settle()
	if x.starts() != 0 {
		t.Fatalf("starts = %d, want 0 with the login on", x.starts())
	}
}

func TestStartLabelOfAnUntrustedAgentAccountStartsNothing(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.bot)
	x.poll()
	x.settle()
	if x.starts() != 0 {
		t.Fatalf("starts = %d; the agent's start label without a resume mark must start nothing", x.starts())
	}
}

func TestQuotaQueuesUntilTheNextDay(t *testing.T) {
	w := newLimit(t, "quota: {daily_usd: 1}\n", func(c *config.Config) { c.Claude.Auth = "api_key" })
	second := w.f.AddIssue("o", "r", 13, "Second", "Do it.", w.master)
	w.step(1, costing(1.2))
	w.step(2, waiting("s", "q"))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.settle()
	w.f.AddLabel(second, "agent:ready", w.master)
	w.poll()
	w.settle()
	q := w.only(second)
	now := w.now()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local)
	if q.Marker.Reason != marker.QueuedQuota || !q.Marker.Until.Equal(midnight) || w.starts() != 1 {
		t.Fatalf("queued marker = %+v starts = %d, want reason quota until %v", q.Marker, w.starts(), midnight)
	}
	w.later(midnight.Sub(now) + time.Minute)
	w.poll()
	w.settle()
	if w.starts() != 2 || len(w.queuedPosts(second)) != 0 {
		t.Fatalf("starts = %d queued = %d after the day changed", w.starts(), len(w.queuedPosts(second)))
	}
}

func TestQuotaIsNotCountedWithASubscription(t *testing.T) {
	w := newLimit(t, "quota: {daily_usd: 1}\n", nil)
	second := w.f.AddIssue("o", "r", 13, "Second", "Do it.", w.master)
	w.step(1, costing(1.2))
	w.step(2, waiting("s", "q"))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.settle()
	w.f.AddLabel(second, "agent:ready", w.master)
	w.poll()
	w.settle()
	if w.starts() != 2 || len(w.queuedPosts(second)) != 0 {
		t.Fatalf("starts = %d queued = %d; a subscription has no quota", w.starts(), len(w.queuedPosts(second)))
	}
}

func TestFixRunOnALimitWaitsAndIsNotCounted(t *testing.T) {
	w := newCI(t, "", nil)
	sha := w.syncHead()
	resets := w.now().Add(time.Hour).Truncate(time.Second)
	w.step(2, limited(resets))
	w.f.SetChecks("o", "r", sha, failedCheck("test", 7))
	w.f.SetLog(7, "FAIL: TestX")
	w.poll()
	w.settle()
	p := w.lastPost(w.ref)
	if p.Marker.State != "waiting" || p.Marker.Reason != "error" || !p.Marker.Until.Equal(resets) {
		t.Fatalf("last post = %+v, want waiting(error) with until %v\n%s", p.Marker, resets, p.Body)
	}
	if !strings.Contains(p.Body, "usage limit") {
		t.Errorf("the wait does not say the run hit a usage limit:\n%s", p.Body)
	}
	// The fix run that hit the limit is not counted toward agent.max_fix_attempts.
	pl := thread(
		agentPost(marker.Marker{V: 1, State: marker.Running, Mode: marker.ModeDirect, Run: "r0"}),
		doneAt(),
		agentPost(marker.Marker{V: 1, State: marker.Running, Reason: marker.Fix, Run: "r1"}),
		agentPost(marker.Marker{V: 1, State: marker.Waiting, Reason: "error", Run: "r1", Until: resets}),
	)
	if n := pl.fixes(); n != 0 {
		t.Errorf("fixes = %d, want 0: the limited fix run is not counted", n)
	}
	pl.comments[3].Body = "text\n\n" + marker.Marker{V: 1, State: marker.Waiting, Reason: "error", Run: "r1"}.Format()
	if n := pl.fixes(); n != 1 {
		t.Errorf("fixes = %d, want 1 for a fix run that failed otherwise", n)
	}
}

func TestQueuedLimitCommentNamesTheTime(t *testing.T) {
	w := newLimit(t, "", func(c *config.Config) { c.Comments.Language = "ja" })
	resets := w.now().Add(2 * time.Hour)
	w.step(1, limited(resets))
	w.f.AddLabel(w.ref, "agent:ready", w.master)
	w.poll()
	w.settle()
	q := w.only(w.ref)
	want := resets.Local().Format("2006-01-02 15:04")
	if !strings.Contains(q.Body, "利用上限") || !strings.Contains(q.Body, want) {
		t.Fatalf("queued comment = %q, want the limit and %s", q.Body, want)
	}
	if strings.Contains(q.Body, "o/r") {
		t.Errorf("the comment names a repository: %q", q.Body)
	}
}
