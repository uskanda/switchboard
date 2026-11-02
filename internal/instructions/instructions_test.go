package instructions

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type audit struct{ events []map[string]any }

func (a *audit) Log(event string, fields map[string]any) {
	rec := map[string]any{"event": event}
	for k, v := range fields {
		rec[k] = v
	}
	a.events = append(a.events, rec)
}

func store(t *testing.T) (Store, *audit) {
	t.Helper()
	a := &audit{}
	return Store{Dir: filepath.Join(t.TempDir(), "instructions"), Audit: a}, a
}

func TestDirSitsNextToTheConfiguration(t *testing.T) {
	if got := For("/home/alice/.config/switchboard/config.yaml", nil).Dir; got != "/home/alice/.config/switchboard/instructions" {
		t.Fatalf("Dir = %q", got)
	}
}

func TestPaths(t *testing.T) {
	s := Store{Dir: "/c/instructions"}
	for target, want := range map[string]string{
		Global:                         "/c/instructions/plan.md",
		"gh:alice/sandbox":             "/c/instructions/gh/alice/sandbox/plan.md",
		"gl:nas/tools/switchboard-sbx": "/c/instructions/gl/nas/tools/switchboard-sbx/plan.md",
	} {
		got, err := s.Path(target)
		if err != nil || got != want {
			t.Errorf("Path(%q) = %q, %v; want %q", target, got, err, want)
		}
	}
	for _, bad := range []string{"gh:../x", "gh:alice", "bitbucket:a/b", "gl:a/../b"} {
		if _, err := s.Path(bad); err == nil {
			t.Errorf("Path(%q) is accepted", bad)
		}
	}
}

func TestSaveWritesPrivatelyAndRecordsTheHashOnly(t *testing.T) {
	s, a := store(t)
	text := "Ask about the API names first.\n"
	if err := s.Save("gh:alice/sandbox", text); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Path("gh:alice/sandbox")
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, %v", fi, err)
	}
	for d := filepath.Dir(p); d != filepath.Dir(s.Dir); d = filepath.Dir(d) {
		if fi, err := os.Stat(d); err != nil || fi.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %v, %v", d, fi.Mode().Perm(), err)
		}
	}
	if got, _ := s.Read("gh:alice/sandbox"); got != text {
		t.Fatalf("Read = %q", got)
	}
	sum := sha256.Sum256([]byte(text))
	if len(a.events) != 1 || a.events[0]["event"] != "instructions.saved" || a.events[0]["target"] != "gh:alice/sandbox" || a.events[0]["sha256"] != hex.EncodeToString(sum[:]) {
		t.Fatalf("audit = %v", a.events)
	}
	for _, v := range a.events[0] {
		if s, ok := v.(string); ok && strings.Contains(s, "API names") {
			t.Fatal("the audit log holds the instructions")
		}
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(p), ".*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

func TestSaveRefusesTooLargeAndSecrets(t *testing.T) {
	s, a := store(t)
	if err := s.Save(Global, "old\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(Global, strings.Repeat("x", MaxSize+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Save of a large text = %v", err)
	}
	if err := s.Save(Global, strings.Repeat("x", MaxSize)); err != nil {
		t.Fatalf("Save of exactly the limit = %v", err)
	}
	if err := s.Save(Global, "use ghp_"+strings.Repeat("Zx9", 12)); !errors.Is(err, ErrSecret) {
		t.Fatalf("Save of a token = %v", err)
	}
	if got, _ := s.Read(Global); got != strings.Repeat("x", MaxSize) {
		t.Fatal("a refused save changed the file")
	}
	if len(a.events) != 2 {
		t.Fatalf("audit = %d events, want the two saves", len(a.events))
	}
}

