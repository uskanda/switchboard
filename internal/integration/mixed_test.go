package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

// mixedStep acts on whichever issue the prompt is about: without the user's reply it asks, with it it commits and
// declares done. Which issue runs first does not matter.
const mixedStep = `p="$ctl/prompt-$n.md"
ask() { cat <<JSON
{"type":"result","subtype":"success","is_error":false,"session_id":"s","num_turns":2,"result":"ok","structured_output":{"status":"waiting","summary":"Looked around.","question":"$1"},"permission_denials":[]}
JSON
}
finish() { printf '%s\n' "$2" > "$1"; git add "$1"; git commit -q -m "Add $1"; cat <<JSON
{"type":"result","subtype":"success","is_error":false,"session_id":"s","num_turns":3,"result":"ok","structured_output":{"status":"done","summary":"Added $1."},"permission_denials":[]}
JSON
}
if grep -q 'Add a greeting' "$p"; then
  if grep -q 'Use hi.' "$p"; then finish greeting.txt hi; else ask 'hello or hi?'; fi
else
  if grep -q 'Off by default.' "$p"; then finish dry-run.txt off; else ask 'On or off by default?'; fi
fi
`

func repoPair(t *testing.T, root, name string) (origin, clone string) {
	t.Helper()
	origin = filepath.Join(root, name+".git")
	seed := filepath.Join(root, name+"-seed")
	clone = filepath.Join(root, name)
	sh(t, root, "git", "init", "-q", "--bare", "-b", "main", origin)
	sh(t, root, "git", "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sh(t, seed, "git", "add", "README.md")
	sh(t, seed, "git", "commit", "-q", "-m", "init")
	sh(t, seed, "git", "push", "-q", origin, "main")
	sh(t, root, "git", "clone", "-q", origin, clone)
	sh(t, clone, "git", "config", "user.name", "agent")
	sh(t, clone, "git", "config", "user.email", "agent@example.test")
	return origin, clone
}

// Phase 2's completion condition on fakes: a GitHub and a GitLab issue wait in one inbox, both get a reply from
// it, and both end done, with a pull request and a merge request.
func TestBothForgesWaitInOneInboxAndFinish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := t.TempDir()
	ghOrigin, ghClone := repoPair(t, root, "tool")
	glOrigin, glClone := repoPair(t, root, "sb")

	ctl := filepath.Join(root, "ctl")
	if err := os.MkdirAll(ctl, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctl, "run.sh"), []byte(mixedStep), 0o644); err != nil {
		t.Fatal(err)
	}
	claude := strings.ReplaceAll(strings.Replace(fakeClaude, `. "$ctl/step-$n.sh"`, `. "$ctl/run.sh"`, 1), "__CTL__", ctl)
	bin := filepath.Join(root, "claude")
	if err := os.WriteFile(bin, []byte(claude), 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(root, "config.yaml")
	yml := fmt.Sprintf(`version: 1
host: {name: laptop}
forges:
  github: {token_ref: t, inbox_token_ref: i, agent_identity: bot, search_scope: "user:me"}
  gitlab: {base_url: "https://gl.test", token_ref: gt, inbox_token_ref: gi, agent_identity: bot}
trusted_authors: {github: [me], gitlab: [me]}
claude: {binary: %s}
projects:
  - {ref: "gh:me/tool", path: %s, host: laptop}
  - {ref: "gl:nas/sb", path: %s, host: laptop}
`, bin, ghClone, glClone)
	if err := os.WriteFile(cfgPath, []byte(yml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	gh, gl := fake.New(nil), fake.NewGitLab(nil)
	meGH, meGL := gh.AddUser("me", 1), gl.AddUser("me", 11)
	gh.AddUser("bot", 2)
	gl.AddUser("bot", 12)
	gh.AddRepo("me", "tool", true)
	gl.AddProject("nas", "sb", "private")
	if _, err := labels.Sync(ctx, gh.As("bot"), cfg.Projects[:1], cfg.Labels); err != nil {
		t.Fatal(err)
	}
	if _, err := labels.Sync(ctx, gl.As("bot"), cfg.Projects[1:], cfg.Labels); err != nil {
		t.Fatal(err)
	}
	ghRef := gh.AddIssue("me", "tool", 7, "Add --dry-run", "Add a --dry-run flag.", meGH)
	glRef := gl.AddIssue("nas", "sb", 3, "Add a greeting", "Add greeting.txt with one word.", meGL)
	gh.AddLabel(ghRef, cfg.Labels.Ready, meGH)
	gl.AddLabel(glRef, cfg.Labels.Ready, meGL)

	dir := state.Dir{Path: filepath.Join(root, "state")}
	audit := dir.Audit()
	jh, err := trust.ResolveFor(ctx, gh.As("bot"), dir, forge.GitHub, cfg.TrustedAuthors.GitHub, "bot", nil, audit)
	if err != nil {
		t.Fatal(err)
	}
	jl, err := trust.ResolveFor(ctx, gl.As("bot"), dir, forge.GitLab, cfg.TrustedAuthors.GitLab, "bot", nil, audit)
	if err != nil {
		t.Fatal(err)
	}
	a := &agent.Agent{Config: cfg, State: dir, Audit: audit, Routes: map[string]agent.Route{
		"gh:me/tool": {Forge: gh.As("bot"), Judge: jh},
		"gl:nas/sb":  {Forge: gl.As("bot"), Judge: jl},
	}}
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	box := &inbox.Inbox{Config: cfg, Forges: map[string]forge.Forge{forge.GitHub: gh.As("me"), forge.GitLab: gl.As("me")},
		Cache: filepath.Join(root, "cache", "inbox.json"), Now: time.Now}

	poll := func() {
		t.Helper()
		if err := a.Poll(ctx); err != nil {
			t.Fatal(err)
		}
		a.Wait()
	}

	// 1. Both issues run and ask.
	poll()
	if g, l := strings.Join(gh.Labels(ghRef), ","), strings.Join(gl.Labels(glRef), ","); g != cfg.Labels.Waiting || l != cfg.Labels.Waiting {
		t.Fatalf("after the first runs: github %q, gitlab %q", g, l)
	}

	// 2. One inbox lists both; the user replies to each from it.
	items, err := box.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("inbox = %+v", items)
	}
	answers := map[string]string{"gh:me/tool#7": "Off by default.", "gl:nas/sb#3": "Use hi."}
	for _, it := range items {
		ref, err := box.Resolve(fmt.Sprint(it.N))
		if err != nil {
			t.Fatal(err)
		}
		if err := box.Reply(ctx, ref, answers[it.Ref], false); err != nil {
			t.Fatalf("reply to %s: %v", it.Ref, err)
		}
	}

	// 3. Both replies resume their issue (the GitLab one without a change of its update time), and both finish.
	poll()
	poll()
	if g, l := strings.Join(gh.Labels(ghRef), ","), strings.Join(gl.Labels(glRef), ","); g != cfg.Labels.Done || l != cfg.Labels.Done {
		t.Fatalf("after the replies: github %q, gitlab %q", g, l)
	}
	if len(gh.PRs) != 1 || gh.PRs[0].Head != "switchboard/issue-7" || len(gl.PRs) != 1 || gl.PRs[0].Head != "switchboard/issue-3" || !gl.PRs[0].Draft {
		t.Fatalf("github PRs %+v, gitlab MRs %+v", gh.PRs, gl.PRs)
	}
	if got := sh(t, glOrigin, "git", "show", "switchboard/issue-3:greeting.txt"); got != "hi" {
		t.Fatalf("greeting.txt on the GitLab branch = %q", got)
	}
	if got := sh(t, ghOrigin, "git", "show", "switchboard/issue-7:dry-run.txt"); got != "off" {
		t.Fatalf("dry-run.txt on the GitHub branch = %q", got)
	}
	if items, err := box.List(ctx); err != nil || len(items) != 0 {
		t.Fatalf("inbox after done = %+v, %v", items, err)
	}
}
