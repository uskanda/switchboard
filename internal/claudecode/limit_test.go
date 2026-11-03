package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/usage"
)

func parseFixture(t *testing.T, name string) *Result {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	res := &Result{}
	if err := Parse(f, res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestParseReadsTheUsageLimitAndItsResetTime(t *testing.T) {
	res := parseFixture(t, "limit.jsonl")
	if res.Limit == nil {
		t.Fatalf("Limit = nil, want the usage limit: %+v", res)
	}
	if res.Limit.Kind != usage.KindUsage {
		t.Errorf("Kind = %q, want %q", res.Limit.Kind, usage.KindUsage)
	}
	want := time.Unix(1793644200, 0)
	if res.Limit.ResetsAt == nil || !res.Limit.ResetsAt.Equal(want) {
		t.Errorf("ResetsAt = %v, want %v", res.Limit.ResetsAt, want)
	}
	if !strings.Contains(res.Limit.Message, "5-hour limit") {
		t.Errorf("Message = %q", res.Limit.Message)
	}
}

func TestParseTakesA429WithoutAnEventAsALimitWithoutResetTime(t *testing.T) {
	res := parseFixture(t, "limit-429.jsonl")
	if res.Limit == nil || res.Limit.ResetsAt != nil || res.Limit.Kind != usage.KindRate {
		t.Fatalf("Limit = %+v, want a rate limit without a reset time", res.Limit)
	}
	if res.CostUSD != 0.25 {
		t.Errorf("CostUSD = %v, want 0.25", res.CostUSD)
	}
}

func TestParseIgnoresARejectedEventWhenTheRunSucceeded(t *testing.T) {
	in := `{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","resetsAt":1793644200}}
{"type":"result","subtype":"success","is_error":false,"num_turns":2,"total_cost_usd":1.5,"structured_output":{"status":"done","summary":"s"}}
`
	res := &Result{}
	if err := Parse(strings.NewReader(in), res); err != nil {
		t.Fatal(err)
	}
	if res.Limit != nil {
		t.Errorf("Limit = %+v, want nil for a run that succeeded", res.Limit)
	}
	if res.CostUSD != 1.5 {
		t.Errorf("CostUSD = %v, want 1.5", res.CostUSD)
	}
}

func TestParseIgnoresAnAllowedWarning(t *testing.T) {
	in := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed_warning","resetsAt":1793644200,"utilization":0.92}}
{"type":"result","subtype":"error_max_turns","is_error":true,"num_turns":80}
`
	res := &Result{}
	if err := Parse(strings.NewReader(in), res); err != nil {
		t.Fatal(err)
	}
	if res.Limit != nil {
		t.Errorf("Limit = %+v, want nil: an allowed warning and max turns are no limit", res.Limit)
	}
}
