package service

import "testing"

// A binary go run built lives in the build cache or in a go-build*/…/exe directory, and disappears or goes stale
// once the cache is cleaned or the source changes. go test's own binary and an installed binary are not such.
func TestGoRunBinary(t *testing.T) {
	const cache = "/home/kanda/.cache/go-build"
	for _, c := range []struct {
		bin  string
		want bool
	}{
		{"/home/kanda/.cache/go-build/e0/e0e15bbc-d/switchboard", true},
		{"/tmp/go-build3129/b001/exe/switchboard", true},
		{"/tmp/go-build3129/b001/switchboard.test", false},
		{"/home/kanda/.local/bin/switchboard", false},
		{"/home/kanda/.cache/go-builder/switchboard", false},
		{"/home/kanda/src/exe/switchboard", false},
	} {
		if got := GoRunBinary(c.bin, cache); got != c.want {
			t.Errorf("GoRunBinary(%q) = %v, want %v", c.bin, got, c.want)
		}
	}
	if GoRunBinary("/home/kanda/.cache/go-build/e0/x/switchboard", "") {
		t.Error("an empty cache directory matched")
	}
}
