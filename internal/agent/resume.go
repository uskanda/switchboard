package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/marker"
)

// launch is how a run started claude, kept in the run's directory so that the next plan run can tell whether it may
// fork from this run's session. A forked session keeps the system prompt it began with (V53), and a change of the
// tools drops the prompt cache, so these must stay the same; the worktree too, so that what the session read holds.
type launch struct {
	Session string   `json:"session"`
	Head    string   `json:"head,omitempty"` // the worktree's HEAD when the run started; plan runs only
	Tools   []string `json:"tools"`
	Model   string   `json:"model,omitempty"`
	Effort  string   `json:"effort,omitempty"`
	System  string   `json:"system"` // SHA-256 of the appended system prompt
}

func launchOf(req claudecode.Request, head string) launch {
	sum := sha256.Sum256([]byte(req.SystemPrompt))
	return launch{Session: req.SessionID, Head: head, Tools: req.Tools, Model: req.Model, Effort: req.Effort, System: hex.EncodeToString(sum[:])}
}

// same reports whether a session started as l may be forked by a run starting as cur.
func (l launch) same(cur launch) bool {
	return l.Head == cur.Head && slices.Equal(l.Tools, cur.Tools) && l.Model == cur.Model && l.Effort == cur.Effort && l.System == cur.System
}

func (a *Agent) writeLaunch(runDir string, l launch) error {
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDir, "launch.json"), append(b, '\n'), 0o600)
}

func (a *Agent) readLaunch(run string) (launch, bool) {
	var l launch
	b, err := os.ReadFile(filepath.Join(a.runDir(run), "launch.json"))
	if err != nil || json.Unmarshal(b, &l) != nil {
		return launch{}, false
	}
	return l, true
}

// resumeFrom returns the session a plan run starting as cur forks from, or "" for a fresh session. It forks from the
// previous plan round when the issue's latest wait is that round's questions or plan, posted by this host within
// agent.resume_within, and the round started claude as cur does (ADR-005).
func (a *Agent) resumeFrom(pl *plan, cur launch) string {
	within := a.Config.Agent.ResumeWithin()
	if pl.kind != planRun || within <= 0 {
		return ""
	}
	var last *marker.Marker
	for i := len(pl.comments) - 1; i >= 0 && last == nil; i-- {
		if m := pl.agentMarker(i); m != nil && m.State == marker.Waiting {
			last = m
		}
	}
	switch {
	case last == nil, last.Host != a.Config.Host.Name, last.Session == "", last.Run == "",
		last.Reason != marker.PlanQuestions && last.Reason != marker.PlanReady,
		a.now().Sub(last.At) > within:
		return ""
	}
	prev, ok := a.readLaunch(last.Run)
	if !ok || prev.Session != last.Session || !prev.same(cur) {
		return ""
	}
	return last.Session
}
