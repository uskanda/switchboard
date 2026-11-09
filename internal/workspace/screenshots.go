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

// PlanImageDir is where a plan run puts the images of its questions, relative to the worktree (ADR-068). It is
// excluded like ScreenshotDir, and its files are moved out before the check that throws a plan run's changes away.
const PlanImageDir = ".switchboard/plan-images"

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
	return s.preparePlace(ctx, ScreenshotDir)
}

// PreparePlanImages readies PlanImageDir for a plan run as PrepareScreenshots readies ScreenshotDir.
func (s *Space) PreparePlanImages(ctx context.Context) error {
	return s.preparePlace(ctx, PlanImageDir)
}

func (s *Space) preparePlace(ctx context.Context, rel string) error {
	if err := s.exclude(ctx, rel); err != nil {
		return err
	}
	if p := s.place(rel); p != "" {
		return os.RemoveAll(p)
	}
	return nil
}

// exclude adds rel, a place under the worktree, to info/exclude once.
func (s *Space) exclude(ctx context.Context, rel string) error {
	dir := s.Dir()
	p, err := s.git(ctx, dir, nil, "rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	line := "/" + rel + "/"
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

// screenshotDir is ScreenshotDir in the worktree; see place.
func (s *Space) screenshotDir() string { return s.place(ScreenshotDir) }

// place is rel in the worktree, or "" when it is missing or when it, or .switchboard, is not a directory of the
// worktree itself, such as a symbolic link a run made: nothing behind a link is read or removed.
func (s *Space) place(rel string) string {
	p := s.Dir()
	for _, part := range strings.Split(rel, "/") {
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

// MaxPlanImageFiles bounds the files moved out of PlanImageDir; the questions may use MaxScreenshots of them.
const MaxPlanImageFiles = 50

// CollectPlanImages moves the regular files directly in PlanImageDir, up to MaxScreenshotSize each and
// MaxPlanImageFiles in all, to dest, and removes PlanImageDir. The questions name the images they use; which of them
// may be posted is checked there. It returns the names moved.
func (s *Space) CollectPlanImages(dest string) ([]string, error) {
	dir := s.place(PlanImageDir)
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.Type().IsRegular() || len(names) == MaxPlanImageFiles {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return nil, err
		}
		if fi.Size() > MaxScreenshotSize {
			continue
		}
		if len(names) == 0 {
			if err := os.MkdirAll(dest, 0o700); err != nil {
				return nil, err
			}
		}
		if err := move(filepath.Join(dir, e.Name()), filepath.Join(dest, e.Name())); err != nil {
			return nil, err
		}
		names = append(names, e.Name())
	}
	return names, os.RemoveAll(dir)
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
