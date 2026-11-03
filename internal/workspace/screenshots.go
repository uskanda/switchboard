package workspace

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ScreenshotDir is where a run that implements an issue puts the images to attach to the issue, relative to the
// worktree (ADR-051). git ignores it through info/exclude; a commit of a file under it blocks the push.
const ScreenshotDir = ".switchboard/screenshots"

// The limits of the images of one run.
const (
	MaxScreenshotSize = 5 << 20
	MaxScreenshots    = 10
)

// Why a file of ScreenshotDir was not attached.
const (
	SkipTooLarge = "too-large" // larger than MaxScreenshotSize
	SkipTooMany  = "too-many"  // past the first MaxScreenshots images, by name
	SkipNotImage = "not-image" // not a .png, .jpg, .jpeg or .webp file
)

// Skipped is a file of ScreenshotDir left out, and why.
type Skipped struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

var imageExts = []string{".png", ".jpg", ".jpeg", ".webp"}

// PrepareScreenshots readies ScreenshotDir for a run: it adds the place to info/exclude once, and removes what an
// earlier run left there.
func (s *Space) PrepareScreenshots(ctx context.Context) error {
	if err := s.excludeScreenshots(ctx); err != nil {
		return err
	}
	return s.ClearScreenshots()
}

func (s *Space) excludeScreenshots(ctx context.Context) error {
	dir := s.Dir()
	p, err := s.git(ctx, dir, nil, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	line := "/" + ScreenshotDir + "/"
	b, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if slices.Contains(strings.Split(string(b), "\n"), line) {
		return nil
	}
	if len(b) > 0 && !strings.HasSuffix(string(b), "\n") {
		line = "\n" + line
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// screenshotDir is ScreenshotDir in the worktree, or "" when it is missing or when it, or .switchboard, is not a
// directory of the worktree itself, such as a symbolic link a run made: nothing behind a link is read or removed.
func (s *Space) screenshotDir() string {
	p := s.Dir()
	for _, part := range strings.Split(ScreenshotDir, "/") {
		p = filepath.Join(p, part)
		if fi, err := os.Lstat(p); err != nil || !fi.IsDir() {
			return ""
		}
	}
	return p
}

// ClearScreenshots removes ScreenshotDir and what is in it.
func (s *Space) ClearScreenshots() error {
	if p := s.screenshotDir(); p != "" {
		return os.RemoveAll(p)
	}
	return nil
}

// CollectScreenshots moves the images of ScreenshotDir to dest, by name: the regular files directly in it with an
// image extension, up to MaxScreenshotSize each and MaxScreenshots in all. It returns the names moved and the files
// left out; symbolic links and directories are passed over. ScreenshotDir is removed afterwards.
func (s *Space) CollectScreenshots(dest string) ([]string, []Skipped, error) {
	dir := s.screenshotDir()
	if dir == "" {
		return nil, nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var names []string
	var skipped []Skipped
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		name := e.Name()
		fi, err := e.Info()
		if err != nil {
			return nil, nil, err
		}
		switch {
		case !slices.Contains(imageExts, strings.ToLower(filepath.Ext(name))):
			skipped = append(skipped, Skipped{name, SkipNotImage})
			continue
		case fi.Size() > MaxScreenshotSize:
			skipped = append(skipped, Skipped{name, SkipTooLarge})
			continue
		case len(names) == MaxScreenshots:
			skipped = append(skipped, Skipped{name, SkipTooMany})
			continue
		}
		if len(names) == 0 {
			if err := os.MkdirAll(dest, 0o700); err != nil {
				return nil, nil, err
			}
		}
		if err := move(filepath.Join(dir, name), filepath.Join(dest, name)); err != nil {
			return nil, nil, err
		}
		names = append(names, name)
	}
	return names, skipped, os.RemoveAll(dir)
}

// move renames from to to, or copies it when the rename fails, as across file systems.
func move(from, to string) error {
	if os.Rename(from, to) == nil {
		return nil
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
