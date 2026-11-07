package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// commented is a configuration as a person writes it: aligned comments, blank lines and a commented-out project
// after the last one.
const commented = `version: 1

host:
  name: ubuntu-a                      # this machine

forges:
  github:
    token_ref: switchboard/github     # agent token
    inbox_token_ref: switchboard/github-inbox
    agent_identity: alice

trusted_authors:                      # accounts whose comments are instructions
  github: [alice]

inbox:
  project_root: ~/repos

projects:
  - ref: gh:alice/sandbox
    path: ~/repos/sandbox
    host: ubuntu-a
    protected_paths_extra: []
    plan_first: false                 # true: plan first
  # - ref: gl:group/other              # a commented-out project
  #   path: ~/repos/other

notify:
  web_url: http://192.0.2.1:8783
`

var notes = NewProject{Ref: "gh:alice/notes", Path: "~/repos/notes", Host: "ubuntu-a", BaseBranch: "main", PlanFirst: PlanAuto}

func TestAddProjectKeepsTheRestOfTheFile(t *testing.T) {
	p := write(t, commented)
	e, err := OpenEditor(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AddProject(notes); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	want := strings.Replace(commented, "    plan_first: false                 # true: plan first\n",
		"    plan_first: false                 # true: plan first\n"+
			"  - ref: gh:alice/notes\n"+
			"    path: ~/repos/notes\n"+
			"    host: ubuntu-a\n"+
			"    base_branch: main\n"+
			"    plan_first: auto\n", 1)
	if string(got) != want {
		t.Errorf("file after adding:\n%s\nwant:\n%s", got, want)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(c.Projects); n != 2 || c.Projects[1].Ref != "gh:alice/notes" || c.Projects[1].PlanFirst != PlanAuto {
		t.Errorf("projects = %+v, want sandbox then notes with plan_first", c.Projects)
	}
}

func TestAddProjectFollowsTheIndentOfTheList(t *testing.T) {
	body := strings.Replace(valid, "projects:\n  - ref: gh:alice/sandbox\n    path: ~/repos/sandbox\n    host: ubuntu-a\n  - ref: gh:alice/other\n    path: /srv/other\n    host: ubuntu-b\n    base_branch: develop\n    verify_command: make test\n",
		"projects:\n- ref: gh:alice/sandbox\n  path: ~/repos/sandbox\n  host: ubuntu-a\n", 1)
	p := write(t, body)
	e, err := OpenEditor(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AddProject(notes); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if !strings.HasSuffix(string(got), "  host: ubuntu-a\n- ref: gh:alice/notes\n  path: ~/repos/notes\n  host: ubuntu-a\n  base_branch: main\n  plan_first: auto\n") {
		t.Errorf("file after adding:\n%s", got)
	}
}

func TestAddProjectAfterABlockScalar(t *testing.T) {
	// The last project ends in a multi-line verify_command.
	body := strings.Replace(commented, "    protected_paths_extra: []\n", "    verify_command: |\n      make test\n      make lint\n", 1)
	p := write(t, body)
	e, err := OpenEditor(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AddProject(notes); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Projects[0].VerifyCommand != "make test\nmake lint\n" || len(c.Projects) != 2 {
		t.Errorf("projects = %+v, want the verify_command kept and notes added", c.Projects)
	}
}

func TestAddProjectWritesThroughASymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "dotfiles", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(real), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(real, []byte(commented), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	e, err := OpenEditor(link)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AddProject(notes); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is no longer a symlink (%v)", link, err)
	}
	got, _ := os.ReadFile(real)
	if !strings.Contains(string(got), "gh:alice/notes") {
		t.Errorf("the link's target was not written:\n%s", got)
	}
	if fi, _ := os.Stat(real); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want the original 0640", fi.Mode().Perm())
	}
}

func TestAddProjectKeepsABackup(t *testing.T) {
	p := write(t, commented)
	e, err := OpenEditor(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AddProject(notes); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(p + ".bak")
	if err != nil || string(bak) != commented {
		t.Errorf("backup = %q (%v), want the file before adding", bak, err)
	}
}

func TestAddProjectThatFailsValidationWritesNothing(t *testing.T) {
	p := write(t, commented)
	e, err := OpenEditor(p)
	if err != nil {
		t.Fatal(err)
	}
	// No forges.gitlab: a GitLab project makes the configuration invalid.
	err = e.AddProject(NewProject{Ref: "gl:group/tools", Path: "~/repos/tools", Host: "ubuntu-a", BaseBranch: "main"})
	if err == nil || !strings.Contains(err.Error(), "forges.gitlab") {
		t.Fatalf("err = %v, want the missing forges.gitlab reported", err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != commented {
		t.Errorf("the file changed:\n%s", got)
	}
	if _, err := os.Stat(p + ".bak"); !os.IsNotExist(err) {
		t.Errorf("a backup was written (%v)", err)
	}
}

func TestOpenEditorReportsAnInvalidFile(t *testing.T) {
	p := write(t, strings.Replace(commented, "    path: ~/repos/sandbox\n", "", 1))
	if _, err := OpenEditor(p); err == nil || !strings.Contains(err.Error(), "projects[0].path") {
		t.Fatalf("err = %v, want projects[0].path reported", err)
	}
}

func TestAddProjectRefusesAnExistingRef(t *testing.T) {
	p := write(t, commented)
	e, err := OpenEditor(p)
	if err != nil {
		t.Fatal(err)
	}
	err = e.AddProject(NewProject{Ref: "gh:alice/sandbox", Path: "~/repos/sandbox2", Host: "ubuntu-a", BaseBranch: "main"})
	if !errors.Is(err, ErrProjectExists) {
		t.Fatalf("err = %v, want ErrProjectExists", err)
	}
}

func TestAddProjectRefusesAFileChangedSinceOpened(t *testing.T) {
	p := write(t, commented)
	e, err := OpenEditor(p)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(commented, "plan_first: false", "plan_first: true", 1)
	if err := os.WriteFile(p, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.AddProject(notes); !errors.Is(err, ErrConfigChanged) {
		t.Fatalf("err = %v, want ErrConfigChanged", err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != edited {
		t.Errorf("the hand edit was lost:\n%s", got)
	}
}

func TestEditorProjectPathKeepsTheWrittenRoot(t *testing.T) {
	e, err := OpenEditor(write(t, commented))
	if err != nil {
		t.Fatal(err)
	}
	if got := e.ProjectPath("notes"); got != "~/repos/notes" {
		t.Errorf("project path = %q, want ~/repos/notes", got)
	}
}

func TestParseRef(t *testing.T) {
	p, err := ParseRef("gl:nas/tools/sb")
	if err != nil || p.Forge != "gl" || p.Owner != "nas/tools" || p.Repo != "sb" {
		t.Fatalf("ParseRef = %+v, %v", p, err)
	}
	if _, err := ParseRef("github.com/o/r"); err == nil {
		t.Fatal("a URL was taken for a reference")
	}
}
