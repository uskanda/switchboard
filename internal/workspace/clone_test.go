package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneMakesAClone(t *testing.T) {
	_, origin := repo(t)
	dir := filepath.Join(t.TempDir(), "repos", "notes")
	if err := Clone(ctx, os.Environ(), origin, dir, "tok", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("the clone has no AGENTS.md: %v", err)
	}
	got, err := Origin(ctx, os.Environ(), dir)
	if err != nil || got != origin {
		t.Errorf("Origin = %q, %v; want %q", got, err, origin)
	}
}

func TestAFailedCloneLeavesNoDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "notes")
	err := Clone(ctx, os.Environ(), filepath.Join(t.TempDir(), "missing.git"), dir, "s3cret-token", "")
	if err == nil {
		t.Fatal("cloning a missing repository succeeded")
	}
	if strings.Contains(err.Error(), "s3cret-token") {
		t.Errorf("the token is in the error: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("%s is left behind (%v)", dir, err)
	}
}

func TestSameRepo(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"https://github.com/alice/notes.git", "https://github.com/alice/notes", true},
		{"https://github.com/alice/notes.git", "git@github.com:alice/notes.git", true},
		{"https://github.com/alice/notes.git", "ssh://git@github.com/Alice/Notes.git", true},
		{"https://x-access-token@github.com/alice/notes.git", "https://github.com/alice/notes.git", true},
		{"https://gl.example/nas/tools/sb.git", "git@gl.example:nas/tools/sb.git", true},
		{"https://github.com/alice/notes.git", "https://github.com/someone/notes.git", false},
		{"https://github.com/alice/notes.git", "https://gitlab.com/alice/notes.git", false},
		{"/srv/git/notes.git", "/srv/git/notes.git", true},
	} {
		if got := SameRepo(c.a, c.b); got != c.want {
			t.Errorf("SameRepo(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
