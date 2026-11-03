package github

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

// fakeGH writes a gh that records its arguments, environment and standard input under dir, and prints out.
func fakeGH(t *testing.T, dir, out string, code int) string {
	t.Helper()
	p := filepath.Join(dir, "gh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + dir + "/args'\nenv > '" + dir + "/env'\ncat > '" + dir + "/stdin'\n" +
		"ls -A \"$GH_CONFIG_DIR\" > '" + dir + "/config'\nprintf '%s\\n' '" + out + "'\nexit " + string(rune('0'+code)) + "\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(b), "\n"), "\n")
}

var ref = forge.IssueRef{Forge: forge.GitHub, Owner: "o", Repo: "r", Number: 12}

func TestAttachCommentRunsGHWithTheTokenAndNothingElse(t *testing.T) {
	s, c := newServer(t)
	dir := t.TempDir()
	c.gh = fakeGH(t, dir, "https://github.com/o/r/issues/12#issuecomment-77", 0)
	const body = "### Screenshots\n\n<!-- switchboard v=1 state=attachment -->\n"
	s.on("GET /repos/o/r/issues/comments/77", jsonReply(map[string]any{
		"id": 77, "body": "### Screenshots\n\n<!-- switchboard v=1 state=attachment -->\n\n![a](https://github.com/user-attachments/assets/1)",
		"user": map[string]any{"login": "agent", "id": 2},
	}))
	var edited string
	s.on("PATCH /repos/o/r/issues/comments/77", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Body string }
		s.mu.Lock()
		_ = json.Unmarshal([]byte(s.bodies[len(s.bodies)-1]), &in)
		s.mu.Unlock()
		edited = in.Body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	t.Setenv("SECRET_OF_THE_HOST", "x")
	shot := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(shot, []byte("img"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := c.AttachComment(ctx, ref, body, []string{shot})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 77 {
		t.Fatalf("comment = %+v", got)
	}
	want := []string{"issue", "comment", "12", "-R", "o/r", "--body-file", "-", "--attach", shot}
	if args := lines(t, filepath.Join(dir, "args")); !slices.Equal(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "stdin")); string(b) != body {
		t.Fatalf("stdin = %q", b)
	}
	env := lines(t, filepath.Join(dir, "env"))
	if !slices.Contains(env, "GH_TOKEN=tok") || !slices.Contains(env, "GH_PROMPT_DISABLED=1") {
		t.Errorf("env = %q", env)
	}
	for _, e := range env {
		if strings.HasPrefix(e, "SECRET_OF_THE_HOST=") || strings.HasPrefix(e, "GITHUB_TOKEN=") {
			t.Errorf("gh got %s", e)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "config")); strings.TrimSpace(string(b)) != "" {
		t.Errorf("GH_CONFIG_DIR is not empty: %q", b)
	}
	if !strings.Contains(edited, "assets/1") || !strings.HasSuffix(edited, "\n\n<!-- switchboard v=1 state=attachment -->\n") {
		t.Errorf("the marker was not moved back to the end: %s", edited)
	}
}

func TestAttachCommentReportsGHFailing(t *testing.T) {
	_, c := newServer(t)
	c.gh = fakeGH(t, t.TempDir(), "unknown flag: --attach", 1)
	if _, err := c.AttachComment(ctx, ref, "x\n<!-- m -->", []string{"a.png"}); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("err = %v", err)
	}
	c.gh = filepath.Join(t.TempDir(), "missing-gh")
	if _, err := c.AttachComment(ctx, ref, "x", nil); err == nil {
		t.Fatal("a missing gh did not fail")
	}
}

func TestGHVersion(t *testing.T) {
	for _, c := range [][2]string{
		{"gh version 2.99.0 (2026-09-01)\nhttps://github.com/cli/cli/releases/tag/v2.99.0\n", "2.99.0"},
		{"gh version 2.40.1 (2023-12-13)\n", "2.40.1"},
		{"something else", ""},
	} {
		if got := parseGHVersion(c[0]); got != c[1] {
			t.Errorf("parseGHVersion(%q) = %q, want %q", c[0], got, c[1])
		}
	}
	for v, ok := range map[string]bool{"2.99.0": true, "2.100.1": true, "3.0.0": true, "2.98.9": false, "2.40.1": false, "": false} {
		if got := GHAttaches(v); got != ok {
			t.Errorf("GHAttaches(%q) = %v", v, got)
		}
	}
}
