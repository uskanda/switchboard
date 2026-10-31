package agent

import (
	"strings"
	"testing"
	"time"
)

func TestProgressLineShape(t *testing.T) {
	start := time.Date(2026, 10, 31, 13, 34, 0, 0, time.UTC)
	now := start.Add(7*time.Minute + 20*time.Second)
	long := strings.Repeat("あ", 130)
	l := &progressLog{msg: japanese, started: start}
	if !l.add(now, map[string]int{"Edit": 5, "Bash": 12}, "DESIGN.mdのバナーの記述を、実装に合わせて直している\n次の行は使わない") {
		t.Fatal("the first line was not added")
	}
	if got, want := l.lines[0], "13:41 7分 Bash 12・Edit 5 ｜ DESIGN.mdのバナーの記述を、実装に合わせて直している"; got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	// The same tools and purpose a minute later add nothing.
	if l.add(now.Add(time.Minute), map[string]int{"Edit": 5, "Bash": 12}, "DESIGN.mdのバナーの記述を、実装に合わせて直している") || len(l.lines) != 1 {
		t.Fatalf("an unchanged minute added a line: %q", l.lines)
	}
	if !l.add(start.Add(75*time.Minute), map[string]int{"Edit": 6, "Bash": 12}, "  "+long) {
		t.Fatal("a changed count added nothing")
	}
	got := l.lines[1]
	if !strings.HasPrefix(got, "14:49 1時間15分 Bash 12・Edit 6 ｜ ") || len([]rune(strings.TrimPrefix(got, "14:49 1時間15分 Bash 12・Edit 6 ｜ "))) != 120 {
		t.Fatalf("line = %q", got)
	}

	en := &progressLog{msg: english, started: start}
	en.add(start.Add(30*time.Second), map[string]int{}, "")
	if en.lines[0] != "13:34 0m no tools | ?" {
		t.Fatalf("english line = %q", en.lines[0])
	}
}
