//go:build realclaude

package integration

// This test runs the real claude CLI (claude.binary: $SWITCHBOARD_CLAUDE, else claude on PATH) through the whole
// Phase 1 loop on the fake forge and a real git repository: ready, a question, a reply, done with a pull request.
// It calls the model, so it runs only with the realclaude build tag:
//
//	go test -tags realclaude -run RealClaude -v -timeout 30m ./internal/integration/

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

func TestRealClaudeFromReadyThroughReplyToDone(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	seed := filepath.Join(root, "seed")
	clone := filepath.Join(root, "clone")
	sh(t, root, "git", "init", "-q", "--bare", "-b", "main", origin)
	sh(t, root, "git", "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("A sandbox repository for switchboard.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sh(t, seed, "git", "add", "README.md")
	sh(t, seed, "git", "commit", "-q", "-m", "init")
	sh(t, seed, "git", "push", "-q", origin, "main")
	sh(t, root, "git", "clone", "-q", origin, clone)
	sh(t, clone, "git", "config", "user.name", "switchboard sandbox")
	sh(t, clone, "git", "config", "user.email", "sandbox@example.test")

	bin := os.Getenv("SWITCHBOARD_CLAUDE")
	if bin == "" {
		bin = "claude"
	}
	cfgPath := filepath.Join(root, "config.yaml")
	yml := fmt.Sprintf(`version: 1
host: {name: sandbox}
forges:
  github: {token_ref: t, inbox_token_ref: i, agent_identity: bot, search_scope: "user:me"}
trusted_authors: {github: [me]}
claude: {binary: %s}
agent: {timeout: 10m, max_turns: 30}
projects:
  - ref: gh:me/sandbox
    path: %s
    host: sandbox
    verify_command: grep -qx hi greeting.txt
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
	f.AddRepo("me", "sandbox", true)
	if _, err := labels.Sync(ctx, f.As("bot"), cfg.Projects, cfg.Labels); err != nil {
		t.Fatal(err)
	}
	ref := f.AddIssue("me", "sandbox", 1, "Add a greeting file",
		"Create greeting.txt containing a single greeting word on one line, and commit it.\n\n"+
			"Before you change anything, ask me which word to use: hello or hi. Do not pick one yourself.", me)
	f.AddLabel(ref, cfg.Labels.Ready, me)

	dir := state.Dir{Path: filepath.Join(root, "state")}
	audit := dir.Audit()
	j, err := trust.Resolve(ctx, f.As("bot"), dir, cfg.TrustedAuthors.GitHub, cfg.Forges.GitHub.AgentIdentity, audit)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	a := &agent.Agent{Config: cfg, Forge: f.As("bot"), Judge: j, State: dir, Audit: audit, Log: &log}
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	box := &inbox.Inbox{Config: cfg, Forge: f.As("me"), Cache: filepath.Join(root, "inbox.json"), Now: time.Now}
	last := func() string {
		cs := f.CommentsOf(ref)
		if len(cs) == 0 {
			return "(no comment)"
		}
		return cs[len(cs)-1].Body
	}

	start := time.Now()
	if err := a.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	a.Wait()
	t.Logf("first run took %s; the agent posted:\n%s", time.Since(start).Round(time.Second), last())
	items, err := box.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Reason != "question" {
		t.Fatalf("after the first run: labels %v, inbox %+v\nlog:\n%s", f.Labels(ref), items, log.String())
	}
	t.Logf("inbox: %+v", items[0])
	if err := box.Reply(ctx, ref, "Use hi.", false); err != nil {
		t.Fatal(err)
	}

	start = time.Now()
	if err := a.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	a.Wait()
	t.Logf("second run took %s; the agent posted:\n%s", time.Since(start).Round(time.Second), last())
	if got := strings.Join(f.Labels(ref), ","); got != cfg.Labels.Done || len(f.PRs) != 1 {
		t.Fatalf("after the second run: labels %q, PRs %d\nlog:\n%s", got, len(f.PRs), log.String())
	}
	t.Logf("pull request: %+v", f.PRs[0])
	if got := sh(t, origin, "git", "show", "switchboard/issue-1:greeting.txt"); got != "hi" {
		t.Fatalf("pushed greeting.txt = %q", got)
	}
	if got := sh(t, origin, "git", "log", "--format=%s", "-1", "main"); got != "init" {
		t.Fatalf("main moved: %q", got)
	}
}
