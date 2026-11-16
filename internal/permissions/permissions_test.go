package permissions

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type audit struct{ events []string }

func (a *audit) Log(event string, _ map[string]any) { a.events = append(a.events, event) }

func store(t *testing.T) (Store, *audit) {
	t.Helper()
	a := &audit{}
	return For(filepath.Join(t.TempDir(), "config.yaml"), a), a
}

func TestPaths(t *testing.T) {
	s, _ := store(t)
	for sc, want := range map[Scope]string{
		{}:                                  "allow.txt",
		{Project: "gh:uskanda/smarternews"}: "gh/uskanda/smarternews/allow.txt",
		{Project: "gh:uskanda/smarternews", Issue: 217}: "gh/uskanda/smarternews/issues/217.txt",
		{Project: "gl:nas/tools/sb"}:                    "gl/nas/tools/sb/allow.txt",
	} {
		got, err := s.Path(sc)
		if err != nil || got != filepath.Join(s.Dir, want) {
			t.Errorf("Path(%+v) = %q, %v; want %q", sc, got, err, want)
		}
	}
	for _, sc := range []Scope{{Project: "gh:../x"}, {Project: "gh:o"}, {Issue: 3}, {Project: "gh:o/r", Issue: -1}} {
		if _, err := s.Path(sc); err == nil {
			t.Errorf("Path(%+v) is no error", sc)
		}
	}
	if filepath.Base(s.Dir) != "permissions" {
		t.Errorf("Dir = %q", s.Dir)
	}
}

func TestAddWritesPrivately(t *testing.T) {
	s, a := store(t)
	sc := Scope{Project: "gh:uskanda/smarternews", Issue: 217}
	if err := s.Add(sc, []string{"Bash(git ls-tree *)"}); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Path(sc)
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, %v", fi, err)
	}
	for d := filepath.Dir(p); d != filepath.Dir(s.Dir); d = filepath.Dir(d) {
		if di, err := os.Stat(d); err != nil || di.Mode().Perm() != 0o700 {
			t.Fatalf("dir %s mode = %v, %v", d, di, err)
		}
	}
	if got, _ := s.Read(sc); !slices.Equal(got, []string{"Bash(git ls-tree *)"}) {
		t.Fatalf("Read = %q", got)
	}
	if !slices.Equal(a.events, []string{"permission.saved"}) {
		t.Fatalf("audit = %q", a.events)
	}
}

func TestReadSkipsCommentsBlanksAndRepeats(t *testing.T) {
	s, _ := store(t)
	p, _ := s.Path(Scope{})
	os.MkdirAll(filepath.Dir(p), 0o700)
	os.WriteFile(p, []byte("# by hand\n\n  Skill  \nBash(git mv *)\nSkill\n"), 0o600)
	if got, err := s.Read(Scope{}); err != nil || !slices.Equal(got, []string{"Skill", "Bash(git mv *)"}) {
		t.Fatalf("Read = %q, %v", got, err)
	}
	if got, err := s.Read(Scope{Project: "gh:o/r"}); err != nil || len(got) != 0 {
		t.Fatalf("a missing file: %q, %v", got, err)
	}
}

func TestAddAndRemoveKeepWhatWasWrittenByHand(t *testing.T) {
	s, a := store(t)
	p, _ := s.Path(Scope{})
	os.MkdirAll(filepath.Dir(p), 0o700)
	os.WriteFile(p, []byte("# kept\nSkill\n"), 0o600)
	if err := s.Add(Scope{}, []string{"Skill", "Bash(git blame *)"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(Scope{}, []string{"Skill", "Bash(nothing)"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "# kept\nBash(git blame *)\n" {
		t.Fatalf("file = %q", b)
	}
	if !slices.Equal(a.events, []string{"permission.saved", "permission.removed"}) {
		t.Fatalf("audit = %q", a.events)
	}
}

func TestAddRefusesEveryRuleWhenOneIsBad(t *testing.T) {
	s, _ := store(t)
	err := s.Add(Scope{Project: "gh:o/r"}, []string{"Bash(git mv *)", "Bash(git *)"})
	re, ok := err.(*RuleError)
	if !ok || len(re.Rules) != 1 || re.Rules[0].Rule != "Bash(git *)" || re.Rules[0].Reason == "" {
		t.Fatalf("err = %#v", err)
	}
	if got, _ := s.Read(Scope{Project: "gh:o/r"}); len(got) != 0 {
		t.Fatalf("saved %q", got)
	}
}

func TestCheck(t *testing.T) {
	for _, bad := range []string{
		"", "Bash", "Bash(*)", "Bash(git push *)", "Bash(gh pr *)", "WebFetch",
		"Bash(git *)", "Bash(git:*)", "Bash(git)", "Bash(git config *)", "Bash(git -c core.pager=sh log)", "Bash(git --exec-path *)",
		"Bash(" + strings.Repeat("a", 195) + ")",
	} {
		if Check(bad) == "" {
			t.Errorf("Check(%q) passes", bad)
		}
	}
	for _, good := range []string{"Bash(git mv *)", "Bash(git ls-tree)", "Skill", "Bash(openspec *)", "Bash(mkdir *)"} {
		if why := Check(good); why != "" {
			t.Errorf("Check(%q) = %q", good, why)
		}
	}
}

func TestCandidates(t *testing.T) {
	for _, c := range []struct {
		tool, cmd string
		want      []string
		compound  bool
	}{
		{"Bash", "git mv a.sql b.sql", []string{"Bash(git mv)", "Bash(git mv *)"}, false},
		{"Bash", "git ls-tree --name-only origin/main x/", []string{"Bash(git ls-tree)", "Bash(git ls-tree *)"}, false},
		{"Bash", "mkdir -p x", []string{"Bash(mkdir)", "Bash(mkdir *)"}, false},
		{"Skill", "", []string{"Skill"}, false},
		{"Bash", "for f in a b; do git log -- $f; done", nil, true},
		{"Bash", "git log | head", nil, true},
		{"Bash", "cd x && git status", nil, true},
		{"Bash", "echo $(git rev-parse HEAD)", nil, true},
		{"Bash", "git -c core.pager=sh log", nil, false},
		{"Bash", "git config user.name x", nil, false},
		{"Bash", "git push origin main", nil, false},
		{"Bash", "FOO=1 make test", nil, false},
		{"WebFetch", "", nil, false},
	} {
		got, compound := Candidates(c.tool, c.cmd)
		if !slices.Equal(got, c.want) || compound != c.compound {
			t.Errorf("Candidates(%q, %q) = %q, %v; want %q, %v", c.tool, c.cmd, got, compound, c.want, c.compound)
		}
	}
}
