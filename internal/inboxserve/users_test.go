package inboxserve

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
	"github.com/uskanda/switchboard/internal/marker"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/webauth"
)

// userGitLab is the fake GitLab as one person: it posts as them and cannot read the projects in hidden.
type userGitLab struct {
	*fake.Forge
	login  string
	hidden map[string]bool
}

func (u userGitLab) PostComment(_ context.Context, ref forge.IssueRef, body string) (*forge.Comment, error) {
	return u.PostCommentAs(ref, body, u.login)
}

func (u userGitLab) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	if u.hidden[owner+"/"+repo] {
		return "", forge.ErrNotFound
	}
	return u.Forge.DefaultBranch(ctx, owner, repo)
}

type audits struct{ lines []map[string]any }

func (a *audits) Log(event string, fields map[string]any) {
	if event == "web.action" {
		a.lines = append(a.lines, fields)
	}
}

var _ state.Auditor = (*audits)(nil)

// multi turns the login on with alice and bob in team, olivia the owner, and alice trusted on GitLab. A GitLab issue
// gl:nas/sb#5 waits; hidden lists the GitLab projects bob cannot read.
func multi(t *testing.T, hidden ...string) (*world, *fake.Forge, *audits) {
	w := newWorld(t)
	p := withAuth(w)
	p.groups["bob"] = []string{"team"}
	p.groups["olivia"] = nil
	w.srv.Auth.Allow.Owner = "olivia"
	g := w.gl.Forge.(*fake.Forge)
	me := g.AddUser("me", 1)
	for i, u := range []string{"alice", "bob", "olivia"} {
		g.AddUser(u, int64(10+i))
	}
	ref := g.AddIssue("nas", "sb", 5, "Pick a port", "body", me)
	g.PostAs = "bot"
	g.PostComment(ctx, ref, "### The agent has a question\n\n**Question**\n\nWhich port?\n\n"+
		marker.Marker{Host: "laptop", Run: "r2", State: "waiting", Reason: "question", At: w.now.Add(-time.Hour)}.Format()+"\n")
	g.AddLabel(ref, "agent:waiting", w.bot)
	g.PostAs = "me"
	hide := map[string]bool{}
	for _, h := range hidden {
		hide[h] = true
	}
	w.srv.Users = &Users{
		Owner: "olivia", TrustedGitLab: []string{"alice"},
		GitLab: func(token string) UserForge {
			login := strings.TrimPrefix(token, "at-")
			u := userGitLab{Forge: g, login: login}
			if login == "bob" {
				u.hidden = hide
			}
			return u
		},
	}
	a := &audits{}
	w.srv.Audit = a
	return w, g, a
}

func as(t *testing.T, w *world, user string) context.Context {
	id, err := logIn(t, w, user)
	if err != nil {
		t.Fatal(err)
	}
	return inboxapi.WithSession(ctx, id)
}

func itemRefs(in *inboxapi.Inbox) string {
	var out []string
	for _, it := range in.Items {
		out = append(out, it.Ref)
	}
	return strings.Join(out, " ")
}

func TestAMemberRepliesOnGitLabAsThemselves(t *testing.T) {
	w, g, a := multi(t)
	if err := w.c.Reply(as(t, w, "alice"), "gl:nas/sb#5", "Use 8080.", false); err != nil {
		t.Fatal(err)
	}
	cs := g.CommentsOf(forge.IssueRef{Forge: forge.GitLab, Owner: "nas", Repo: "sb", Number: 5})
	if last := cs[len(cs)-1]; last.Author.Login != "alice" || last.Body != "Use 8080." {
		t.Fatalf("last comment = %+v, want alice's reply", last)
	}
	if len(a.lines) != 1 || a.lines[0]["actor"] != "alice" || a.lines[0]["action"] != "reply" || a.lines[0]["target"] != "gl:nas/sb#5" {
		t.Fatalf("audit = %+v", a.lines)
	}
}

func TestOnlyTheOwnerSeesAndWritesGitHub(t *testing.T) {
	w, _, _ := multi(t)
	alice := as(t, w, "alice")
	in, err := w.c.List(alice)
	if err != nil {
		t.Fatal(err)
	}
	if got := itemRefs(in); got != "gl:nas/sb#5" {
		t.Fatalf("alice's list = %s, want the GitLab issue only", got)
	}
	if _, err := w.c.Issue(alice, "gh:o/r#12"); !errors.Is(err, inboxapi.ErrNotFound) {
		t.Fatalf("alice opening a GitHub issue: err = %v, want ErrNotFound", err)
	}
	if err := w.c.Reply(alice, "gh:o/r#12", "B", false); !errors.Is(err, inboxapi.ErrNotFound) {
		t.Fatalf("alice replying on GitHub: err = %v, want ErrNotFound", err)
	}

	olivia := as(t, w, "olivia")
	in, _ = w.c.List(olivia)
	if got := itemRefs(in); !strings.Contains(got, "gh:o/r#12") || !strings.Contains(got, "gl:nas/sb#5") {
		t.Fatalf("olivia's list = %s, want both", got)
	}
	if err := w.c.Reply(olivia, "gh:o/r#12", "Go with B.", false); err != nil {
		t.Fatalf("olivia replying on GitHub: %v", err)
	}
	cs := w.f.CommentsOf(w.ref)
	if last := cs[len(cs)-1]; last.Author.Login != "me" {
		t.Fatalf("olivia's GitHub reply was posted as %s, want the inbox token's me", last.Author.Login)
	}
}

