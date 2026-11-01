package inboxserve

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/instructions"
)

const projectsYAML = `version: 1
host: {name: here}
forges:
  github: {token_ref: t, inbox_token_ref: ti, agent_identity: bot, search_scope: "user:me"}
trusted_authors: {github: [me]}
inbox:
  project_root: ~/repos
projects:
  - ref: gh:o/r
    path: /src/r
    host: here
  - ref: gh:o/far
    path: /src/far
    host: elsewhere
`

// failingRepos is a forge.Repos that cannot be reached.
type failingRepos struct{}

func (failingRepos) DefaultBranch(context.Context, string, string) (string, error) {
	return "", errors.New("connection refused")
}
func (failingRepos) LabelNames(context.Context, string, string) ([]string, error) {
	return nil, errors.New("connection refused")
}

// editingRepos edits the configuration file by hand while the default branch is read.
type editingRepos struct {
	forge.Repos
	path string
}

func (e editingRepos) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	data, _ := os.ReadFile(e.path)
	_ = os.WriteFile(e.path, append(data, "# edited by hand\n"...), 0o600)
	return e.Repos.DefaultBranch(ctx, owner, repo)
}

type projectsWorld struct {
	*world
	path   string
	clones map[string]bool
	store  instructions.Store
}

// withProjects serves the projects of a configuration file holding body, with HOME at /home/me.
func withProjects(t *testing.T, body string) *projectsWorld {
	t.Helper()
	t.Setenv("HOME", "/home/me")
	w := &projectsWorld{world: newWorld(t), clones: map[string]bool{"/src/r": true}}
	w.path = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(w.path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(w.path)
	if err != nil {
		t.Fatal(err)
	}
	w.f.AddRepo("o", "notes", true)
	w.f.SetDefaultBranch("o", "notes", "trunk")
	_, _ = w.f.EnsureLabel(ctx, "o", "r", "agent:ready", "", "")
	w.store = instructions.Store{Dir: filepath.Join(t.TempDir(), "instructions"), Audit: &events{}}
	w.srv.Instructions = &Instructions{Store: w.store, Projects: refs(cfg.Projects), Host: "here"}
	w.srv.Projects = &Projects{
		Config:  cfg,
		Repos:   map[string]forge.Repos{forge.GitHub: w.f},
		IsClone: func(dir string) bool { return w.clones[dir] },
		Reload: func(c *config.Config) {
			w.srv.Reload(c, &inbox.Inbox{Config: c, Forges: map[string]forge.Forge{forge.GitHub: w.gh}, Now: w.srv.Now})
		},
	}
	return w
}

func (w *projectsWorld) file() string {
	data, _ := os.ReadFile(w.path)
	return string(data)
}

func TestProjectsListsThisHostsProjectsAndTheirState(t *testing.T) {
	w := withProjects(t, projectsYAML)
	got, err := w.c.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "here" || !got.Enabled || len(got.Items) != 1 {
		t.Fatalf("projects = %+v, want the one project of here, adding enabled", got)
	}
	p := got.Items[0]
	want := []string{"agent:plan", "agent:go", "agent:running", "agent:waiting", "agent:done"}
	if p.Ref != "gh:o/r" || !p.Cloned || !slices.Equal(p.MissingLabels, want) || p.LabelsError != "" {
		t.Errorf("project = %+v, want cloned and missing %v", p, want)
	}
}

func TestProjectsReportsLabelsThatCannotBeRead(t *testing.T) {
	w := withProjects(t, projectsYAML)
	w.srv.Projects.Repos[forge.GitHub] = failingRepos{}
	got, err := w.c.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if p := got.Items[0]; !p.Cloned || p.LabelsError == "" || len(p.MissingLabels) != 0 {
		t.Errorf("project = %+v, want cloned with the labels error", p)
	}
}

func TestProjectsWithoutAProjectRootIsDisabled(t *testing.T) {
	w := withProjects(t, strings.Replace(projectsYAML, "inbox:\n  project_root: ~/repos\n", "", 1))
	got, err := w.c.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled {
		t.Error("adding is enabled without inbox.project_root")
	}
	if _, err := w.c.AddProject(ctx, inboxapi.AddProjectRequest{Ref: "gh:o/notes"}); !errors.Is(err, inboxapi.ErrAddingDisabled) {
		t.Errorf("err = %v, want ErrAddingDisabled", err)
	}
}

func TestAddProjectWritesTheProject(t *testing.T) {
	w := withProjects(t, projectsYAML)
	got, err := w.c.AddProject(ctx, inboxapi.AddProjectRequest{Ref: "gh:o/notes", PlanFirst: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Project.Ref != "gh:o/notes" || got.Project.Path != "/home/me/repos/notes" || got.Project.Cloned || got.Webhook != "" {
		t.Errorf("response = %+v, want notes at /home/me/repos/notes, not cloned, no webhook", got)
	}
	want := "  - ref: gh:o/notes\n    path: ~/repos/notes\n    host: here\n    base_branch: trunk\n    plan_first: true\n"
	if !strings.HasSuffix(w.file(), want) {
		t.Errorf("file:\n%s\nwant it to end in:\n%s", w.file(), want)
	}
}

func TestAddedProjectIsServedAtOnce(t *testing.T) {
	w := withProjects(t, projectsYAML)
	if _, err := w.c.AddProject(ctx, inboxapi.AddProjectRequest{Ref: "gh:o/notes"}); err != nil {
		t.Fatal(err)
	}
	in, err := w.c.Instructions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if last := in.Items[len(in.Items)-1]; last.Target != "gh:o/notes" || last.Content != "" {
		t.Errorf("instructions = %+v, want gh:o/notes last and empty", in.Items)
	}
	if err := w.c.SaveInstructions(ctx, "gh:o/notes", "Short notes."); err != nil {
		t.Errorf("saving the new project's instructions: %v", err)
	}
	ps, err := w.c.Projects(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps.Items) != 2 || ps.Items[1].Ref != "gh:o/notes" {
		t.Errorf("projects = %+v, want notes listed", ps.Items)
	}
}

const withRelay = "agent:\n  relay: {url: https://relay.example/events, token_ref: r}\n  reconcile_interval: 10m\ninbox:\n"

func TestAddProjectNamesTheWebhookCommandForGitLab(t *testing.T) {
	body := strings.Replace(projectsYAML, "inbox:\n", withRelay, 1)
	body = strings.Replace(body, "  github:", "  gitlab: {base_url: https://gl.example, token_ref: g, inbox_token_ref: gi, agent_identity: bot}\n  github:", 1)
	body = strings.Replace(body, "trusted_authors: {github: [me]}", "trusted_authors: {github: [me], gitlab: [me]}", 1)
	body += "  - ref: gl:nas/sb\n    path: /src/sb\n    host: here\n"
	w := withProjects(t, body)
	g := fake.NewGitLab(nil)
	g.AddProject("nas", "tools", "private")
	w.srv.Projects.Repos[forge.GitLab] = g
	got, err := w.c.AddProject(ctx, inboxapi.AddProjectRequest{Ref: "gl:nas/tools"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Webhook != "switchboard relay hook add gl:nas/tools" {
		t.Errorf("webhook = %q", got.Webhook)
	}
}

// GitHub cannot reach the relay, which listens on 127.0.0.1 (ADR-007), so a GitHub project is polled instead.
func TestAddProjectNamesNoWebhookForGitHub(t *testing.T) {
	w := withProjects(t, strings.Replace(projectsYAML, "inbox:\n", withRelay, 1))
	got, err := w.c.AddProject(ctx, inboxapi.AddProjectRequest{Ref: "gh:o/notes"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Webhook != "" {
		t.Errorf("webhook = %q, want none for a GitHub project", got.Webhook)
	}
}

func TestAddProjectRefusals(t *testing.T) {
	taken := strings.Replace(projectsYAML, "  - ref: gh:o/far\n", "  - ref: gh:x/notes\n    path: ~/repos/notes\n    host: here\n  - ref: gh:o/far\n", 1)
	for _, c := range []struct {
		name, body, ref string
		want            error
	}{
		{"not a reference", projectsYAML, "https://github.com/o/notes", inboxapi.ErrInvalid},
		{"configured already", projectsYAML, "gh:o/r", inboxapi.ErrProjectExists},
		{"configured for another host", projectsYAML, "gh:o/far", inboxapi.ErrProjectExists},
		{"a forge this host does not use", projectsYAML, "gl:nas/tools", inboxapi.ErrForgeNotUsed},
		{"a repository the token cannot read", projectsYAML, "gh:o/missing", inboxapi.ErrRepoUnreadable},
		{"a path another project has", taken, "gh:o/notes", inboxapi.ErrPathTaken},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := withProjects(t, c.body)
			before := w.file()
			if _, err := w.c.AddProject(ctx, inboxapi.AddProjectRequest{Ref: c.ref}); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if w.file() != before {
				t.Errorf("the file changed:\n%s", w.file())
			}
		})
	}
}

func TestAddProjectRefusesAnInvalidFile(t *testing.T) {
	w := withProjects(t, projectsYAML)
	invalid := strings.Replace(projectsYAML, "    path: /src/r\n", "", 1) // saved by hand after the service started
	if err := os.WriteFile(w.path, []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := w.c.AddProject(ctx, inboxapi.AddProjectRequest{Ref: "gh:o/notes"})
	if !errors.Is(err, inboxapi.ErrConfigInvalid) || !strings.Contains(err.Error(), "projects[0].path") {
		t.Fatalf("err = %v, want ErrConfigInvalid naming projects[0].path", err)
	}
	if w.file() != invalid {
		t.Errorf("the file changed:\n%s", w.file())
	}
}

func TestAddProjectRefusesAFileEditedMeanwhile(t *testing.T) {
	w := withProjects(t, projectsYAML)
	w.srv.Projects.Repos[forge.GitHub] = editingRepos{Repos: w.f, path: w.path}
	if _, err := w.c.AddProject(ctx, inboxapi.AddProjectRequest{Ref: "gh:o/notes"}); !errors.Is(err, inboxapi.ErrConfigChanged) {
		t.Fatalf("err = %v, want ErrConfigChanged", err)
	}
	if got := w.file(); !strings.HasSuffix(got, "# edited by hand\n") || strings.Contains(got, "gh:o/notes") {
		t.Errorf("file:\n%s\nwant the hand edit kept and nothing added", got)
	}
}

var _ forge.Repos = (*fake.Forge)(nil)
