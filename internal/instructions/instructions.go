// Package instructions keeps the user's additional instructions for the runs in the plan stage: one file for every
// project and one per project, in the instructions directory next to the configuration file.
package instructions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/workspace"
)

// Global is the target of the instructions for every project.
const Global = ""

// MaxSize is the largest text Save accepts, in bytes.
const MaxSize = 32 << 10

var (
	// ErrTooLarge is returned by Save for a text larger than MaxSize.
	ErrTooLarge = fmt.Errorf("the instructions are larger than %d KiB", MaxSize>>10)
	// ErrSecret is returned by Save for a text that looks like it holds a secret.
	ErrSecret = errors.New("the instructions look like they hold a secret")
)

// Store reads and writes the instruction files under Dir. Audit records every save; it may be nil for a store that
// only reads.
type Store struct {
	Dir   string
	Audit state.Auditor
}

// For returns the store next to the configuration file at configPath.
func For(configPath string, audit state.Auditor) Store {
	return Store{Dir: filepath.Join(filepath.Dir(configPath), "instructions"), Audit: audit}
}

var (
	ghRef = regexp.MustCompile(`^gh:([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)$`)
	glRef = regexp.MustCompile(`^gl:((?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)$`)
)

// Path returns the file of target: Global, or a project as "gh:owner/repo" or "gl:group/sub/project".
func (s Store) Path(target string) (string, error) {
	if target == Global {
		return filepath.Join(s.Dir, "plan.md"), nil
	}
	var parts []string
	switch {
	case ghRef.MatchString(target):
		m := ghRef.FindStringSubmatch(target)
		parts = []string{"gh", m[1], m[2]}
	case glRef.MatchString(target):
		m := glRef.FindStringSubmatch(target)
		parts = append(append([]string{"gl"}, strings.Split(m[1], "/")...), m[2])
	default:
		return "", fmt.Errorf("%q is not a project of the form gh:owner/repo or gl:group/project", target)
	}
	for _, p := range parts {
		if p == "." || p == ".." {
			return "", fmt.Errorf("%q is not a project of the form gh:owner/repo or gl:group/project", target)
		}
	}
	return filepath.Join(append(append([]string{s.Dir}, parts...), "plan.md")...), nil
}

// Read returns the instructions of target, or "" when there is no file.
func (s Store) Read(target string) (string, error) {
	p, err := s.Path(target)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// Plan returns what a plan run of project gets: the global instructions, then the project's, each left out when it
// is blank or cannot be read.
func (s Store) Plan(project string) string {
	var parts []string
	for _, t := range []string{Global, project} {
		if text, err := s.Read(t); err == nil && strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
	}
	return strings.Join(parts, "\n\n")
}

// Save replaces the instructions of target with text, after the size and secret checks. The file is written in
// full before it replaces the old one, so a run never reads half of it.
func (s Store) Save(target, text string) error {
	if len(text) > MaxSize {
		return ErrTooLarge
	}
	if workspace.HasSecret(text) {
		return ErrSecret
	}
	p, err := s.Path(target)
	if err != nil {
		return err
	}
	if err := s.mkdirs(filepath.Dir(p)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".plan-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
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
		sum := sha256.Sum256([]byte(text))
		name := target
		if name == Global {
			name = "global"
		}
		s.Audit.Log("instructions.saved", map[string]any{"target": name, "sha256": hex.EncodeToString(sum[:])})
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
