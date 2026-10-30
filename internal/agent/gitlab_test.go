package agent

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/trust"
)

// gl extends a world with a fake GitLab holding the project nas/sb, its own git origin and clone, and a route that
// sends that project to the GitLab fake.
type gl struct {
	*h
	g               *fake.Forge
	gref            forge.IssueRef
	gorigin, gclone string
	log             bytes.Buffer
}

func newGL(t *testing.T, edit func(*config.Config)) *gl {
	t.Helper()
	x := &gl{}
	x.h = newH(t, "", func(c *config.Config) {
		c.Forges.GitLab = config.GitLab{BaseURL: "https://gl.test", TokenRef: "g", AgentIdentity: "agent"}
		c.TrustedAuthors.GitLab = []string{"master"}
		if edit != nil {
			edit(c)
		}
	})
	root := t.TempDir()
	x.gorigin = filepath.Join(root, "origin.git")
	seed := filepath.Join(root, "seed")
	run(t, root, "git", "init", "-q", "--bare", "-b", "main", x.gorigin)
	run(t, root, "git", "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("gitlab sandbox\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, seed, "git", "add", "README.md")
	run(t, seed, "git", "commit", "-q", "-m", "init")
	run(t, seed, "git", "push", "-q", x.gorigin, "main")
	x.gclone = filepath.Join(root, "clone")
	run(t, root, "git", "clone", "-q", x.gorigin, x.gclone)
	run(t, x.gclone, "git", "config", "user.name", "agent")
	run(t, x.gclone, "git", "config", "user.email", "agent@example.test")
	x.cfg.Projects = append(x.cfg.Projects, config.Project{
		Ref: "gl:nas/sb", Path: x.gclone, Host: "test", BaseBranch: "main", Forge: "gl", Owner: "nas", Repo: "sb",
	})

	x.g = fake.NewGitLab(nil)
	x.g.AddUser("master", 1)
	x.g.AddUser("agent", 2)
	x.g.AddProject("nas", "sb", "private")
	x.gref = x.g.AddIssue("nas", "sb", 3, "Add a greeting", "Please add greeting.txt.", x.master)
	return x
}

func (x *gl) agent() *Agent {
	x.t.Helper()
	a := x.h.agentWith(func(a *Agent) {
		j, err := trust.ResolveFor(x.ctx, x.g, x.dir, forge.GitLab, x.cfg.TrustedAuthors.GitLab, "agent", nil, x.audit)
		if err != nil {
			x.t.Fatal(err)
		}
		a.Routes = map[string]Route{"gl:nas/sb": {Forge: x.g, Judge: j}}
		a.Log = &x.log
	})
	return a
}

func (x *gl) poll() {
	x.t.Helper()
	if x.a == nil {
		x.agent()
	}
	if err := x.a.Poll(x.ctx); err != nil {
		x.t.Fatal(err)
	}
}

func (x *gl) glabels() string { return strings.Join(x.g.Labels(x.gref), ",") }

// gposts returns the agent's GitLab comments, without the ones that claim a run.
func (x *gl) gposts() []forge.Comment {
	var out []forge.Comment
	for _, c := range x.g.CommentsOf(x.gref) {
		if m, ok := marker.Parse(c.Body); c.Author.ID == x.bot.ID && !(ok && m.State == "running") {
			out = append(out, c)
		}
	}
	return out
}

