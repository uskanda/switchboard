// Package claudecode starts the claude CLI in print mode and reads its result. Everything that depends on the CLI's
// flags, its stream-json output and its settings format lives here, so that a CLI change touches one package.
// The flags and fields used were checked against claude 2.1.312 (docs/ROADMAP.md, verification items).
package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/uskanda/switchboard/internal/usage"
)

// Settings is the per-run Claude Code settings file: what the run may and may not do without asking.
type Settings struct {
	Allow []string
	Deny  []string
}

// Request describes one run.
type Request struct {
	Binary       string // the claude executable
	Dir          string // working directory (the issue's worktree)
	RunDir       string // where the run's files are written: prompt, settings, output
	SessionID    string // a new UUID for every run
	Prompt       string // sent on stdin
	SystemPrompt string // appended to the system prompt
	Settings     Settings
	MaxTurns     int
	Schema       string // JSON schema of the final structured output
	// Tools are the built-in tools the session gets (--tools); nil leaves the CLI's default set.
	Tools []string
	// Model and Effort pick the model and the effort level; empty leaves them to the CLI and the user's settings.
	Model, Effort string
	Auth          string // "subscription" or "api_key"
	Timeout       time.Duration
	// IntGrace and TermGrace are how long to wait after SIGINT and after SIGTERM before the next signal.
	// Zero means 30s and 10s.
	IntGrace, TermGrace time.Duration
	// Env is the parent environment to pick from (os.Environ() when nil).
	Env []string
}

