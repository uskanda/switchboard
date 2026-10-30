package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

// h is one test's world: a fake forge, a real git origin and clone, a fake claude, and an agent.
type h struct {
	t                    *testing.T
	f                    *fake.Forge
	cfg                  *config.Config
	dir                  state.Dir
	audit                *recorder
	a                    *Agent
	master, bot, mallory forge.User
	origin, clone, ctl   string
	ctx                  context.Context
	stop                 context.CancelFunc
	ref                  forge.IssueRef
	envExtra             []string
}

// recorder is an in-memory audit log.
type recorder struct {
	mu     sync.Mutex
	events []map[string]any
}

func (r *recorder) Log(event string, fields map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := map[string]any{"event": event}
	for k, v := range fields {
		rec[k] = v
	}
	r.events = append(r.events, rec)
}

func (r *recorder) has(event string, match func(map[string]any) bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e["event"] == event && (match == nil || match(e)) {
			return true
		}
	}
	return false
}

const fakeClaude = `#!/bin/bash
ctl='__CTL__'
n=$(( $(cat "$ctl/count" 2>/dev/null || echo 0) + 1 ))
echo $n > "$ctl/count"
session=""
prev=""
for a in "$@"; do
  [ "$prev" = "--session-id" ] && session=$a
  prev=$a
done
echo "$session" > "$ctl/session-$n"
cat > "$ctl/prompt-$n.md"
env > "$ctl/env-$n"
step="$ctl/step-$n.sh"
[ -f "$step" ] || step="$ctl/step.sh"
. "$step"
`

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newH builds the world. edit may change the configuration YAML before it is loaded, and the loaded configuration
// afterwards.
func newH(t *testing.T, yamlExtra string, edit func(*config.Config)) *h {
	t.Helper()
	root := t.TempDir()
	x := &h{t: t, f: fake.New(nil), audit: &recorder{}}
	x.master = x.f.AddUser("master", 1)
	x.bot = x.f.AddUser("agent", 2)
	x.mallory = x.f.AddUser("mallory", 3)
	x.f.AddRepo("o", "r", true)

	x.origin = filepath.Join(root, "origin.git")
	seed := filepath.Join(root, "seed")
	run(t, root, "git", "init", "-q", "--bare", "-b", "main", x.origin)
	run(t, root, "git", "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("sandbox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, seed, "git", "add", "README.md")
	run(t, seed, "git", "commit", "-q", "-m", "init")
	run(t, seed, "git", "push", "-q", x.origin, "main")
	x.clone = filepath.Join(root, "clone")
	run(t, root, "git", "clone", "-q", x.origin, x.clone)
	run(t, x.clone, "git", "config", "user.name", "agent")
	run(t, x.clone, "git", "config", "user.email", "agent@example.test")

	x.ctl = filepath.Join(root, "ctl")
	if err := os.MkdirAll(x.ctl, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "claude")
	if err := os.WriteFile(bin, []byte(strings.ReplaceAll(fakeClaude, "__CTL__", x.ctl)), 0o755); err != nil {
		t.Fatal(err)
	}
	x.step(0, waiting("I need a decision.", "Which default?"))

	yml := fmt.Sprintf(`version: 1
host: {name: test}
forges:
  github: {token_ref: t, agent_identity: agent}
trusted_authors: {github: [master]}
claude: {binary: %s}
%s
projects:
  - ref: gh:o/r
    path: %s
    host: test
`, bin, yamlExtra, x.clone)
	cfgPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Agent.Timeout = 20 * time.Second
	if edit != nil {
		edit(cfg)
	}
	x.cfg = cfg
	x.dir = state.Dir{Path: filepath.Join(root, "state")}
	x.ctx, x.stop = context.WithCancel(context.Background())
	t.Cleanup(func() {
		x.stop()
		if x.a != nil {
			x.a.Wait()
			x.a.Close()
		}
	})
	x.ref = x.f.AddIssue("o", "r", 12, "Add a flag", "Please add --dry-run.", x.master)
	return x
}

// agent makes a fresh agent over the same world, as a restart does.
func (x *h) agent() *Agent {
	x.t.Helper()
	if x.a != nil {
		x.a.Wait()
		x.a.Close()
	}
	j, err := trust.Resolve(x.ctx, x.f, x.dir, x.cfg.TrustedAuthors.GitHub, x.cfg.Forges.GitHub.AgentIdentity, x.audit)
	if err != nil {
		x.t.Fatal(err)
	}
	x.a = &Agent{
		Config: x.cfg, Forge: x.f, Judge: j, State: x.dir, Audit: x.audit,
		Env:      append(os.Environ(), x.envExtra...),
		IntGrace: 200 * time.Millisecond, TermGrace: 200 * time.Millisecond,
	}
	if err := x.a.Start(x.ctx); err != nil {
		x.t.Fatal(err)
	}
	return x.a
}

