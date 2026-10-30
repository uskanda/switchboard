// Package integration runs switchboard end to end on a real git repository, the fake forge and a fake claude:
// an issue goes from ready to waiting, gets a reply through the inbox, and ends done with a pull request.
package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

func sh(t *testing.T, dir, name string, args ...string) string {
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

// The fake claude runs $ctl/step-<n>.sh on its n-th start, after saving the prompt.
const fakeClaude = `#!/bin/bash
ctl='__CTL__'
n=$(( $(cat "$ctl/count" 2>/dev/null || echo 0) + 1 ))
echo $n > "$ctl/count"
cat > "$ctl/prompt-$n.md"
. "$ctl/step-$n.sh"
`

const firstRun = `cat <<'JSON'
{"type":"result","subtype":"success","is_error":false,"session_id":"s","num_turns":3,"result":"ok","structured_output":{"status":"waiting","summary":"Read the flag parser.","question":"Should --dry-run be on or off by default?"},"permission_denials":[]}
JSON
`

// The second run fails unless the prompt carries the reply, then commits the change and declares done.
const secondRun = `grep -q 'Off by default, please.' "$ctl/prompt-2.md" || exit 3
printf 'dry-run: off\n' > dry-run.txt
git add dry-run.txt
git commit -q -m 'Add --dry-run, off by default'
cat <<'JSON'
{"type":"result","subtype":"success","is_error":false,"session_id":"s","num_turns":5,"result":"ok","structured_output":{"status":"done","summary":"Added --dry-run, off by default, with a test."},"permission_denials":[]}
JSON
`

func TestIssueFromReadyThroughReplyToDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := t.TempDir()

	// A repository with an origin, as on the user's machine.
	origin := filepath.Join(root, "origin.git")
	seed := filepath.Join(root, "seed")
	clone := filepath.Join(root, "clone")
	sh(t, root, "git", "init", "-q", "--bare", "-b", "main", origin)
	sh(t, root, "git", "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("tool\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sh(t, seed, "git", "add", "README.md")
	sh(t, seed, "git", "commit", "-q", "-m", "init")
	sh(t, seed, "git", "push", "-q", origin, "main")
	sh(t, root, "git", "clone", "-q", origin, clone)
	sh(t, clone, "git", "config", "user.name", "agent")
	sh(t, clone, "git", "config", "user.email", "agent@example.test")

	ctl := filepath.Join(root, "ctl")
	if err := os.MkdirAll(ctl, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"step-1.sh": firstRun, "step-2.sh": secondRun} {
		if err := os.WriteFile(filepath.Join(ctl, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(root, "claude")
	if err := os.WriteFile(bin, []byte(strings.ReplaceAll(fakeClaude, "__CTL__", ctl)), 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(root, "config.yaml")
	yml := fmt.Sprintf(`version: 1
host: {name: laptop}
forges:
  github: {token_ref: t, inbox_token_ref: i, agent_identity: bot, search_scope: "user:me"}
trusted_authors: {github: [me]}
claude: {binary: %s}
projects:
  - ref: gh:me/tool
    path: %s
    host: laptop
    verify_command: test -f dry-run.txt
`, bin, clone)
	if err := os.WriteFile(cfgPath, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	f := fake.New(nil)
	me := f.AddUser("me", 1)
	f.AddUser("bot", 2)
	f.AddRepo("me", "tool", true)
	if _, err := labels.Sync(ctx, f.As("bot"), cfg.Projects, cfg.Labels); err != nil {
		t.Fatal(err)
	}
	ref := f.AddIssue("me", "tool", 7, "Add --dry-run", "Add a --dry-run flag that prints what would change.", me)
	f.AddLabel(ref, cfg.Labels.Ready, me)

	dir := state.Dir{Path: filepath.Join(root, "state")}
	audit := dir.Audit()
	agentForge := f.As("bot")
	j, err := trust.Resolve(ctx, agentForge, dir, cfg.TrustedAuthors.GitHub, cfg.Forges.GitHub.AgentIdentity, audit)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{Config: cfg, Forge: agentForge, Judge: j, State: dir, Audit: audit}
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	box := &inbox.Inbox{Config: cfg, Forge: f.As("me"), Cache: filepath.Join(root, "cache", "inbox.json"), Now: time.Now}

	// 1. ready -> running -> waiting with the question.
	if err := a.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	a.Wait()
	if got := strings.Join(f.Labels(ref), ","); got != cfg.Labels.Waiting {
		t.Fatalf("after the first run: labels %q", got)
	}

	// 2. The inbox lists it; the user reads the question and replies.
	items, err := box.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Ref != "gh:me/tool#7" || items[0].Host != "laptop" || items[0].Reason != "question" ||
		items[0].Question != "Should --dry-run be on or off by default?" {
		t.Fatalf("inbox = %+v", items)
	}
	row, err := box.Resolve("1")
	if err != nil {
		t.Fatal(err)
	}
	var shown bytes.Buffer
	if err := box.Show(ctx, row, &shown); err != nil || !strings.Contains(shown.String(), "Read the flag parser.") {
		t.Fatalf("show = %v\n%s", err, shown.String())
	}
	if err := box.Reply(ctx, row, "Off by default, please.", false); err != nil {
		t.Fatal(err)
	}

	// 3. The reply resumes the issue; the run commits, passes verification and opens a draft pull request.
	if err := a.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	a.Wait()
	if got := strings.Join(f.Labels(ref), ","); got != cfg.Labels.Done {
		cs := f.CommentsOf(ref)
		t.Fatalf("after the second run: labels %q, last comment:\n%s", got, cs[len(cs)-1].Body)
	}
	if len(f.PRs) != 1 || !f.PRs[0].Draft || f.PRs[0].Head != "switchboard/issue-7" || !strings.Contains(f.PRs[0].Body, "Closes #7") {
		t.Fatalf("PRs = %+v", f.PRs)
	}
	if got := sh(t, origin, "git", "log", "--format=%s", "-1", "switchboard/issue-7"); got != "Add --dry-run, off by default" {
		t.Fatalf("pushed branch head = %q", got)
	}
	if got := sh(t, origin, "git", "log", "--format=%s", "-1", "main"); got != "init" {
		t.Fatalf("main moved: %q", got)
	}

	// The thread: claim, question, reply, claim, done; every agent post carries its marker.
	var states []string
	for _, c := range f.CommentsOf(ref) {
		if m, ok := marker.Parse(c.Body); ok && c.Author.Login == "bot" {
			states = append(states, m.State)
		} else {
			states = append(states, "reply by "+c.Author.Login)
		}
	}
	if got := strings.Join(states, " | "); got != "running | waiting | reply by me | running | done" {
		t.Fatalf("thread = %s", got)
	}
	if items, err := box.List(ctx); err != nil || len(items) != 0 {
		t.Fatalf("inbox after done = %+v, %v", items, err)
	}
}
