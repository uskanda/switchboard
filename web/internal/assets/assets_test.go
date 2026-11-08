package assets

import (
	"io/fs"
	"strings"
	"testing"
)

// sonner inserts its layout as a <style> at run time, which the page's CSP blocked until ADR-066 allowed inline
// styles. The layout still comes in the built stylesheet, so the toasts do not depend on that allowance: without it
// they would sit unpositioned below the page, out of sight.
func TestBuiltStylesheetCarriesTheToasterLayout(t *testing.T) {
	fsys := FS()
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		t.Skip("the front end is not built; make verify builds it before this test")
	}
	var css strings.Builder
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".css") {
			return err
		}
		b, err := fs.ReadFile(fsys, path)
		css.Write(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(css.String(), "[data-sonner-toaster]") {
		t.Fatal("no built stylesheet holds sonner's [data-sonner-toaster] rules")
	}
}
