package inbox

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
)

// mixed adds a GitLab with one configured project (nas/sb) whose issue has waited 30 minutes, next to the GitHub
// world, whose issues waited 3 and 5 hours. A second GitLab project is not configured.
type mixed struct {
	*world
	g  *fake.Forge
	gl forge.IssueRef
}

func newMixed(t *testing.T) *mixed {
	t.Helper()
	w := &mixed{world: newWorld(t), g: fake.NewGitLab(nil)}
	me := w.g.AddUser("me", 1)
	bot := w.g.AddUser("bot", 2)
	w.g.AddProject("nas", "sb", "private")
	w.g.AddProject("nas", "other", "private")
	w.b.Config.Forges.GitLab = config.GitLab{AgentIdentity: "bot"}
	w.b.Config.Projects = append(w.b.Config.Projects, config.Project{Ref: "gl:nas/sb", Forge: "gl", Owner: "nas", Repo: "sb", Host: "here"})
	w.b.Forges = map[string]forge.Forge{forge.GitLab: w.g}

	w.g.PostAs = "bot"
	w.gl = w.g.AddIssue("nas", "sb", 3, "Add a greeting", "body", me)
	w.g.PostComment(ctx, w.gl, post("hello or hi?", marker.Marker{Host: "nas", Run: "g1", State: "waiting", Reason: "question", At: w.now.Add(-30 * time.Minute)}))
	w.g.AddLabel(w.gl, "agent:waiting", bot)
	hidden := w.g.AddIssue("nas", "other", 1, "Elsewhere", "body", me)
	w.g.PostComment(ctx, hidden, post("Hidden?", marker.Marker{Host: "x", Run: "g2", State: "waiting", Reason: "question", At: w.now}))
	w.g.AddLabel(hidden, "agent:waiting", bot)
	w.g.PostAs = "me"
	return w
}

func refs(items []Item) string {
	var out []string
	for _, it := range items {
		out = append(out, it.Ref)
	}
	return strings.Join(out, " ")
}

func TestListMergesBothForgesOldestFirst(t *testing.T) {
	w := newMixed(t)
	items, err := w.b.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := refs(items); got != "gh:o/s#3 gh:o/r#12 gl:nas/sb#3" {
		t.Fatalf("listing = %s", got)
	}
	if it := items[2]; it.Host != "nas" || it.Question != "hello or hi?" || it.Project != "nas/sb" {
		t.Fatalf("GitLab item = %+v", it)
	}
}

func TestListFiltersByForge(t *testing.T) {
	w := newMixed(t)
	w.b.Only = forge.GitLab
	items, err := w.b.List(ctx)
	if err != nil || refs(items) != "gl:nas/sb#3" {
		t.Fatalf("--forge gitlab = %s, %v", refs(items), err)
	}
	w.b.Only = forge.GitHub
	items, err = w.b.List(ctx)
	if err != nil || refs(items) != "gh:o/s#3 gh:o/r#12" {
		t.Fatalf("--forge github = %s, %v", refs(items), err)
	}
}

// down is a forge that cannot be reached.
type down struct{ forge.Forge }

var errDown = errors.New("connection refused")

func (down) SearchOpenIssuesWithLabel(context.Context, string, string) ([]forge.IssueSummary, error) {
	return nil, errDown
}

func (down) User(context.Context, string) (*forge.User, error) { return nil, errDown }

func TestListShowsTheReachableForgeAndReportsTheOther(t *testing.T) {
	w := newMixed(t)
	w.b.Forges[forge.GitLab] = down{w.g}
	items, err := w.b.List(ctx)
	if refs(items) != "gh:o/s#3 gh:o/r#12" {
		t.Fatalf("listing = %s", refs(items))
	}
	if err == nil || !strings.Contains(err.Error(), "gitlab") || !errors.Is(err, errDown) {
		t.Fatalf("err = %v, want GitLab named as unreachable", err)
	}
	// The rows of what was listed are recorded, so show and reply work on them.
	if ref, err := w.b.Resolve("2"); err != nil || ref != w.a {
		t.Fatalf("Resolve(2) = %v, %v", ref, err)
	}
}

func TestShowAndReplyOnGitLab(t *testing.T) {
	w := newMixed(t)
	if _, err := w.b.List(ctx); err != nil {
		t.Fatal(err)
	}
	ref, err := w.b.Resolve("3")
	if err != nil || ref != w.gl {
		t.Fatalf("Resolve(3) = %v, %v", ref, err)
	}
	direct, err := w.b.Resolve("gl:nas/sb#3")
	if err != nil || direct != w.gl {
		t.Fatalf("Resolve(gl:nas/sb#3) = %v, %v", direct, err)
	}
	if _, err := w.b.Resolve("gl:nas/other#1"); err == nil || !strings.Contains(err.Error(), "not a configured project") {
		t.Fatalf("an unconfigured GitLab project resolved: %v", err)
	}
	var out bytes.Buffer
	if err := w.b.Show(ctx, ref, &out); err != nil || !strings.Contains(out.String(), "hello or hi?") {
		t.Fatalf("show = %q, %v", out.String(), err)
	}
	if err := w.b.Reply(ctx, ref, "hi", false); err != nil {
		t.Fatal(err)
	}
	cs := w.g.CommentsOf(w.gl)
	if last := cs[len(cs)-1]; last.Body != "hi" || last.Author.Login != "me" {
		t.Fatalf("last note = %+v", last)
	}
	if n := len(w.f.CommentsOf(w.a)); n != 1 {
		t.Fatalf("the reply reached GitHub too: %d comments", n)
	}
}

func TestResolveAcceptsSubgroups(t *testing.T) {
	w := newMixed(t)
	w.b.Config.Projects = append(w.b.Config.Projects, config.Project{Ref: "gl:nas/tools/sb", Forge: "gl", Owner: "nas/tools", Repo: "sb"})
	ref, err := w.b.Resolve("gl:nas/tools/sb#7")
	if err != nil || ref != (forge.IssueRef{Forge: forge.GitLab, Owner: "nas/tools", Repo: "sb", Number: 7}) {
		t.Fatalf("Resolve = %+v, %v", ref, err)
	}
}