func (x *h) poll() {
	x.t.Helper()
	if x.a == nil {
		x.agent()
	}
	if err := x.a.Poll(x.ctx); err != nil {
		x.t.Fatal(err)
	}
}

// settle waits for every run, including the ones a finishing run starts.
func (x *h) settle() { x.a.Wait() }

// step sets what the fake claude does on its n-th start (0: every start without its own step).
func (x *h) step(n int, script string) {
	x.t.Helper()
	name := "step.sh"
	if n > 0 {
		name = fmt.Sprintf("step-%d.sh", n)
	}
	if err := os.WriteFile(filepath.Join(x.ctl, name), []byte(script), 0o644); err != nil {
		x.t.Fatal(err)
	}
}

// block makes a step wait for release(n) before going on.
func block(n int, then string) string {
	return fmt.Sprintf("touch \"$ctl/started-%d\"\nwhile [ ! -f \"$ctl/release-%d\" ]; do sleep 0.02; done\n%s", n, n, then)
}

func (x *h) release(n int) {
	x.t.Helper()
	if err := os.WriteFile(filepath.Join(x.ctl, fmt.Sprintf("release-%d", n)), nil, 0o644); err != nil {
		x.t.Fatal(err)
	}
}

func (x *h) waitStarted(n int) {
	x.t.Helper()
	x.waitFor(fmt.Sprintf("claude start %d", n), func() bool {
		_, err := os.Stat(filepath.Join(x.ctl, fmt.Sprintf("started-%d", n)))
		return err == nil
	})
}

func (x *h) waitFor(what string, cond func() bool) {
	x.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			x.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// starts is how many times the fake claude started.
func (x *h) starts() int {
	data, err := os.ReadFile(filepath.Join(x.ctl, "count"))
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscan(string(data), &n)
	return n
}

func (x *h) file(name string) string {
	x.t.Helper()
	data, err := os.ReadFile(filepath.Join(x.ctl, name))
	if err != nil {
		x.t.Fatal(err)
	}
	return string(data)
}

func (x *h) labels(ref forge.IssueRef) string { return strings.Join(x.f.Labels(ref), ",") }

// posts returns the agent's comments with their markers.
func (x *h) posts(ref forge.IssueRef) []post {
	var out []post
	for _, c := range x.f.CommentsOf(ref) {
		if c.Author.ID != x.bot.ID {
			continue
		}
		m, _ := marker.Parse(c.Body)
		out = append(out, post{Body: c.Body, Marker: m})
	}
	return out
}

type post struct {
	Body   string
	Marker *marker.Marker
}

func (x *h) lastPost(ref forge.IssueRef) post {
	x.t.Helper()
	ps := x.posts(ref)
	if len(ps) == 0 {
		x.t.Fatal("the agent posted nothing")
	}
	p := ps[len(ps)-1]
	if p.Marker == nil {
		x.t.Fatalf("the last post has no marker:\n%s", p.Body)
	}
	return p
}

// result is a stream-json result line with a structured output.
func result(structured map[string]any, denials ...map[string]any) string {
	if denials == nil {
		denials = []map[string]any{}
	}
	line, _ := json.Marshal(map[string]any{
		"type": "result", "subtype": "success", "is_error": false, "session_id": "s", "num_turns": 2,
		"result": "ok", "structured_output": structured, "permission_denials": denials,
	})
	return "cat <<'JSON'\n" + string(line) + "\nJSON\n"
}

func waiting(summary, question string) string {
	return result(map[string]any{"status": "waiting", "summary": summary, "question": question})
}

func done(summary string) string {
	return result(map[string]any{"status": "done", "summary": summary})
}

// commit is a step fragment that commits a file in the worktree.
func commit(name, body string) string {
	return fmt.Sprintf("mkdir -p \"$(dirname '%s')\"\nprintf '%%s' '%s' > '%s'\ngit add '%s'\ngit commit -q -m 'change %s'\n", name, body, name, name, name)
}

func (x *h) originHas(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "-q", "--verify", "refs/heads/"+branch)
	cmd.Dir = x.origin
	var out bytes.Buffer
	cmd.Stdout = &out
	return cmd.Run() == nil
}
