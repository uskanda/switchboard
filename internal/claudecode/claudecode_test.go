package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// fakeClaude writes an executable that records how it was started (argv, env, cwd, stdin) into rec and then
// replays fixture, replacing __SESSION__ with the --session-id it was given. With sleep > 0 it sleeps first; with
// ignoreInt it ignores SIGINT, and with ignoreTerm SIGTERM too. Every signal it receives is appended to rec/signals.
func fakeClaude(t *testing.T, fixture string, sleep float64, ignoreInt, ignoreTerm bool, exit int) (bin, rec string) {
	t.Helper()
	dir := t.TempDir()
	rec = filepath.Join(dir, "rec")
	if err := os.Mkdir(rec, 0o755); err != nil {
		t.Fatal(err)
	}
	fixturePath, err := filepath.Abs(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	intAction := `echo INT >> "$rec/signals"; exit 130`
	if ignoreInt {
		intAction = `echo INT >> "$rec/signals"`
	}
	termAction := `echo TERM >> "$rec/signals"; exit 143`
	if ignoreTerm {
		termAction = `echo TERM >> "$rec/signals"`
	}
	script := fmt.Sprintf(`#!/usr/bin/env bash
rec=%q
trap '%s' INT
trap '%s' TERM
printf '%%s\n' "$@" > "$rec/argv"
env | sort > "$rec/env"
pwd > "$rec/cwd"
cat > "$rec/stdin"
sid=""
prev=""
for a in "$@"; do [ "$prev" = "--session-id" ] && sid="$a"; prev="$a"; done
end=$(( $(date +%%s) + %d ))
while [ %d -gt 0 ] && [ "$(date +%%s)" -lt "$end" ]; do sleep 0.05; done
sed -e "s/__SESSION__/$sid/g" -e "s#__CWD__#$(pwd)#g" %q
exit %d
`, rec, intAction, termAction, int(sleep+0.999), int(sleep*1000), fixturePath, exit)
	bin = filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, rec
}

func request(t *testing.T, bin string) Request {
	t.Helper()
	return Request{
		Binary:       bin,
		Dir:          t.TempDir(),
		RunDir:       t.TempDir(),
		SessionID:    "11111111-2222-3333-4444-555555555555",
		Prompt:       "do the thing",
		SystemPrompt: "rules",
		Settings:     Settings{Allow: []string{"Read", "Bash(git status:*)"}, Deny: []string{"Bash(git push:*)"}},
		MaxTurns:     5,
		Schema:       `{"type":"object"}`,
		Auth:         "subscription",
		Timeout:      30 * time.Second,
		Env:          []string{"PATH=" + os.Getenv("PATH"), "HOME=/home/alice", "GH_TOKEN=secret", "SSH_AUTH_SOCK=/tmp/agent", "CLAUDE_CODE_SESSION_ID=parent", "ANTHROPIC_API_KEY=sk-x", "HTTPS_PROXY=http://proxy:1"},
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunStartsClaudeAndParsesTheResult(t *testing.T) {
	bin, rec := fakeClaude(t, "done.jsonl", 0, false, false, 0)
	req := request(t, bin)
	res, err := Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID != req.SessionID || res.Subtype != "success" || res.IsError || res.ExitCode != 0 {
		t.Fatalf("result = %+v", res)
	}
	var out struct{ Status string }
	if err := json.Unmarshal(res.Structured, &out); err != nil || out.Status != "done" {
		t.Fatalf("structured = %s, %v", res.Structured, err)
	}
	argv := strings.Split(strings.TrimSpace(read(t, filepath.Join(rec, "argv"))), "\n")
	for _, want := range [][]string{
		{"-p"}, {"--session-id", req.SessionID}, {"--output-format", "stream-json"}, {"--verbose"},
		{"--permission-mode", "dontAsk"}, {"--settings", filepath.Join(req.RunDir, "settings.json")},
		{"--strict-mcp-config"}, {"--mcp-config", filepath.Join(req.RunDir, "mcp.json")},
		{"--append-system-prompt-file", filepath.Join(req.RunDir, "system.md")},
		{"--max-turns", "5"}, {"--json-schema", req.Schema},
	} {
		if !hasSeq(argv, want) {
			t.Errorf("argv lacks %v: %v", want, argv)
		}
	}
	if hasSeq(argv, []string{"--resume"}) || hasSeq(argv, []string{"--setting-sources"}) {
		t.Errorf("argv must not resume or restrict setting sources: %v", argv)
	}
	if got := strings.TrimSpace(read(t, filepath.Join(rec, "cwd"))); got != req.Dir {
		t.Errorf("cwd = %q, want %q", got, req.Dir)
	}
	if got := read(t, filepath.Join(rec, "stdin")); got != "do the thing" {
		t.Errorf("stdin = %q", got)
	}
	var settings map[string]map[string][]string
	if err := json.Unmarshal([]byte(read(t, filepath.Join(req.RunDir, "settings.json"))), &settings); err != nil {
		t.Fatal(err)
	}
	if settings["permissions"]["deny"][0] != "Bash(git push:*)" {
		t.Errorf("settings = %v", settings)
	}
	if got := strings.TrimSpace(read(t, filepath.Join(req.RunDir, "mcp.json"))); got != `{"mcpServers":{}}` {
		t.Errorf("mcp.json = %s", got)
	}
	if !strings.Contains(read(t, filepath.Join(req.RunDir, "stream.jsonl")), `"type":"result"`) {
		t.Error("stream.jsonl does not keep the output")
	}
}

func hasSeq(xs, seq []string) bool {
	for i := 0; i+len(seq) <= len(xs); i++ {
		match := true
		for j := range seq {
			if xs[i+j] != seq[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestRunPassesOnlyTheAllowlistedEnvironment(t *testing.T) {
	bin, rec := fakeClaude(t, "done.jsonl", 0, false, false, 0)
	if _, err := Run(context.Background(), request(t, bin)); err != nil {
		t.Fatal(err)
	}
	env := read(t, filepath.Join(rec, "env"))
	for _, banned := range []string{"GH_TOKEN=", "SSH_AUTH_SOCK=", "CLAUDE_CODE_SESSION_ID=", "ANTHROPIC_API_KEY="} {
		if strings.Contains(env, banned) {
			t.Errorf("claude received %s", banned)
		}
	}
	for _, kept := range []string{"HOME=/home/alice", "HTTPS_PROXY=http://proxy:1"} {
		if !strings.Contains(env, kept) {
			t.Errorf("claude did not receive %s", kept)
		}
	}
}

func TestEnvPassesTheAPIKeyOnlyForAPIKeyAuth(t *testing.T) {
	parent := []string{"PATH=/bin", "ANTHROPIC_API_KEY=sk-x", "GITHUB_TOKEN=t", "LANG=C.UTF-8", "TMPDIR=/tmp", "SHELL=/bin/bash", "USER=alice"}
	sub := Env(parent, "subscription")
	sort.Strings(sub)
	if strings.Join(sub, ",") != "LANG=C.UTF-8,PATH=/bin,SHELL=/bin/bash,TMPDIR=/tmp,USER=alice" {
		t.Errorf("subscription env = %v", sub)
	}
	api := strings.Join(Env(parent, "api_key"), ",")
	if !strings.Contains(api, "ANTHROPIC_API_KEY=sk-x") || strings.Contains(api, "GITHUB_TOKEN") {
		t.Errorf("api_key env = %v", api)
	}
}

func TestRunReportsPermissionDenials(t *testing.T) {
	bin, _ := fakeClaude(t, "waiting-denied.jsonl", 0, false, false, 0)
	res, err := Run(context.Background(), request(t, bin))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Denials) != 1 || res.Denials[0].ToolName != "Bash" || res.Denials[0].Command() != "touch created.txt" {
		t.Fatalf("denials = %+v", res.Denials)
	}
}

func TestRunReportsMaxTurns(t *testing.T) {
	bin, _ := fakeClaude(t, "max-turns.jsonl", 0, false, false, 1)
	res, err := Run(context.Background(), request(t, bin))
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || res.Subtype != "error_max_turns" || res.Structured != nil || res.ExitCode != 1 || len(res.Errors) != 1 {
		t.Fatalf("result = %+v", res)
	}
}

func TestTimeoutSendsINTThenTERMThenKILL(t *testing.T) {
	bin, rec := fakeClaude(t, "done.jsonl", 30, true, true, 0)
	req := request(t, bin)
	req.Timeout = 300 * time.Millisecond
	// The fake runs its trap only after its current sleep returns, which a loaded machine can delay past a short
	// grace: give each signal a second to be recorded.
	req.IntGrace = time.Second
	req.TermGrace = time.Second
	start := time.Now()
	res, err := Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.TimedOut || time.Since(start) > 10*time.Second {
		t.Fatalf("result = %+v after %v", res, time.Since(start))
	}
	if got := strings.Fields(read(t, filepath.Join(rec, "signals"))); strings.Join(got, ",") != "INT,TERM" {
		t.Fatalf("signals = %v, want INT then TERM before the KILL", got)
	}
}

func TestCancelStopsWithSIGINT(t *testing.T) {
	bin, rec := fakeClaude(t, "done.jsonl", 30, false, false, 0)
	req := request(t, bin)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	res, err := Run(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Canceled || res.TimedOut {
		t.Fatalf("result = %+v", res)
	}
	if got := strings.TrimSpace(read(t, filepath.Join(rec, "signals"))); got != "INT" {
		t.Fatalf("signals = %q", got)
	}
}

func TestVersion(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho '2.1.312 (Claude Code)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := Version(context.Background(), bin)
	if err != nil || v != "2.1.312" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	if _, err := Version(context.Background(), filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing binary has a version")
	}
}

func TestOlder(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"2.1.312", false}, {"2.1.400", false}, {"2.2.0", false}, {"3.0.0", false},
		{"2.1.311", true}, {"2.0.999", true}, {"1.9.0", true}, {"garbage", true},
	}
	for _, c := range cases {
		if got := Older(c.v, CheckedVersion); got != c.want {
			t.Errorf("Older(%q) = %v", c.v, got)
		}
	}
}
