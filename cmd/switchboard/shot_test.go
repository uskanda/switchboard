package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// A URL that is not on this host is refused before any browser is looked for or started.
func TestShotRefusesOtherHosts(t *testing.T) {
	var out, errb bytes.Buffer
	code := runShot([]string{"--url", "https://example.com/", "--out", filepath.Join(t.TempDir(), "x.png"),
		"--browser", "/nonexistent/chrome"}, &out, &errb)
	if code == 0 || !strings.Contains(errb.String(), "not on localhost") {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
}

func TestShotNeedsURLAndOut(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runShot([]string{"--url", "http://localhost/"}, &out, &errb); code != 2 {
		t.Fatalf("code %d, want 2", code)
	}
}

func TestShotWithoutBrowser(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var out, errb bytes.Buffer
	code := runShot([]string{"--url", "http://localhost/", "--out", filepath.Join(t.TempDir(), "x.png"),
		"--config", filepath.Join(t.TempDir(), "none.yaml")}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "no browser") {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
}
