package marker

import (
	"strings"
	"testing"
	"time"
)

func TestLimitQueuedMarkerCarriesItsRunAndUntil(t *testing.T) {
	until := time.Date(2026, 11, 3, 18, 30, 0, 0, time.UTC)
	m := Marker{Host: "h", Run: "r1", State: Queued, Reason: QueuedLimit, At: at, Until: until}
	line := m.Format()
	if !strings.Contains(line, "state=queued reason=limit") || !strings.Contains(line, "until=2026-11-03T18:30:00Z") {
		t.Fatalf("Format = %q", line)
	}
	got, ok := Parse(line)
	if !ok || got.Run != "r1" || got.Reason != QueuedLimit || !got.Until.Equal(until) {
		t.Fatalf("Parse = %+v", got)
	}
	if QueuedQuota != "quota" {
		t.Errorf("QueuedQuota = %q", QueuedQuota)
	}
	if strings.Contains(Marker{Host: "h", State: Queued, At: at}.Format(), "until=") {
		t.Error("a marker without Until writes until=")
	}
}
