package state

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Writers of one name at once (runs that write last.json) each write a whole file, and leave no temporary file.
func TestWriteJSONOfOneNameAtOnce(t *testing.T) {
	d := Dir{Path: t.TempDir()}
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- d.WriteJSON("last.json", map[string]int{"run": i})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var got map[string]int
	if err := d.ReadJSON("last.json", &got); err != nil {
		t.Fatalf("last.json is not whole: %v", err)
	}
	entries, _ := os.ReadDir(d.Path)
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("files left: %v", names)
	}
	if fi, err := os.Stat(filepath.Join(d.Path, "last.json")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("last.json: %v, %v", fi, err)
	}
}
