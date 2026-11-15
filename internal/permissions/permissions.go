// Package permissions keeps the allow rules added to the runs that write files from the web inbox: one file for
// every project, one per project and one per issue, under the configuration's directory (add-permission-grants).
// A file holds one rule per line; blank lines and lines starting with # are left out. The agent reads the files at
// the start of every run that writes files, after the allow_tools_extra of the configuration.
package permissions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/uskanda/switchboard/internal/claudecode"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/state"
)

// MaxRuleLen is the longest rule the web inbox may add.
const MaxRuleLen = 200

// Scope is where rules apply: every project when Project is empty, a project ("gh:owner/repo" or
// "gl:group/project"), or one issue of it when Issue is set.
type Scope struct {
	Project string
	Issue   int
}

// String names the scope in the audit log: "global", the project, or the issue as "gh:owner/repo#12".
func (sc Scope) String() string {
	switch {
	case sc.Project == "":
		return "global"
	case sc.Issue > 0:
		return sc.Project + "#" + strconv.Itoa(sc.Issue)
	}
	return sc.Project
}

// Store reads and writes the rule files under Dir. Audit records every change.
type Store struct {
	Dir   string
	Audit state.Auditor
}

// For is the store next to the configuration file at configPath.
func For(configPath string, audit state.Auditor) Store {
	return Store{Dir: filepath.Join(filepath.Dir(configPath), "permissions"), Audit: audit}
}

var (
	ghRef = regexp.MustCompile(`^gh:([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)$`)
	glRef = regexp.MustCompile(`^gl:((?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)$`)
)

// Path returns the file of sc.
func (s Store) Path(sc Scope) (string, error) {
	if sc.Project == "" {
		if sc.Issue != 0 {
			return "", fmt.Errorf("an issue needs its project")
		}
		return filepath.Join(s.Dir, "allow.txt"), nil
	}
	var parts []string
	switch {
	case ghRef.MatchString(sc.Project):
		m := ghRef.FindStringSubmatch(sc.Project)
		parts = []string{"gh", m[1], m[2]}
	case glRef.MatchString(sc.Project):
		m := glRef.FindStringSubmatch(sc.Project)
		parts = append(append([]string{"gl"}, strings.Split(m[1], "/")...), m[2])
	default:
		return "", fmt.Errorf("%q is not a project of the form gh:owner/repo or gl:group/project", sc.Project)
	}
	for _, p := range parts {
		if p == "." || p == ".." {
			return "", fmt.Errorf("%q is not a project of the form gh:owner/repo or gl:group/project", sc.Project)
		}
	}
	switch {
	case sc.Issue < 0:
		return "", fmt.Errorf("issue %d is no issue number", sc.Issue)
	case sc.Issue > 0:
		parts = append(parts, "issues", strconv.Itoa(sc.Issue)+".txt")
	default:
		parts = append(parts, "allow.txt")
	}
	return filepath.Join(append([]string{s.Dir}, parts...)...), nil
}

// Read returns the rules of sc, in order, each once; none when there is no file.
func (s Store) Read(sc Scope) ([]string, error) {
	p, err := s.Path(sc)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return rules(string(b)), nil
}

// rules are the rules of a file's text.
func rules(text string) []string {
	out := []string{}
	for _, l := range strings.Split(text, "\n") {
		if r := strings.TrimSpace(l); r != "" && !strings.HasPrefix(r, "#") && !slices.Contains(out, r) {
			out = append(out, r)
		}
	}
	return out
}

// RuleError is the refusal of rules that Check does not pass: nothing was saved.
type RuleError struct {
	Rules []Refused
}

// Refused is one rule Check refused, and why.
type Refused struct {
	Rule   string `json:"rule"`
	Reason string `json:"reason"`
}

func (e *RuleError) Error() string {
	var parts []string
	for _, r := range e.Rules {
		parts = append(parts, fmt.Sprintf("%q %s", r.Rule, r.Reason))
	}
	return "refused: " + strings.Join(parts, "; ")
}

// locks serializes the changes of the files of one store, by its directory.
var locks sync.Map

