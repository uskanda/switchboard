package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeRefusesEveryInterface(t *testing.T) {
	p := filepath.Join(t.TempDir(), "web.yaml")
	os.WriteFile(p, []byte("listen: 0.0.0.0:8783\n"), 0o600)
	var out, errOut bytes.Buffer
	if code := run([]string{"serve", "--config", p}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "every interface") {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
}

func TestUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage: switchboard-web serve") {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
	if code := run([]string{"serve", "extra"}, &out, &errOut); code != 2 {
		t.Fatalf("extra argument: code = %d", code)
	}
}