// Denial is a tool call the permission system refused.
type Denial struct {
	ToolName  string          `json:"tool_name"`
	ToolUseID string          `json:"tool_use_id"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// Command returns the denied Bash command, or "" for other tools.
func (d Denial) Command() string {
	var in struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(d.ToolInput, &in)
	return in.Command
}

// Result is what a run produced.
type Result struct {
	SessionID  string
	Subtype    string // "success", "error_max_turns", "error_during_execution", ...
	IsError    bool
	NumTurns   int
	Text       string          // the final text result
	Structured json.RawMessage // the structured output, nil when the run produced none
	Denials    []Denial
	Errors     []string
	ExitCode   int
	TimedOut   bool // stopped because Timeout passed
	Canceled   bool // stopped because the context was canceled
	HasResult  bool // the output contained a result message
	// Limit is set when the run ended in an error because the service refused it on a limit (V12).
	Limit *usage.LimitHit
	// CostUSD is what the run cost, as claude reported it (total_cost_usd).
	CostUSD float64
}

// allowedEnv are the variables passed to claude. Forge tokens, the SSH agent, the parent Claude Code session and
// everything else stay behind (ADR-012, ADR-027).
var allowedEnv = map[string]bool{
	"PATH": true, "HOME": true, "LANG": true, "TMPDIR": true, "SHELL": true, "USER": true,
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true, "http_proxy": true, "https_proxy": true, "no_proxy": true,
	"SSL_CERT_FILE": true, "SSL_CERT_DIR": true, "NODE_EXTRA_CA_CERTS": true,
}

// Env picks the environment for claude from parent. ANTHROPIC_API_KEY passes only when auth is "api_key".
func Env(parent []string, auth string) []string {
	var out []string
	for _, kv := range parent {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		if allowedEnv[k] || (auth == "api_key" && k == "ANTHROPIC_API_KEY") {
			out = append(out, kv)
		}
	}
	return out
}

// Efforts are the effort levels the CLI accepts for --effort.
var Efforts = []string{"low", "medium", "high", "xhigh", "max"}

// Args returns the command-line arguments for req. The files they name are written by Run.
func Args(req Request) []string {
	args := []string{
		"-p",
		"--session-id", req.SessionID,
		"--output-format", "stream-json", "--verbose",
		"--permission-mode", "dontAsk",
		"--settings", filepath.Join(req.RunDir, "settings.json"),
		"--strict-mcp-config", "--mcp-config", filepath.Join(req.RunDir, "mcp.json"),
		"--append-system-prompt-file", filepath.Join(req.RunDir, "system.md"),
		"--max-turns", strconv.Itoa(req.MaxTurns),
		"--json-schema", req.Schema,
	}
	if len(req.Tools) > 0 {
		args = append(args, "--tools", strings.Join(req.Tools, ","))
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.Effort != "" {
		args = append(args, "--effort", req.Effort)
	}
	return args
}

func writeFiles(req Request) error {
	if err := os.MkdirAll(req.RunDir, 0o700); err != nil {
		return err
	}
	allow, deny := req.Settings.Allow, req.Settings.Deny
	if allow == nil {
		allow = []string{}
	}
	if deny == nil {
		deny = []string{}
	}
	settings, err := json.MarshalIndent(map[string]any{"permissions": map[string]any{"allow": allow, "deny": deny}}, "", "  ")
	if err != nil {
		return err
	}
	files := map[string][]byte{
		"settings.json": append(settings, '\n'),
		"mcp.json":      []byte("{\"mcpServers\":{}}\n"),
		"system.md":     []byte(req.SystemPrompt),
		"prompt.md":     []byte(req.Prompt),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(req.RunDir, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// Run starts claude for req and waits for it. On timeout or cancellation it sends SIGINT to the process group
// (which ends the turn cleanly), then SIGTERM, then SIGKILL. The output is kept in RunDir/stream.jsonl.
// An error is returned only when claude could not be started or its output could not be read.
func Run(ctx context.Context, req Request) (*Result, error) {
	if err := writeFiles(req); err != nil {
		return nil, err
	}
	parent := req.Env
	if parent == nil {
		parent = os.Environ()
	}
	stdin, err := os.Open(filepath.Join(req.RunDir, "prompt.md"))
	if err != nil {
		return nil, err
	}
	defer stdin.Close()
	out, err := os.OpenFile(filepath.Join(req.RunDir, "stream.jsonl"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	errLog, err := os.OpenFile(filepath.Join(req.RunDir, "stderr.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	defer errLog.Close()

	cmd := exec.Command(req.Binary, Args(req)...)
	cmd.Dir = req.Dir
	cmd.Env = Env(parent, req.Auth)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, out, errLog
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", req.Binary, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	res := &Result{}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 45 * time.Minute
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var waitErr error
	select {
	case waitErr = <-done:
	case <-timer.C:
		res.TimedOut = true
		waitErr = stop(cmd.Process.Pid, done, grace(req.IntGrace, 30*time.Second), grace(req.TermGrace, 10*time.Second))
	case <-ctx.Done():
		res.Canceled = true
		waitErr = stop(cmd.Process.Pid, done, grace(req.IntGrace, 30*time.Second), grace(req.TermGrace, 10*time.Second))
	}
	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
	case errors.As(waitErr, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		return nil, waitErr
	}
	f, err := os.Open(filepath.Join(req.RunDir, "stream.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := Parse(f, res); err != nil {
		return nil, err
	}
	return res, nil
}

func grace(d, def time.Duration) time.Duration {
	if d <= 0 {
		return def
	}
	return d
}

// stop signals the process group of pid until the process exits, and returns its wait error.
func stop(pid int, done <-chan error, intGrace, termGrace time.Duration) error {
	_ = syscall.Kill(-pid, syscall.SIGINT)
	select {
	case err := <-done:
		return err
	case <-time.After(intGrace):
	}
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	select {
	case err := <-done:
		return err
	case <-time.After(termGrace):
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	return <-done
}

// Parse reads stream-json output and fills res from the last result message.
//
// A usage limit shows as a rate_limit_event with the status "rejected" and the reset time in resetsAt (Unix
// seconds), followed by a result with is_error, api_error_status 429 and, for the usage limit, api_error
// "usage_limit_reached"; the subtype stays "success" (V12). Only a result in error counts as a limit: a run that
// went on and succeeded after a rejected event did not stop on it.
func Parse(r io.Reader, res *Result) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	var rejected *time.Time // the reset time of the last rejected rate_limit_event
	sawRejected := false
	for sc.Scan() {
		line := sc.Bytes()
		var head struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(line, &head) != nil {
			continue
		}
		if head.Type == "rate_limit_event" {
			var ev struct {
				Info struct {
					Status   string `json:"status"`
					ResetsAt int64  `json:"resetsAt"`
				} `json:"rate_limit_info"`
			}
			if json.Unmarshal(line, &ev) == nil && ev.Info.Status == "rejected" {
				sawRejected, rejected = true, nil
				if ev.Info.ResetsAt > 0 {
					t := time.Unix(ev.Info.ResetsAt, 0)
					rejected = &t
				}
			}
			continue
		}
		if head.Type != "result" {
			continue
		}
		var m struct {
			Subtype           string          `json:"subtype"`
			IsError           bool            `json:"is_error"`
			SessionID         string          `json:"session_id"`
			NumTurns          int             `json:"num_turns"`
			Result            string          `json:"result"`
			StructuredOutput  json.RawMessage `json:"structured_output"`
			PermissionDenials []Denial        `json:"permission_denials"`
			Errors            []string        `json:"errors"`
			TotalCostUSD      float64         `json:"total_cost_usd"`
			APIErrorStatus    int             `json:"api_error_status"`
			APIError          string          `json:"api_error"`
		}
		if err := json.Unmarshal(line, &m); err != nil {
			return fmt.Errorf("decode result message: %w", err)
		}
		res.HasResult = true
		res.Subtype, res.IsError, res.SessionID, res.NumTurns = m.Subtype, m.IsError, m.SessionID, m.NumTurns
		res.Text, res.Denials, res.Errors = m.Result, m.PermissionDenials, m.Errors
		res.CostUSD = m.TotalCostUSD
		res.Structured = nil
		if len(m.StructuredOutput) > 0 && string(m.StructuredOutput) != "null" {
			res.Structured = m.StructuredOutput
		}
		res.Limit = nil
		if m.IsError && (sawRejected || m.APIErrorStatus == 429) {
			kind := usage.KindRate
			if sawRejected || m.APIError == "usage_limit_reached" {
				kind = usage.KindUsage
			}
			res.Limit = &usage.LimitHit{Kind: kind, ResetsAt: rejected, Message: m.Result}
		}
	}
	return sc.Err()
}

// CheckedVersion is the claude version the flags and output formats above were checked against.
const CheckedVersion = "2.1.312"

var versionRE = regexp.MustCompile(`\d+\.\d+\.\d+`)

// Version runs `binary --version` and returns the version number it prints.
func Version(ctx context.Context, binary string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binary, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("%s --version: %w", binary, err)
	}
	v := versionRE.FindString(string(out))
	if v == "" {
		return "", fmt.Errorf("%s --version printed no version: %q", binary, strings.TrimSpace(string(out)))
	}
	return v, nil
}

// Older reports whether version v is older than base. A version that does not parse counts as older.
func Older(v, base string) bool {
	parse := func(s string) ([3]int, bool) {
		var n [3]int
		parts := strings.Split(s, ".")
		if len(parts) != 3 {
			return n, false
		}
		for i, p := range parts {
			x, err := strconv.Atoi(p)
			if err != nil {
				return n, false
			}
			n[i] = x
		}
		return n, true
	}
	a, ok := parse(v)
	b, _ := parse(base)
	if !ok {
		return true
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
