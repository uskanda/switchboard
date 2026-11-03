package service

import (
	"path/filepath"
	"strings"
)

// GoRunBinary reports whether bin is a binary go run built: one in the build cache goCache, or in the exe directory
// of a go-build* work directory. A unit that runs such a binary keeps the version it was installed with, or breaks
// once the cache is cleaned. go test's binary sits in a go-build* directory too, but not under exe.
func GoRunBinary(bin, goCache string) bool {
	if goCache != "" {
		if rel, err := filepath.Rel(goCache, bin); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			return true
		}
	}
	dir := filepath.Dir(bin)
	if filepath.Base(dir) != "exe" {
		return false
	}
	for d := filepath.Dir(dir); d != filepath.Dir(d); d = filepath.Dir(d) {
		if strings.HasPrefix(filepath.Base(d), "go-build") {
			return true
		}
	}
	return false
}
