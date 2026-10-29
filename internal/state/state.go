// Package state manages the state directory: the audit log, the resolved user IDs, run records and locks.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// DefaultDir is $XDG_STATE_HOME/switchboard, with ~/.local/state when XDG_STATE_HOME is unset.
func DefaultDir() string {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(base, "switchboard")
}

// Dir is a state directory.
type Dir struct {
	Path string
}

// Ensure creates the directory, readable by the owner only.
func (d Dir) Ensure() error { return os.MkdirAll(d.Path, 0o700) }

// Auditor records events. Tests substitute their own.
type Auditor interface {
	Log(event string, fields map[string]any)
}

// Audit is the append-only audit log, audit.jsonl, one JSON object per line.
type Audit struct {
	path string
	mu   sync.Mutex
	now  func() time.Time
}

// Audit returns the audit log of d.
func (d Dir) Audit() *Audit {
	return &Audit{path: filepath.Join(d.Path, "audit.jsonl"), now: time.Now}
}

// Log appends one record with the event name, the time and fields. A failure to write is reported on stderr: an
// audit gap must be visible, but must not stop the agent.
func (a *Audit) Log(event string, fields map[string]any) {
	rec := map[string]any{"at": a.now().UTC().Format(time.RFC3339Nano), "event": event}
	for k, v := range fields {
		rec[k] = v
	}
	line, err := json.Marshal(rec)
	if err == nil {
		a.mu.Lock()
		err = appendLine(a.path, line)
		a.mu.Unlock()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "switchboard: audit log: %v\n", err)
	}
}

func appendLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// Identities returns the recorded login-to-ID map (empty when none was recorded).
func (d Dir) Identities() (map[string]int64, error) {
	out := map[string]int64{}
	data, err := os.ReadFile(filepath.Join(d.Path, "identities.json"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("identities.json: %w", err)
	}
	return out, nil
}

// SaveIdentities writes the login-to-ID map.
func (d Dir) SaveIdentities(ids map[string]int64) error {
	return d.WriteJSON("identities.json", ids)
}

// WriteJSON writes v as JSON to name inside the directory, replacing it atomically.
func (d Dir) WriteJSON(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(d.Path, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadJSON reads name inside the directory into v. It returns os.ErrNotExist (wrapped) when the file is missing.
func (d Dir) ReadJSON(name string, v any) error {
	data, err := os.ReadFile(filepath.Join(d.Path, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// ErrLocked reports that another holder has the lock.
var ErrLocked = errors.New("locked")

// Lock takes the exclusive lock locks/<name>.lock without waiting and returns the function that releases it. The
// lock is an flock, so it is released when the process exits, even after a crash.
func (d Dir) Lock(name string) (func(), error) {
	dir := filepath.Join(d.Path, "locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, name+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%s: %w", name, ErrLocked)
		}
		return nil, err
	}
	return func() { f.Close() }, nil
}