func gitOK(dir string, args ...string) bool {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// One agent polls a GitHub and a GitLab project, and writes each issue's outcome to its own forge.
func TestMixedForgesArePolledTogether(t *testing.T) {
	x := newGL(t, nil) // one run at a time: the fake claude counts its starts without a lock
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.starts() != 2 || x.labels(x.ref) != "agent:waiting" || x.glabels() != "agent:waiting" {
		t.Fatalf("starts=%d github=%q gitlab=%q", x.starts(), x.labels(x.ref), x.glabels())
	}
	if len(x.posts(x.ref)) != 1 || len(x.gposts()) != 1 {
		t.Fatalf("github posts %d, gitlab posts %d", len(x.posts(x.ref)), len(x.gposts()))
	}
}

// On GitLab a note leaves the issue's update time alone (V22); the agent still sees the reply and runs again.
func TestGitLabReplyWithoutAnUpdateTimeResumes(t *testing.T) {
	x := newGL(t, nil)
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.starts() != 1 || x.glabels() != "agent:waiting" {
		t.Fatalf("starts=%d labels=%q", x.starts(), x.glabels())
	}
	x.poll()
	x.settle()
	if x.starts() != 1 {
		t.Fatalf("a poll without a reply ran again: starts=%d", x.starts())
	}
	x.g.AddComment(x.gref, "Use hi.", x.master)
	x.poll()
	x.settle()
	if x.starts() != 2 {
		t.Fatalf("the reply did not resume the run: starts=%d labels=%q", x.starts(), x.glabels())
	}
	if !strings.Contains(x.file("prompt-2.md"), "Use hi.") {
		t.Fatalf("the second prompt lacks the reply:\n%s", x.file("prompt-2.md"))
	}
}

func TestGitLabDoneOpensADraftMergeRequest(t *testing.T) {
	x := newGL(t, nil)
	x.step(0, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.glabels() != "agent:done" || len(x.g.PRs) != 1 || len(x.f.PRs) != 0 {
		t.Fatalf("labels=%q gitlab MRs=%d github PRs=%d", x.glabels(), len(x.g.PRs), len(x.f.PRs))
	}
	mr := x.g.PRs[0]
	if mr.Owner != "nas" || mr.Repo != "sb" || mr.Head != "switchboard/issue-3" || !mr.Draft || !strings.HasPrefix(mr.Body, "Closes #3") {
		t.Fatalf("merge request = %+v", mr)
	}
	if !gitOK(x.gorigin, "rev-parse", "-q", "--verify", "refs/heads/switchboard/issue-3") {
		t.Fatal("the branch was not pushed to the GitLab origin")
	}
	ps := x.gposts()
	if len(ps) != 1 || !strings.Contains(ps[0].Body, "Merge request: ") {
		t.Fatalf("posts = %+v", ps)
	}
}

func TestGitLabCIDefinitionIsProtected(t *testing.T) {
	x := newGL(t, nil)
	x.step(0, commit(".gitlab-ci.yml", "x: 1")+done("Changed CI."))
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.poll()
	x.settle()
	if x.glabels() != "agent:waiting" || len(x.g.PRs) != 0 {
		t.Fatalf("labels=%q MRs=%d", x.glabels(), len(x.g.PRs))
	}
	ps := x.gposts()
	if len(ps) != 1 || !strings.Contains(ps[0].Body, ".gitlab-ci.yml") {
		t.Fatalf("posts = %+v", ps)
	}
}

// A closed issue's worktree and local branch go away; the remote branch stays.
func TestClosedIssueWorktreeIsRemoved(t *testing.T) {
	x := newGL(t, nil)
	x.step(0, commit("greeting.txt", "hi")+done("Added greeting.txt."))
	x.g.AddLabel(x.gref, "agent:ready", x.master)
	x.poll()
	x.settle()
	wt := filepath.Join(x.dir.Path, "worktrees", "@gitlab", "nas", "sb", "issue-3")
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("no worktree after the run: %v", err)
	}
	x.g.Close(x.gref)
	x.poll()
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("the worktree is still there: %v", err)
	}
	if gitOK(x.gclone, "rev-parse", "-q", "--verify", "refs/heads/switchboard/issue-3") {
		t.Fatal("the local branch is still there")
	}
	if !gitOK(x.gorigin, "rev-parse", "-q", "--verify", "refs/heads/switchboard/issue-3") {
		t.Fatal("the remote branch was removed")
	}
}

func TestClosedIssueWorktreeWithChangesIsKept(t *testing.T) {
	x := newH(t, "", nil)
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.settle()
	wt := filepath.Join(x.dir.Path, "worktrees", "o", "r", "issue-12")
	if err := os.WriteFile(filepath.Join(wt, "notes.txt"), []byte("draft"), 0o644); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	x.a.Log = &log
	x.f.Close(x.ref)
	x.poll()
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("a worktree with changes was removed: %v", err)
	}
	if !strings.Contains(log.String(), "issue-12") || !strings.Contains(log.String(), "uncommitted") {
		t.Fatalf("log:\n%s", log.String())
	}
}

func TestRunningIssueWorktreeIsNotRemoved(t *testing.T) {
	x := newH(t, "", nil)
	x.step(1, block(1, waiting("s", "q")))
	x.f.AddLabel(x.ref, "agent:ready", x.master)
	x.poll()
	x.waitStarted(1)
	wt := filepath.Join(x.dir.Path, "worktrees", "o", "r", "issue-12")
	x.f.Close(x.ref)
	x.poll()
	if _, err := os.Stat(wt); err != nil {
		t.Fatalf("the worktree of a running issue was removed: %v", err)
	}
	x.release(1)
	x.settle()
	x.poll()
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("the worktree is still there after the run stopped: %v", err)
	}
}
