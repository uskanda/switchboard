package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/cli"
)

// fakeEditor sets $EDITOR to a script that replaces the file with text, or leaves it alone when text is "". It
// returns a path the script creates when it runs.
func fakeEditor(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	p, opened := filepath.Join(dir, "ed"), filepath.Join(dir, "opened")
	script := "#!/bin/sh\ntouch '" + opened + "'\n"
	if text != "" {
		script += "printf '%s' '" + text + "' > \"$1\"\n"
	}
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", p)
	return opened
}

func TestInstructionsEditSavesTheProjectFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := writeConfig(t)
	fakeEditor(t, "Mind the API names.")
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"instructions", "edit", "gh:o/r", "--config", cfg}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	p := filepath.Join(filepath.Dir(cfg), "instructions", "gh", "o", "r", "plan.md")
	if b, err := os.ReadFile(p); err != nil || string(b) != "Mind the API names." {
		t.Fatalf("%s = %q, %v", p, b, err)
	}
	if !strings.Contains(out.String(), p) {
		t.Fatalf("output does not name the file: %q", out.String())
	}
}

func TestInstructionsEditWithoutAProjectEditsTheGlobalFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := writeConfig(t)
	fakeEditor(t, "Short questions.")
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"instructions", "edit", "--config", cfg}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if b, _ := os.ReadFile(filepath.Join(filepath.Dir(cfg), "instructions", "plan.md")); string(b) != "Short questions." {
		t.Fatalf("global file = %q", b)
	}
}

func TestInstructionsEditWithoutAChangeSavesNothing(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := writeConfig(t)
	fakeEditor(t, "")
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"instructions", "edit", "--config", cfg}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfg), "instructions", "plan.md")); !os.IsNotExist(err) {
		t.Fatal("an unchanged edit wrote the file")
	}
	if !strings.Contains(out.String(), "unchanged") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestInstructionsEditOfAnUnknownProject(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := writeConfig(t)
	opened := fakeEditor(t, "x")
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"instructions", "edit", "gh:someone/other", "--config", cfg}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "gh:someone/other") {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if _, err := os.Stat(opened); err == nil {
		t.Fatal("the editor was opened for an unknown project")
	}
}

func TestInstructionsEditRefusesASecret(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := writeConfig(t)
	fakeEditor(t, "token ghp_"+strings.Repeat("Zx9", 12))
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"instructions", "edit", "--config", cfg}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "secret") {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
}
