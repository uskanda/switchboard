package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func shot(t *testing.T, dir, name string, size int) {
	t.Helper()
	p := filepath.Join(dir, ScreenshotDir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareScreenshotsExcludesThePlaceOnceAndEmptiesIt(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	shot(t, dir, "old.png", 10)
	for range 2 {
		if err := s.PrepareScreenshots(ctx); err != nil {
			t.Fatal(err)
		}
	}
	exclude := git(t, dir, "rev-parse", "--git-path", "info/exclude")
	if !filepath.IsAbs(exclude) {
		exclude = filepath.Join(dir, exclude)
	}
	b, err := os.ReadFile(exclude)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), "/"+ScreenshotDir+"/\n"); n != 1 {
		t.Fatalf("info/exclude has the line %d times:\n%s", n, b)
	}
	if _, err := os.Stat(filepath.Join(dir, ScreenshotDir, "old.png")); !os.IsNotExist(err) {
		t.Fatalf("the earlier run's file is still there: %v", err)
	}
	shot(t, dir, "new.png", 10)
	if st := git(t, dir, "status", "--porcelain"); st != "" {
		t.Fatalf("git sees the place: %q", st)
	}
}

func TestPrepareScreenshotsLeavesALinkedPlaceAlone(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	keep := filepath.Join(outside, "screenshots", "keep.png")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, ".switchboard")); err != nil {
		t.Fatal(err)
	}
	if err := s.PrepareScreenshots(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("a file behind a link was removed: %v", err)
	}
	got, skipped, err := s.CollectScreenshots(t.TempDir())
	if err != nil || len(got) != 0 || len(skipped) != 0 {
		t.Fatalf("CollectScreenshots behind a link = %v, %v, %v", got, skipped, err)
	}
}

func TestCollectScreenshots(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PrepareScreenshots(ctx); err != nil {
		t.Fatal(err)
	}
	shot(t, dir, "a.png", 3<<20)
	shot(t, dir, "b.png", MaxScreenshotSize+1)
	shot(t, dir, "c.JPG", MaxScreenshotSize)
	shot(t, dir, "notes.txt", 5)
	shot(t, dir, "sub/d.png", 5)
	for i := range 9 {
		shot(t, dir, fmt.Sprintf("e%d.webp", i), 5)
	}
	if err := os.Symlink(filepath.Join(dir, "AGENTS.md"), filepath.Join(dir, ScreenshotDir, "link.png")); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "screenshots")
	got, skipped, err := s.CollectScreenshots(dest)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.png", "c.JPG", "e0.webp", "e1.webp", "e2.webp", "e3.webp", "e4.webp", "e5.webp", "e6.webp", "e7.webp"}
	if !slices.Equal(got, want) {
		t.Fatalf("collected = %v, want %v", got, want)
	}
	wantSkipped := []Skipped{{"b.png", SkipTooLarge}, {"e8.webp", SkipTooMany}, {"notes.txt", SkipNotImage}}
	if !slices.Equal(skipped, wantSkipped) {
		t.Fatalf("skipped = %v, want %v", skipped, wantSkipped)
	}
	for _, name := range want {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Errorf("%s was not moved: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ScreenshotDir)); !os.IsNotExist(err) {
		t.Fatalf("the place is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatalf("the file a link pointed to is gone: %v", err)
	}
}

func TestCollectScreenshotsWithoutThePlace(t *testing.T) {
	s, _ := space(t)
	if _, err := s.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	got, skipped, err := s.CollectScreenshots(t.TempDir())
	if err != nil || got != nil || skipped != nil {
		t.Fatalf("CollectScreenshots = %v, %v, %v", got, skipped, err)
	}
}

func TestCheckOutputRefusesACommittedScreenshot(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, dir, "a.txt", "a")
	commit(t, dir, ScreenshotDir+"/a.png", "png")
	problems, err := s.CheckOutput(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], ScreenshotDir+"/a.png") {
		t.Fatalf("problems = %v", problems)
	}
}

func planImage(t *testing.T, dir, name string, size int) {
	t.Helper()
	p := filepath.Join(dir, PlanImageDir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPreparePlanImagesExcludesAndEmpties(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	planImage(t, dir, "old.png", 10)
	if err := s.PreparePlanImages(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, PlanImageDir, "old.png")); !os.IsNotExist(err) {
		t.Fatalf("the earlier run's image is still there: %v", err)
	}
	planImage(t, dir, "now.png", 10)
	if st := git(t, dir, "status", "--porcelain", "--untracked-files=all"); st != "" {
		t.Fatalf("git sees the place: %q", st)
	}
}

func TestCollectPlanImages(t *testing.T) {
	s, _ := space(t)
	dir, err := s.Prepare(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PreparePlanImages(ctx); err != nil {
		t.Fatal(err)
	}
	planImage(t, dir, "a.png", 10)
	planImage(t, dir, "layout.svg", 10)
	planImage(t, dir, "huge.png", MaxScreenshotSize+1)
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, PlanImageDir, "link.png")); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "plan-images")
	names, err := s.CollectPlanImages(dest)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	if want := []string{"a.png", "layout.svg"}; !slices.Equal(names, want) {
		t.Fatalf("moved %v, want %v", names, want)
	}
	if _, err := os.Stat(filepath.Join(dir, PlanImageDir)); !os.IsNotExist(err) {
		t.Fatalf("the place is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a.png")); err != nil {
		t.Fatal(err)
	}
}
