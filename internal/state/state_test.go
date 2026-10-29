package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirUsesXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/xdg")
	if got := DefaultDir(); got != "/xdg/switchboard" {
		t.Errorf("DefaultDir() = %q", got)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/alice")
	if got := DefaultDir(); got != "/home/alice/.local/state/switchboard" {
		t.Errorf("DefaultDir() = %q", got)
	}
}

func TestAuditAppendsOneJSONLinePerEvent(t *testing.T) {
	d := Dir{Path: t.TempDir()}
	a := d.Audit()
	a.Log("trust.comment", map[string]any{"verdict": "accept"})
	a.Log("labels.normalized", map[string]any{"issue": "o/r#1"})
	data, err := os.ReadFile(filepath.Join(d.Path, "audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %q", lines)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatal(err)
	}
	if rec["event"] != "trust.comment" || rec["verdict"] != "accept" || rec["at"] == nil {
		t.Fatalf("record = %v", rec)
	}
	info, _ := os.Stat(filepath.Join(d.Path, "audit.jsonl"))
	if info.Mode().Perm() != 0o600 {
		t.Errorf("audit log mode = %v", info.Mode().Perm())
	}
}

func TestIdentitiesRoundTrip(t *testing.T) {
	d := Dir{Path: t.TempDir()}
	got, err := d.Identities()
	if err != nil || len(got) != 0 {
		t.Fatalf("empty Identities = %v, %v", got, err)
	}
	if err := d.SaveIdentities(map[string]int64{"alice": 1}); err != nil {
		t.Fatal(err)
	}
	got, err = d.Identities()
	if err != nil || got["alice"] != 1 {
		t.Fatalf("Identities = %v, %v", got, err)
	}
}
