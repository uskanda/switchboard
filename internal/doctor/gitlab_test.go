package doctor

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/secret"
)

// withGitLab adds a GitLab with two projects of this host: nas/a on the forge's token, and nas/b overridden with a
// project access token's bot. Each forge and token gets its own client.
func withGitLab(t *testing.T, w *world, visibility string) (*fake.Forge, map[string]string) {
	t.Helper()
	g := fake.NewGitLab(nil)
	g.AddUser("me", 1)
	g.AddUser("me-bot", 3)
	g.AddUser("project_7_bot", 50)
	g.AddProject("nas", "a", visibility)
	g.AddProject("nas", "b", "private")
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	w.cfg.Forges.GitLab = config.GitLab{BaseURL: "https://gl.test", TokenRef: "gl-agent", InboxTokenRef: "gl-inbox", AgentIdentity: "me-bot"}
	w.cfg.TrustedAuthors.GitLab = []string{"me"}
	w.cfg.Projects = append(w.cfg.Projects,
		config.Project{Ref: "gl:nas/a", Forge: "gl", Owner: "nas", Repo: "a", Host: "here", Path: repo},
		config.Project{Ref: "gl:nas/b", Forge: "gl", Owner: "nas", Repo: "b", Host: "here", Path: repo, TokenRef: "gl-bot", AgentIdentity: "project_7_bot"})
	w.d.Secrets = secret.Resolver{Store: store{"agent": "a", "inbox": "i", "gl-agent": "ga", "gl-inbox": "gi", "gl-bot": "gb"}}
	asked := map[string]string{}
	w.d.Forge = func(kind, token string) forge.Forge {
		asked[token] = kind
		if kind == forge.GitLab {
			return g
		}
		return w.f
	}
	return g, asked
}

func TestGitLabProjectsAreChecked(t *testing.T) {
	w := newWorld(t)
	_, asked := withGitLab(t, w, "private")
	if n := w.run(t); n != 0 {
		t.Fatalf("%d failures:\n%s", n, w.out.String())
	}
	w.expect(t, "ok", "agent token (forges.gitlab.token_ref gl-agent) found")
	w.expect(t, "ok", "inbox token (forges.gitlab.inbox_token_ref gl-inbox) found")
	w.expect(t, "ok", "gitlab https://gl.test is version")
	w.expect(t, "ok", "gl:nas/a is project ID")
	w.expect(t, "ok", "gl:nas/a is private")
	w.expect(t, "ok", "gl:nas/b: agent token (token_ref gl-bot) found")
	w.expect(t, "ok", "gl:nas/b: agent identity project_7_bot is user ID 50")
	if asked["gb"] != forge.GitLab || asked["ga"] != forge.GitLab || asked["a"] != forge.GitHub {
		t.Fatalf("clients = %v", asked)
	}
	ids, _ := w.d.State.Identities()
	if ids["gl:me"] != 1 || ids["gl:project_7_bot"] != 50 || ids["gh:me"] != 1 {
		t.Fatalf("identities = %v", ids)
	}
}

func TestInternalGitLabProjectIsNotPrivate(t *testing.T) {
	w := newWorld(t)
	withGitLab(t, w, "internal")
	w.cfg.Projects[1].IncludeUntrusted = true
	if n := w.run(t); n == 0 {
		t.Fatalf("include_untrusted on an internal project passed:\n%s", w.out.String())
	}
	w.expect(t, "fail", "gl:nas/a: include_untrusted is refused")
}

func TestGitHubOnlyDoesNotCheckGitLab(t *testing.T) {
	w := newWorld(t)
	if n := w.run(t); n != 0 {
		t.Fatalf("%d failures:\n%s", n, w.out.String())
	}
	for _, s := range []string{"gitlab", "gl:"} {
		if strings.Contains(w.out.String(), s) {
			t.Fatalf("GitHub-only output mentions %q:\n%s", s, w.out.String())
		}
	}
}
