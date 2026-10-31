package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	t.Setenv("EDITOR", "")
	if got := Default(); got != "vi" {
		t.Fatalf("Default = %q", got)
	}
	t.Setenv("EDITOR", "nano -w")
	if got := Default(); got != "nano -w" {
		t.Fatalf("Default = %q", got)
	}
}

func TestRunPassesThePathAsOneArgument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a file.md")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(`printf edited >`, path); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "edited" {
		t.Fatalf("file = %q", b)
	}
	if err := Run("false", path); err == nil {
		t.Fatal("a failing editor is no error")
	}
}