func TestAProjectSomeoneCannotReadIsLeftOut(t *testing.T) {
	w, _, _ := multi(t, "nas/sb")
	bob := as(t, w, "bob")
	in, err := w.c.List(bob)
	if err != nil {
		t.Fatal(err)
	}
	if got := itemRefs(in); got != "" {
		t.Fatalf("bob's list = %q, want nothing", got)
	}
	if _, err := w.c.Issue(bob, "gl:nas/sb#5"); !errors.Is(err, inboxapi.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	targets, err := w.c.IssueTargets(bob)
	if err == nil && len(targets.Items) != 0 {
		t.Fatalf("bob's targets = %+v, want none", targets.Items)
	}
}

func TestReadableIsKeptTenMinutes(t *testing.T) {
	w, _, _ := multi(t)
	calls := 0
	gl := w.srv.Users.GitLab
	w.srv.Users.GitLab = func(token string) UserForge { calls++; return gl(token) }
	w.srv.Users.Now = func() time.Time { return w.now }
	alice := as(t, w, "alice")
	w.c.Issue(alice, "gl:nas/sb#5")
	w.c.Issue(alice, "gl:nas/sb#5")
	if calls != 1 {
		t.Fatalf("GitLab asked %d times within ten minutes, want 1", calls)
	}
	w.now = w.now.Add(11 * time.Minute)
	w.c.Issue(alice, "gl:nas/sb#5")
	if calls != 2 {
		t.Fatalf("GitLab asked %d times after ten minutes, want 2", calls)
	}
}

func TestTheIssueSaysWhetherTheReplyIsTrusted(t *testing.T) {
	w, _, _ := multi(t)
	d, err := w.c.Issue(as(t, w, "alice"), "gl:nas/sb#5")
	if err != nil || d.Trusted == nil || !*d.Trusted {
		t.Fatalf("alice: trusted = %v, err = %v; want true", d.Trusted, err)
	}
	d, err = w.c.Issue(as(t, w, "bob"), "gl:nas/sb#5")
	if err != nil || d.Trusted == nil || *d.Trusted {
		t.Fatalf("bob: trusted = %v, err = %v; want false", d.Trusted, err)
	}
}

func TestOnlyAnAdminEditsTheInstructions(t *testing.T) {
	w, _, _ := multi(t)
	w.srv.Instructions = &Instructions{Projects: []string{"gl:nas/sb"}}
	w.srv.Users.AdminGroups = []string{"team/leads"}
	_, err := w.c.Instructions(as(t, w, "alice"))
	if !errors.Is(err, inboxapi.ErrForbidden) {
		t.Fatalf("alice reading the instructions: err = %v, want ErrForbidden", err)
	}
	st, _ := w.c.AuthStatus(as(t, w, "olivia"))
	if !st.Admin {
		t.Errorf("olivia's status = %+v, want an admin", st)
	}
	if st, _ := w.c.AuthStatus(as(t, w, "alice")); st.Admin {
		t.Errorf("alice's status = %+v, want no admin", st)
	}
}

func TestAnAutomaticMergeWhoseAskerIsForgottenIsCancelled(t *testing.T) {
	u := &Users{}
	if _, err := u.mergesOf(ctx, "gl:nas/sb#5"); !errors.Is(err, errForgotten) {
		t.Fatalf("err = %v, want errForgotten", err)
	}
	s := &webauth.Session{User: "alice"}
	u.ask("gl:nas/sb#5", s)
	u.forget("gl:nas/sb#5")
	if _, err := u.mergesOf(ctx, "gl:nas/sb#5"); !errors.Is(err, errForgotten) {
		t.Fatalf("after forget: err = %v, want errForgotten", err)
	}
}

func TestAdmin(t *testing.T) {
	u := &Users{Owner: "olivia", AdminGroups: []string{"ops"}}
	for _, c := range []struct {
		user   string
		groups []string
		want   bool
	}{
		{"olivia", nil, true}, {"carol", []string{"ops/infra"}, true}, {"alice", []string{"team"}, false},
	} {
		if got := u.Admin(&webauth.Session{User: c.user, Groups: c.groups}); got != c.want {
			t.Errorf("Admin(%s) = %v, want %v", c.user, got, c.want)
		}
	}
}
