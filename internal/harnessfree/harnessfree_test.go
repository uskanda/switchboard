// Package harnessfree holds the test that keeps the product independent of any personal development harness
// (ADR-021): the product code and the default configuration must not mention it.
package harnessfree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// banned is assembled at run time so that this file does not match itself.
var banned = "us" + "kn"

// productRoots are the paths, relative to the repository root, that make up the product.
var productRoots = []string{"cmd", "internal", "config", "Makefile", "go.mod", ".github"}

func TestProductDoesNotMentionTheHarness(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	self, err := filepath.Abs("harnessfree_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range productRoots {
		start := filepath.Join(root, rel)
		if _, err := os.Stat(start); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(start, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || path == self {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(strings.ToLower(string(data)), banned) {
				t.Errorf("%s mentions the development harness", strings.TrimPrefix(path, root+string(filepath.Separator)))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
