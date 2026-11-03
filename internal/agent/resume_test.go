package agent

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/prompt"
	"github.com/uskanda/switchboard/internal/state"
)

func TestResumeFromTheLastPlanPostOfThisHost(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	within := time.Hour
	a := &Agent{Config: &config.Config{Host: config.Host{Name: "box"}}, State: state.Dir{Path: t.TempDir()},
		Now: func() time.Time { return now }}
	a.Config.Agent.ResumeWithinSet = &within
	cur := launch{Head: "abc", Tools: planTools, Model: "opus", Effort: "high", System: "sha"}
	prev := cur
	prev.Session = "s1"
	if err := a.writeLaunch(a.runDir("r1"), prev); err != nil {
		t.Fatal(err)
	}
	post := func(mod func(*marker.Marker)) entry {
		m := marker.Marker{V: 1, Host: "box", Run: "r1", Session: "s1", State: marker.Waiting, Reason: marker.PlanQuestions, Q: 2, At: now.Add(-20 * time.Minute)}
		if mod != nil {
			mod(&m)
		}
		return agentPost(m)
	}
	changed := func(mod func(*launch)) launch {
		l := cur
		mod(&l)
		return l
	}
	for name, c := range map[string]struct {
		pl   *plan
		cur  launch
		want string
	}{
		"questions":          {thread(claimOf("plan"), post(nil), reply("A")), cur, "s1"},
		"plan":               {thread(claimOf("plan"), post(func(m *marker.Marker) { m.Reason = marker.PlanReady }), reply("change it")), cur, "s1"},
		"another host":       {thread(claimOf("plan"), post(func(m *marker.Marker) { m.Host = "other" }), reply("A")), cur, ""},
		"too long ago":       {thread(claimOf("plan"), post(func(m *marker.Marker) { m.At = now.Add(-2 * time.Hour) }), reply("A")), cur, ""},
		"an error":           {thread(claimOf("plan"), post(nil), reply("A"), post(func(m *marker.Marker) { m.Reason = "error"; m.Run = "r2" }), reply("again")), cur, ""},
		"other session":      {thread(claimOf("plan"), post(func(m *marker.Marker) { m.Session = "s9" }), reply("A")), cur, ""},
		"no launch file":     {thread(claimOf("plan"), post(func(m *marker.Marker) { m.Run = "r9" }), reply("A")), cur, ""},
		"other head":         {thread(claimOf("plan"), post(nil), reply("A")), changed(func(l *launch) { l.Head = "def" }), ""},
		"other instructions": {thread(claimOf("plan"), post(nil), reply("A")), changed(func(l *launch) { l.System = "sha2" }), ""},
		"other tools":        {thread(claimOf("plan"), post(nil), reply("A")), changed(func(l *launch) { l.Tools = runTools }), ""},
		"other model":        {thread(claimOf("plan"), post(nil), reply("A")), changed(func(l *launch) { l.Model = "sonnet" }), ""},
		"other effort":       {thread(claimOf("plan"), post(nil), reply("A")), changed(func(l *launch) { l.Effort = "low" }), ""},
	} {
		c.pl.kind = planRun
		if got := a.resumeFrom(c.pl, c.cur); got != c.want {
			t.Errorf("%s: resumeFrom = %q, want %q", name, got, c.want)
		}
	}
	pl := thread(claimOf("plan"), post(nil), reply("/ok"))
	for _, k := range []runKind{approvedRun, fixRun, directRun} {
		pl.kind = k
		if got := a.resumeFrom(pl, cur); got != "" {
			t.Errorf("%v: resumeFrom = %q, want none", k, got)
		}
	}
	off := time.Duration(0)
	a.Config.Agent.ResumeWithinSet = &off
	pl.kind = planRun
	if got := a.resumeFrom(pl, cur); got != "" {
		t.Errorf("resume_within 0: resumeFrom = %q, want none", got)
	}
}

func TestTheSecondPlanRoundForksFromTheFirst(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, questions([3]string{"Name?", "A or B", "A"}))
	x.step(2, planDecl("The plan."))
	x.startPlan()
	p := x.replyAndRun("A")
	if p.Marker.Reason != marker.PlanReady {
		t.Fatalf("post = %+v", p)
	}
	first := strings.TrimSpace(x.file("session-1"))
	argv := strings.Split(strings.TrimSpace(x.file("argv-2")), "\n")
	if !hasSeq(argv, []string{"--resume", first}) || !hasSeq(argv, []string{"--fork-session"}) {
		t.Fatalf("argv of the second run = %v, want --resume %s --fork-session", argv, first)
	}
	if second := strings.TrimSpace(x.file("session-2")); second == first || second != p.Marker.Session {
		t.Errorf("second session = %q, first %q, marker %q", second, first, p.Marker.Session)
	}
	if !strings.HasPrefix(x.file("prompt-2.md"), prompt.ContinuedNote) {
		t.Errorf("the second prompt does not start with the continuation line:\n%s", x.file("prompt-2.md"))
	}
	if strings.Contains(x.file("argv-1"), "--resume") {
		t.Error("the first run resumed")
	}
}

func TestTheApprovedRunStartsAFreshSession(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("The plan."))
	x.step(2, commit("a.txt", "a")+done("Done."))
	x.startPlan()
	x.replyAndRun("/ok")
	if strings.Contains(x.file("argv-2"), "--resume") {
		t.Fatalf("the approved run resumed: %s", x.file("argv-2"))
	}
}

// noSession is a step that ends like claude asked to resume a session it does not have (V53).
const noSession = `if printf '%s\n' "$@" | grep -qx -- --resume; then
cat <<'JSON'
{"type":"result","subtype":"error_during_execution","is_error":true,"num_turns":0,"session_id":"s","errors":["No conversation found with session ID: x"],"permission_denials":[]}
JSON
exit 1
fi
`

func TestAMissingSessionStartsTheRunAgainFresh(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, questions([3]string{"Name?", "A or B", "A"}))
	x.step(2, noSession)
	x.step(3, planDecl("The plan."))
	x.startPlan()
	p := x.replyAndRun("A")
	if n := x.starts(); n != 3 {
		t.Fatalf("claude started %d times, want 3", n)
	}
	if strings.Contains(x.file("argv-3"), "--resume") {
		t.Fatal("the run started again with --resume")
	}
	if p.Marker.Reason != marker.PlanReady || p.Marker.Session != strings.TrimSpace(x.file("session-3")) {
		t.Fatalf("post = %+v, session-3 = %s", p.Marker, x.file("session-3"))
	}
	if n := len(x.posts(x.ref)); n != 2 {
		t.Errorf("posts = %d, want the questions and the plan", n)
	}
	var started []map[string]any
	x.audit.mu.Lock()
	for _, e := range x.audit.events {
		if e["event"] == "run.started" && e["run"] == p.Marker.Run {
			started = append(started, e)
		}
	}
	x.audit.mu.Unlock()
	if len(started) != 2 || started[0]["resumed_from"] != strings.TrimSpace(x.file("session-1")) || started[1]["resumed_from"] != "" {
		t.Errorf("run.started = %v", started)
	}
}

func TestLaunchIsWrittenForEveryRun(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, questions([3]string{"Name?", "A or B", "A"}))
	p := x.startPlan()
	if _, err := os.Stat(filepath.Join(x.dir.Path, "runs", p.Marker.Run, "launch.json")); err != nil {
		t.Fatal(err)
	}
}

func hasSeq(xs, seq []string) bool {
	for i := 0; i+len(seq) <= len(xs); i++ {
		if slices.Equal(xs[i:i+len(seq)], seq) {
			return true
		}
	}
	return false
}
