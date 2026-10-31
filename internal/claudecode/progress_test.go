package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProgressReadsNewLinesOnly(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "progress.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "stream.jsonl")
	p := NewProgress(path)
	if err := p.Read(); err != nil || len(p.Tools) != 0 || p.Text != "" {
		t.Fatalf("before the file exists: %+v, %v", p, err)
	}
	if err := os.WriteFile(path, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := p.Read(); err != nil {
		t.Fatal(err)
	}
	if p.Tools["Read"] != 1 || p.Tools["Bash"] != 2 || len(p.Tools) != 2 {
		t.Fatalf("tools = %v", p.Tools)
	}
	if p.Text != "DESIGN.mdのバナーの記述を確かめる\n詳しくは後で" {
		t.Fatalf("text = %q", p.Text)
	}
	if s := fmt.Sprintf("%+v", *p); strings.Contains(s, "SECRET") || strings.Contains(s, "/secret/") {
		t.Fatalf("Progress keeps a tool input or output: %s", s)
	}

	// A line still being written waits for the next read; nothing is counted twice.
	next := `{"type":"assistant","message":{"content":[{"type":"text","text":"テストを直す"},{"type":"tool_use","id":"t4","name":"Edit","input":{}}]}}` + "\n"
	appendTo(t, path, next[:40])
	if err := p.Read(); err != nil || p.Tools["Edit"] != 0 || p.Text != "DESIGN.mdのバナーの記述を確かめる\n詳しくは後で" {
		t.Fatalf("after a partial line: %v, %q, %v", p.Tools, p.Text, err)
	}
	appendTo(t, path, next[40:])
	if err := p.Read(); err != nil || p.Tools["Edit"] != 1 || p.Tools["Bash"] != 2 || p.Text != "テストを直す" {
		t.Fatalf("after the line ends: %v, %q, %v", p.Tools, p.Text, err)
	}
}

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}
