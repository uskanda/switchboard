// Package workspace manages the git worktree of one issue: creating it, checking the Claude Code configuration
// before a run, checking the output before a push, and pushing the issue branch.
package workspace

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Space is the worktree of one issue.
type Space struct {
	Repo   string // the local clone of the project
	Root   string // the directory under which this project's worktrees live
	Base   string // the base branch
	Number int    // the issue number
	Branch string // the issue branch; switchboard/issue-<n> when empty
	Env    []string
	// Token, when set, is offered to git push through GIT_ASKPASS for https remotes. It never reaches claude.
	Token string
}

// Dir is the worktree path.
func (s *Space) Dir() string { return filepath.Join(s.Root, fmt.Sprintf("issue-%d", s.Number)) }

func (s *Space) branch() string {
	if s.Branch != "" {
		return s.Branch
	}
	return fmt.Sprintf("switchboard/issue-%d", s.Number)
}

func (s *Space) base() string { return "origin/" + s.Base }

func (s *Space) git(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(append([]string(nil), s.Env...), env...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// Prepare fetches the base branch and returns the worktree, creating it from the base branch on first use.
func (s *Space) Prepare(ctx context.Context) (string, error) {
	if _, err := s.git(ctx, s.Repo, nil, "fetch", "-q", "origin", s.Base); err != nil {
		return "", err
	}
	dir := s.Dir()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return dir, nil
	}
	if err := os.MkdirAll(s.Root, 0o700); err != nil {
		return "", err
	}
	if _, err := s.git(ctx, s.Repo, nil, "rev-parse", "-q", "--verify", "refs/heads/"+s.branch()); err == nil {
		_, err = s.git(ctx, s.Repo, nil, "worktree", "add", "-q", dir, s.branch())
		return dir, err
	}
	_, err := s.git(ctx, s.Repo, nil, "worktree", "add", "-q", "-b", s.branch(), dir, s.base())
	return dir, err
}

// claudeConfig are the paths Claude Code reads its project configuration and instructions from.
var claudeConfig = []string{".claude", ".mcp.json", "CLAUDE.md", "AGENTS.md"}

// CheckClaudeConfig fails when the worktree's Claude Code configuration differs from the base branch: changed,
// committed on the issue branch, or present but untracked (ignored files included). ADR-018.
func (s *Space) CheckClaudeConfig(ctx context.Context) error {
	dir := s.Dir()
	changed, err := s.git(ctx, dir, nil, append([]string{"diff", "--name-only", s.base(), "--"}, claudeConfig...)...)
	if err != nil {
		return err
	}
	untracked, err := s.git(ctx, dir, nil, append([]string{"ls-files", "--others", "--"}, claudeConfig...)...)
	if err != nil {
		return err
	}
	files := nonEmpty(strings.Split(changed+"\n"+untracked, "\n"))
	if len(files) > 0 {
		return fmt.Errorf("the Claude Code configuration differs from %s: %s", s.base(), strings.Join(files, ", "))
	}
	return nil
}

func nonEmpty(xs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

// protected are the paths a run must not change: directories end with a slash.
var protected = []string{".github/workflows/", ".claude/", ".mcp.json", "CLAUDE.md", "AGENTS.md"}

func isProtected(path string, extra []string) bool {
	for _, p := range append(append([]string(nil), protected...), extra...) {
		if path == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p)) {
			return true
		}
	}
	return false
}

// secretPatterns match tokens and private keys.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
}

// HasSecret reports whether s contains something that looks like a token or a private key.
func HasSecret(s string) bool {
	for _, p := range secretPatterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

// CheckOutput returns the reasons the issue branch must not be pushed: it is not the issue branch, it has no
// commit ahead of the base, it changes a protected path, or a diff, a commit message or one of texts contains a
// secret. A problem names the file or the text by its position, never the secret itself.
func (s *Space) CheckOutput(ctx context.Context, extraProtected, texts []string) ([]string, error) {
	dir := s.Dir()
	var problems []string
	cur, err := s.git(ctx, dir, nil, "branch", "--show-current")
	if err != nil {
		return nil, err
	}
	if cur != s.branch() {
		problems = append(problems, fmt.Sprintf("the worktree is on %q, not on %s", cur, s.branch()))
	}
	ahead, err := s.git(ctx, dir, nil, "rev-list", "--count", s.base()+"..HEAD")
	if err != nil {
		return nil, err
	}
	if n, _ := strconv.Atoi(ahead); n == 0 {
		problems = append(problems, fmt.Sprintf("there is no commit ahead of %s", s.base()))
		return problems, nil
	}
	names, err := s.git(ctx, dir, nil, "diff", "--name-only", s.base()+"...HEAD")
	if err != nil {
		return nil, err
	}
	for _, f := range nonEmpty(strings.Split(names, "\n")) {
		if isProtected(f, extraProtected) {
			problems = append(problems, fmt.Sprintf("the change touches the protected path %s", f))
		}
		diff, err := s.git(ctx, dir, nil, "diff", s.base()+"...HEAD", "--", f)
		if err != nil {
			return nil, err
		}
		if HasSecret(addedLines(diff)) {
			problems = append(problems, fmt.Sprintf("%s adds what looks like a secret", f))
		}
	}
	msgs, err := s.git(ctx, dir, nil, "log", "--format=%B", s.base()+"..HEAD")
	if err != nil {
		return nil, err
	}
	if HasSecret(msgs) {
		problems = append(problems, "a commit message contains what looks like a secret")
	}
	for i, t := range texts {
		if HasSecret(t) {
			problems = append(problems, fmt.Sprintf("text %d to be posted contains what looks like a secret", i+1))
		}
	}
	return problems, nil
}

func addedLines(diff string) string {
	var b strings.Builder
	sc := bufio.NewScanner(strings.NewReader(diff))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		l := sc.Text()
		if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
			b.WriteString(l[1:])
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// askpass answers git's credential prompts from SWITCHBOARD_GIT_TOKEN.
const askpass = `#!/bin/sh
case "$1" in
Username*) echo x-access-token ;;
*) printf '%s\n' "$SWITCHBOARD_GIT_TOKEN" ;;
esac
`

// Push pushes the issue branch, and only that branch, to origin. Pushing the base branch is refused.
func (s *Space) Push(ctx context.Context) error {
	b := s.branch()
	if b == s.Base || b == "main" || b == "master" {
		return fmt.Errorf("refusing to push %s: switchboard pushes only issue branches", b)
	}
	var env []string
	if s.Token != "" {
		tmp, err := os.MkdirTemp("", "switchboard-askpass-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		script := filepath.Join(tmp, "askpass")
		if err := os.WriteFile(script, []byte(askpass), 0o700); err != nil {
			return err
		}
		env = []string{"GIT_ASKPASS=" + script, "SWITCHBOARD_GIT_TOKEN=" + s.Token, "GIT_TERMINAL_PROMPT=0"}
	}
	_, err := s.git(ctx, s.Dir(), env, "push", "-q", "origin", "refs/heads/"+b+":refs/heads/"+b)
	return err
}
