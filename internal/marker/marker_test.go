package marker

import (
	"strings"
	"testing"
	"time"
)

var at = time.Date(2026, 10, 29, 5, 0, 0, 0, time.UTC)

func TestFormatAndParseRoundTrip(t *testing.T) {
	m := Marker{Host: "ubuntu-a", Session: "s-1", Run: "r-1", State: "waiting", Reason: "question", Ack: 42, At: at}
	line := m.Format()
	if !strings.HasPrefix(line, "<!-- switchboard v=1 ") || !strings.HasSuffix(line, " -->") {
		t.Fatalf("Format = %q", line)
	}
	got, ok := Parse("Question?\n\n" + line + "\n")
	if !ok {
		t.Fatalf("Parse failed on %q", line)
	}
	if got.V != 1 || got.Host != "ubuntu-a" || got.Ack != 42 || got.State != "waiting" || !got.At.Equal(at) {
		t.Fatalf("Parse = %+v", got)
	}
}

func TestParseNeedsTheMarkerOnTheLastLine(t *testing.T) {
	m := Marker{Host: "h", State: "waiting", At: at}
	if _, ok := Parse(m.Format() + "\nmore text"); ok {
		t.Fatal("a marker followed by text must not count")
	}
}

func TestParseIgnoresQuotedMarkers(t *testing.T) {
	m := Marker{Host: "h", State: "waiting", At: at}
	if _, ok := Parse("I agree\n> " + m.Format()); ok {
		t.Fatal("a quoted marker must not count")
	}
}

func TestParseIgnoresUnknownKeysAndKeepsNewerVersions(t *testing.T) {
	got, ok := Parse("<!-- switchboard v=2 host=h state=done shiny=yes at=2026-10-29T05:00:00Z -->")
	if !ok || got.V != 2 || got.Host != "h" || got.State != "done" {
		t.Fatalf("Parse = %+v, %v", got, ok)
	}
}

func TestFormatReplacesSpacesInValues(t *testing.T) {
	m := Marker{Host: "my host", State: "waiting", At: at}
	got, ok := Parse(m.Format())
	if !ok || got.Host != "my_host" {
		t.Fatalf("Parse = %+v", got)
	}
}

func TestProgressMarker(t *testing.T) {
	m, ok := Parse("progress\n\n" + Marker{Host: "desk", Run: "r-1", State: Progress, At: at}.Format())
	if !ok || m.State != "progress" || m.Run != "r-1" || !m.Transient() {
		t.Fatalf("Parse = %+v, %v", m, ok)
	}
	for state, want := range map[string]bool{"running": true, "progress": true, "waiting": false, "done": false} {
		if got := (&Marker{State: state}).Transient(); got != want {
			t.Errorf("Transient(%s) = %v", state, got)
		}
	}
}
