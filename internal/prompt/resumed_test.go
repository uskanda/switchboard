package prompt

import (
	"strings"
	"testing"
)

func TestResumedRunGetsTheLimitNoteBeforeItsTask(t *testing.T) {
	in := input()
	if strings.Contains(Build(in), ResumedNote) {
		t.Fatal("a run that is not resumed got the limit note")
	}
	in.Resumed = true
	p := Build(in)
	note, task := strings.Index(p, ResumedNote), strings.Index(p, doTheWork)
	if note < 0 || task < note {
		t.Fatalf("the note is missing or after the task:\n%s", p)
	}
}
