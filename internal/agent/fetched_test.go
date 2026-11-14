package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fetches is a step fragment that prints an assistant message calling WebFetch on each URL, as claude does.
func fetches(urls ...string) string {
	var out strings.Builder
	for i, u := range urls {
		line, _ := json.Marshal(map[string]any{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "id": fmt.Sprintf("f%d", i), "name": "WebFetch", "input": map[string]any{"url": u, "prompt": "read"}},
		}}})
		out.WriteString("cat <<'JSON'\n" + string(line) + "\nJSON\n")
	}
	return out.String()
}

func TestFetchedList(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("a", 250)
	var many []string
	for i := range 23 {
		many = append(many, fmt.Sprintf("https://example.com/%d", i))
	}
	cases := []struct {
		name    string
		urls    []string
		want    []string
		notWant []string
	}{
		{"two", []string{"https://example.com/a", "https://example.com/b"},
			[]string{"- `https://example.com/a`\n- `https://example.com/b`"}, nil},
		{"long", []string{long}, []string{"- `" + long[:200] + "…`"}, []string{long[:201]}},
		{"characters", []string{"https://example.com/" + strings.Repeat("あ", 190)},
			[]string{"- `https://example.com/" + strings.Repeat("あ", 180) + "…`"}, nil},
		{"backtick", []string{"https://example.com/`x`"}, []string{"- ``https://example.com/`x` ``"}, nil},
		{"many", many, []string{"- `https://example.com/19`", "3 more"}, []string{"https://example.com/20`"}},
	}
	for _, c := range cases {
		got := fetchedList(c.urls, "%d more")
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: the list lacks %q:\n%s", c.name, w, got)
			}
		}
		for _, w := range c.notWant {
			if strings.Contains(got, w) {
				t.Errorf("%s: the list has %q:\n%s", c.name, w, got)
			}
		}
	}
}

// The plan's comment lists the pages the run read, folded away, before the refused operations.
func TestThePlanCommentListsThePagesRead(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, fetches("https://example.com/a", "https://example.com/b", "https://example.com/a")+
		result(map[string]any{"status": "plan", "title": "T", "plan": "The plan."},
			map[string]any{"tool_name": "Bash", "tool_use_id": "b1", "tool_input": map[string]any{"command": "make deploy"}}))
	p := x.startPlan()
	i, j := strings.Index(p.Body, "<summary>Pages read</summary>"), strings.Index(p.Body, "<summary>Refused operations</summary>")
	if i < 0 || j < i {
		t.Fatalf("the pages read are not listed before the refused operations:\n%s", p.Body)
	}
	if strings.Count(p.Body, "https://example.com/a") != 1 || !strings.Contains(p.Body, "- `https://example.com/a`\n- `https://example.com/b`") {
		t.Errorf("the pages are not listed once each, in order:\n%s", p.Body)
	}
}

func TestTheQuestionsCommentListsThePagesRead(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, fetches("https://example.com/q")+questions([3]string{"Default", "Which default?", "A."}))
	if p := x.startPlan(); !strings.Contains(p.Body, "Pages read") || !strings.Contains(p.Body, "https://example.com/q") {
		t.Errorf("the questions do not list the page read:\n%s", p.Body)
	}
}

func TestTheSkippedPlanListsThePagesRead(t *testing.T) {
	x := newH(t, "", autoProject)
	x.step(1, fetches("https://example.com/s")+implementDecl("The plan."))
	x.step(2, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	x.startReady()
	if _, body, _ := x.skipComment(); !strings.Contains(body, "Pages read") || !strings.Contains(body, "https://example.com/s") {
		t.Errorf("the skipped plan does not list the page read:\n%s", body)
	}
}

func TestAPlanWithoutPagesListsNone(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, planDecl("The plan."))
	if p := x.startPlan(); strings.Contains(p.Body, "Pages read") {
		t.Errorf("the plan lists pages it did not read:\n%s", p.Body)
	}
}