func (s Store) lock() func() {
	m, _ := locks.LoadOrStore(filepath.Clean(s.Dir), &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// Add appends to the file of sc the rules it does not hold yet. When Check refuses any of them, nothing is written
// and the error is a *RuleError naming each.
func (s Store) Add(sc Scope, add []string) error {
	var refused []Refused
	var clean []string
	for _, r := range add {
		r = strings.TrimSpace(r)
		if why := Check(r); why != "" {
			refused = append(refused, Refused{Rule: r, Reason: why})
			continue
		}
		if !slices.Contains(clean, r) {
			clean = append(clean, r)
		}
	}
	if len(refused) > 0 {
		return &RuleError{Rules: refused}
	}
	return s.change(sc, "permission.saved", clean, func(lines []string) []string {
		have := rules(strings.Join(lines, "\n"))
		for _, r := range clean {
			if !slices.Contains(have, r) {
				lines = append(lines, r)
			}
		}
		return lines
	})
}

// Remove drops from the file of sc the lines that are one of rules. A rule the file does not hold is no error.
func (s Store) Remove(sc Scope, remove []string) error {
	for i := range remove {
		remove[i] = strings.TrimSpace(remove[i])
	}
	return s.change(sc, "permission.removed", remove, func(lines []string) []string {
		return slices.DeleteFunc(lines, func(l string) bool { return slices.Contains(remove, strings.TrimSpace(l)) })
	})
}

// change rewrites the file of sc with edit, keeping every other line as it was, and logs event with the rules.
func (s Store) change(sc Scope, event string, changed []string, edit func([]string) []string) error {
	p, err := s.Path(sc)
	if err != nil {
		return err
	}
	defer s.lock()()
	b, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var lines []string
	if text := strings.TrimRight(string(b), "\n"); text != "" {
		lines = strings.Split(text, "\n")
	}
	lines = edit(lines)
	if err := s.mkdirs(filepath.Dir(p)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".allow-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	text := strings.Join(lines, "\n")
	if text != "" {
		text += "\n"
	}
	if _, err := tmp.WriteString(text); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Log(event, map[string]any{"scope": sc.String(), "rules": changed})
	}
	return nil
}

// mkdirs creates dir and the missing directories above it, up to and including s.Dir, with mode 0700.
func (s Store) mkdirs(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if dir != s.Dir {
		if err := s.mkdirs(filepath.Dir(dir)); err != nil {
			return err
		}
	}
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// Check tells why the web inbox may not add rule, or "" when it may: what the configuration refuses
// (config.CheckAllowRule), and, since git runs outside the sandbox, git without a subcommand, git config, an option
// before the subcommand (git -c could run any program), and a rule longer than MaxRuleLen.
func Check(rule string) string {
	rule = strings.TrimSpace(rule)
	if why := config.CheckAllowRule(rule); why != "" {
		return why
	}
	if len(rule) > MaxRuleLen {
		return fmt.Sprintf("is longer than %d characters", MaxRuleLen)
	}
	words := config.RuleWords(rule)
	switch {
	case len(words) == 0:
	case strings.Contains(words[0], "*"):
		return "allows every command"
	case words[0] != "git":
	case len(words) == 1 || strings.Contains(words[1], "*"):
		return "allows every git command, outside the sandbox"
	case words[1] == "config" || strings.HasPrefix(words[1], "-"):
		return "could run any program outside the sandbox through git"
	}
	return ""
}

// compoundStarts are the first words of a command a rule cannot match as a whole.
var compoundStarts = []string{"for", "while", "until", "if", "case", "cd", "{", "("}

// Candidates returns the rules that would let the refused call of tool with command through, as the web inbox offers
// them: the tool's name for a tool other than Bash; for a command, its first word, or git and its subcommand, with
// and without arguments. compound reports a command joined with others (a pipe, &&, ;, a loop, cd), which no rule
// matches. Candidates that Check refuses are left out.
func Candidates(tool, command string) (out []string, compound bool) {
	if tool != "Bash" {
		if Check(tool) == "" {
			out = []string{tool}
		}
		return out, false
	}
	command = strings.TrimSpace(command)
	words := strings.Fields(command)
	if len(words) == 0 {
		return nil, false
	}
	if strings.ContainsAny(command, "|&;<>\n") || strings.Contains(command, "$(") || slices.Contains(compoundStarts, words[0]) {
		return nil, true
	}
	var cands []string
	switch {
	case strings.Contains(words[0], "="):
		return nil, false // an environment assignment before the command
	case words[0] == "git" && len(words) > 1:
		cands = claudecode.Commands("git " + words[1])
	default:
		cands = claudecode.Commands(words[0])
	}
	for _, c := range cands {
		if Check(c) == "" {
			out = append(out, c)
		}
	}
	return out, false
}