func TestPlanJoinsGlobalThenProject(t *testing.T) {
	s, _ := store(t)
	if got := s.Plan("gh:alice/sandbox"); got != "" {
		t.Fatalf("Plan without files = %q", got)
	}
	if err := s.Save("gh:alice/sandbox", "Project rule.\n"); err != nil {
		t.Fatal(err)
	}
	if got := s.Plan("gh:alice/sandbox"); got != "Project rule." {
		t.Fatalf("Plan = %q", got)
	}
	if err := s.Save(Global, "  \n"); err != nil {
		t.Fatal(err)
	}
	if got := s.Plan("gh:alice/sandbox"); got != "Project rule." {
		t.Fatalf("a blank global file counts: %q", got)
	}
	if err := s.Save(Global, "Global rule.\n"); err != nil {
		t.Fatal(err)
	}
	if got := s.Plan("gh:alice/sandbox"); got != "Global rule.\n\nProject rule." {
		t.Fatalf("Plan = %q", got)
	}
	if got := s.Plan("gh:alice/other"); got != "Global rule." {
		t.Fatalf("Plan of another project = %q", got)
	}
}

func TestRunInstructionsLiveBesideThePlanOnes(t *testing.T) {
	s := Store{Dir: "/c/instructions"}.Of(Run)
	for target, want := range map[string]string{
		Global:                         "/c/instructions/run.md",
		"gh:uskanda/switchboard":       "/c/instructions/gh/uskanda/switchboard/run.md",
		"gl:nas/tools/switchboard-sbx": "/c/instructions/gl/nas/tools/switchboard-sbx/run.md",
	} {
		got, err := s.Path(target)
		if err != nil || got != want {
			t.Errorf("run Path(%q) = %q, %v; want %q", target, got, err, want)
		}
	}
	if got, _ := (Store{Dir: "/c/instructions"}).Of(Plan).Path(Global); got != "/c/instructions/plan.md" {
		t.Errorf("plan Path = %q", got)
	}
}

func TestRunInstructionsAreSavedAndJoinedApart(t *testing.T) {
	s, a := store(t)
	run := s.Of(Run)
	if err := run.Save("gh:alice/sandbox", "Record an OpenSpec change.\n"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("gh:alice/sandbox", "Ask first.\n"); err != nil {
		t.Fatal(err)
	}
	if err := run.Save(Global, "Run the checks.\n"); err != nil {
		t.Fatal(err)
	}
	if got := s.Run("gh:alice/sandbox"); got != "Run the checks.\n\nRecord an OpenSpec change." {
		t.Errorf("Run = %q", got)
	}
	if got := s.Plan("gh:alice/sandbox"); got != "Ask first." {
		t.Errorf("Plan = %q, want the plan file only", got)
	}
	p, _ := run.Path("gh:alice/sandbox")
	if fi, err := os.Stat(p); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("run file mode = %v, %v", fi, err)
	}
	kinds := []any{}
	for _, e := range a.events {
		kinds = append(kinds, e["kind"])
	}
	if len(kinds) != 3 || kinds[0] != "run" || kinds[1] != "plan" || kinds[2] != "run" {
		t.Errorf("audit kinds = %v, want run, plan, run", kinds)
	}
}

func TestRunInstructionsRefuseSecrets(t *testing.T) {
	s, _ := store(t)
	run := s.Of(Run)
	if err := run.Save(Global, "old\n"); err != nil {
		t.Fatal(err)
	}
	if err := run.Save(Global, "token glpat-"+strings.Repeat("Ab1", 8)); !errors.Is(err, ErrSecret) {
		t.Fatalf("Save of a token = %v", err)
	}
	if got, _ := run.Read(Global); got != "old\n" {
		t.Fatalf("a refused save changed the file: %q", got)
	}
}

func TestParseKind(t *testing.T) {
	for in, want := range map[string]Kind{"": Plan, "plan": Plan, "run": Run} {
		if got, err := ParseKind(in); err != nil || got != want {
			t.Errorf("ParseKind(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParseKind("build"); err == nil {
		t.Error("ParseKind(build) is accepted")
	}
}
