package doctor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/secret"
	"github.com/uskanda/switchboard/internal/state"
)

type store map[string]string

func (s store) Get(ref string) (string, error) {
	if v, ok := s[ref]; ok {
		return v, nil
	}
	return "", secret.ErrNotFound
}

type world struct {
	d   *Doctor
	f   *fake.Forge
	out *bytes.Buffer
	cfg *config.Config
}

func claudeScript(t *testing.T, version string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho '"+version+" (Claude Code)'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func newWorld(t *testing.T) *world {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	f := fake.New(nil)
	f.AddUser("me", 1)
	f.AddUser("bot", 2)
	f.AddRepo("o", "r", true)
	cfg := &config.Config{
		Host:           config.Host{Name: "here"},
		Forges:         config.Forges{GitHub: config.GitHub{TokenRef: "agent", InboxTokenRef: "inbox", AgentIdentity: "bot"}},
		TrustedAuthors: config.TrustedAuthors{GitHub: []string{"me"}},
		Claude:         config.Claude{Binary: claudeScript(t, "2.1.312")},
		Projects:       []config.Project{{Ref: "gh:o/r", Owner: "o", Repo: "r", Host: "here", Path: repo}},
	}
	w := &world{f: f, out: &bytes.Buffer{}, cfg: cfg}
	w.d = &Doctor{
		Config:  cfg,
		State:   state.Dir{Path: t.TempDir()},
		Secrets: secret.Resolver{Store: store{"agent": "a", "inbox": "i"}},
		Forge:   func(string) forge.Forge { return f },
		Out:     w.out,
	}
	return w
}

func (w *world) run(t *testing.T) int {
	t.Helper()
	return w.d.Run(context.Background())
}

func (w *world) expect(t *testing.T, level, text string) {
	t.Helper()
	for _, line := range strings.Split(w.out.String(), "\n") {
		if strings.HasPrefix(line, level) && strings.Contains(line, text) {
			return
		}
	}
	t.Fatalf("no %q line with %q:\n%s", level, text, w.out.String())
}

func TestHealthyHost(t *testing.T) {
	w := newWorld(t)
	if n := w.run(t); n != 0 {
		t.Fatalf("%d failures:\n%s", n, w.out.String())
	}
	w.expect(t, "ok", "2.1.312")
	w.expect(t, "ok", "me")
	ids, err := w.d.State.Identities()
	if err != nil || ids["me"] != 1 || ids["bot"] != 2 {
		t.Fatalf("identities = %v, %v", ids, err)
	}
}

func TestMissingTokenFails(t *testing.T) {
	w := newWorld(t)
	w.d.Secrets = secret.Resolver{Store: store{}}
	if n := w.run(t); n == 0 {
		t.Fatal("a missing token passed")
	}
	w.expect(t, "fail", "agent")
	w.expect(t, "ok", "2.1.312") // checks that need no token still run
}

func TestMissingInboxTokenRefWarns(t *testing.T) {
	w := newWorld(t)
	w.cfg.Forges.GitHub.InboxTokenRef = ""
	if n := w.run(t); n != 0 {
		t.Fatalf("%d failures:\n%s", n, w.out.String())
	}
	w.expect(t, "warn", "inbox_token_ref")
}

func TestOldClaudeWarns(t *testing.T) {
	w := newWorld(t)
	w.cfg.Claude.Binary = claudeScript(t, "2.0.1")
	if n := w.run(t); n != 0 {
		t.Fatalf("%d failures:\n%s", n, w.out.String())
	}
	w.expect(t, "warn", "2.0.1")
}

func TestMissingClaudeFails(t *testing.T) {
	w := newWorld(t)
	w.cfg.Claude.Binary = filepath.Join(t.TempDir(), "nothing")
	if n := w.run(t); n == 0 {
		t.Fatal("a missing claude passed")
	}
	w.expect(t, "fail", "claude")
}

func TestChangedUserIDFails(t *testing.T) {
	w := newWorld(t)
	if err := w.d.State.SaveIdentities(map[string]int64{"me": 99}); err != nil {
		t.Fatal(err)
	}
	if n := w.run(t); n == 0 {
		t.Fatal("a changed user ID passed")
	}
	w.expect(t, "fail", "me")
}

func TestProjectChecks(t *testing.T) {
	w := newWorld(t)
	w.cfg.Projects[0].Path = t.TempDir()
	if n := w.run(t); n == 0 {
		t.Fatal("a project path that is not a git repository passed")
	}
	w.expect(t, "fail", "git")

	w = newWorld(t)
	w.f.AddRepo("o", "r", false)
	if n := w.run(t); n != 0 {
		t.Fatalf("%d failures:\n%s", n, w.out.String())
	}
	w.expect(t, "warn", "public")

	w = newWorld(t)
	w.f.AddRepo("o", "r", false)
	w.cfg.Projects[0].IncludeUntrusted = true
	if n := w.run(t); n == 0 {
		t.Fatal("include_untrusted on a public repository passed")
	}
	w.expect(t, "fail", "include_untrusted")
}
