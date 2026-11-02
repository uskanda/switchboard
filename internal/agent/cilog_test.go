package agent

import (
	"fmt"
	"strings"
	"testing"
)

func TestTrimLogKeepsTheEnd(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 5000; i++ {
		fmt.Fprintf(&b, "2026-11-02T01:02:03.4567890Z \x1b[31mline %d\x1b[0m\n", i)
	}
	got := trimLog(b.String())
	lines := strings.Split(got, "\n")
	if len(lines) != 200 || lines[0] != "line 4801" || lines[199] != "line 5000" {
		t.Fatalf("trimLog kept %d lines, first %q, last %q", len(lines), lines[0], lines[len(lines)-1])
	}
}

func TestTrimLogCapsBytes(t *testing.T) {
	long := strings.Repeat("x", 1000)
	var b strings.Builder
	for i := 0; i < 100; i++ {
		b.WriteString(long + "\n")
	}
	if got := trimLog(b.String()); len(got) > 20*1024 {
		t.Fatalf("trimLog kept %d bytes, want at most 20KB", len(got))
	}
}

func TestTrimLogRedactsAndCleansGitLabSections(t *testing.T) {
	raw := "section_start:1700000000:step_script\r\x1b[0K\x1b[36;1m$ go test\x1b[0m\n" +
		"progress 10%\rprogress 100%\n" +
		"token ghp_abcdefghijklmnopqrstuvwxyz0123\n" +
		"section_end:1700000001:step_script\r\x1b[0K\n"
	got := trimLog(raw)
	want := "$ go test\nprogress 100%\ntoken [redacted]"
	if got != want {
		t.Fatalf("trimLog = %q, want %q", got, want)
	}
}
