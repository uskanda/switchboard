package inboxserve

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/webauth"
)

// UserForge is the GitLab client of a logged-in person: the web inbox's writes to GitLab are made with it, and it
// tells which projects the person may read.
type UserForge interface {
	forge.Forge
	forge.Merges
	forge.Issues
	forge.Repos
}

// ReadableTTL is how long whether a person may read a project is known without asking GitLab again.
const ReadableTTL = 10 * time.Minute

// Users tells apart the people logged in to the web inbox (issue #146): who is an admin, what each one may see, and
// the client each one writes to GitLab with. It is set with Server.Auth. Owner is the one admin who may also see and
// write to GitHub projects, with GitHub's inbox token; TrustedGitLab are the GitLab logins whose replies the agent
// takes as instructions.
type Users struct {
	Owner         string
	AdminGroups   []string
	TrustedGitLab []string
	GitLab        func(token string) UserForge
	Now           func() time.Time // time.Now when nil

	mu       sync.Mutex
	readable map[string]readable
	// asked holds the session of whoever asked for each automatic merge still waiting, by its issue's ref: it is
	// merged as them. A restart forgets it, and the merge is cancelled.
	asked map[string]*webauth.Session
}

type readable struct {
	ok bool
	at time.Time
}

func (u *Users) now() time.Time {
	if u.Now != nil {
		return u.Now()
	}
	return time.Now()
}

// Admin tells whether s may edit the instructions and add projects: the owner, and the members of an admin group or
// of a group below one.
func (u *Users) Admin(s *webauth.Session) bool {
	return s.User == u.Owner || slices.ContainsFunc(s.Groups, func(g string) bool {
		return slices.ContainsFunc(u.AdminGroups, func(a string) bool { return g == a || strings.HasPrefix(g, a+"/") })
	})
}

// sees tells whether s may see the project ref names (gh:owner/repo or gl:group/project, with or without #n): a
// GitHub project only the owner, a GitLab one whoever can read it with their own token. A failure to ask GitLab is
// a no, and is not kept.
func (u *Users) sees(ctx context.Context, s *webauth.Session, ref string) bool {
	project, _, _ := strings.Cut(ref, "#")
	kind, path, ok := strings.Cut(project, ":")
	if !ok {
		return false
	}
	if kind != forge.GitLab {
		return s.User == u.Owner
	}
	key := s.User + "\x00" + path
	u.mu.Lock()
	r, hit := u.readable[key]
	u.mu.Unlock()
	if hit && u.now().Sub(r.at) < ReadableTTL {
		return r.ok
	}
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return false
	}
	tok, err := s.Token(ctx)
	if err != nil {
		return false
	}
	_, err = u.GitLab(tok).DefaultBranch(ctx, path[:i], path[i+1:])
	if err != nil && !errors.Is(err, forge.ErrNotFound) && !errors.Is(err, forge.ErrForbidden) {
		return false
	}
	u.mu.Lock()
	if u.readable == nil {
		u.readable = map[string]readable{}
	}
	u.readable[key] = readable{ok: err == nil, at: u.now()}
	u.mu.Unlock()
	return err == nil
}

// trusted tells whether the agent takes s's replies on a project of kind as instructions. On GitHub only the owner
// writes, with the inbox token, whose owner switchboard doctor checks against trusted_authors.github.
func (u *Users) trusted(s *webauth.Session, kind string) bool {
	if kind != forge.GitLab {
		return s.User == u.Owner
	}
	return slices.Contains(u.TrustedGitLab, s.User)
}

// errNotYours refuses a write on a GitHub project from someone other than the owner.
var errNotYours = errors.New("only the owner (inbox.auth.owner) writes to GitHub projects")

// writer gives the clients a write on ref is made with: nil (the inbox tokens) without the login, the person's own
// on GitLab, the inbox token for the owner on GitHub, and errNotYours for anyone else on GitHub.
func (s *Server) writer(ctx context.Context, kind string) (UserForge, error) {
	sess := SessionFrom(ctx)
	if s.Users == nil || sess == nil {
		return nil, nil
	}
	if kind != forge.GitLab {
		if sess.User == s.Users.Owner {
			return nil, nil
		}
		return nil, errNotYours
	}
	tok, err := sess.Token(ctx)
	if err != nil {
		return nil, err
	}
	return s.Users.GitLab(tok), nil
}

// sees tells whether the request's person may see ref; everyone may without the login.
func (s *Server) sees(ctx context.Context, ref string) bool {
	sess := SessionFrom(ctx)
	if s.Users == nil || sess == nil {
		return true
	}
	return s.Users.sees(ctx, sess, ref)
}

// visible is in with only what the request's person may see.
func (s *Server) visible(ctx context.Context, in *inboxapi.Inbox) *inboxapi.Inbox {
	sess := SessionFrom(ctx)
	if s.Users == nil || sess == nil || in == nil {
		return in
	}
	out := *in
	out.Items = []inboxapi.Item{}
	for _, it := range in.Items {
		if s.Users.sees(ctx, sess, it.Ref) {
			out.Items = append(out.Items, it)
		}
	}
	out.AutoMergeResults = nil
	for _, a := range in.AutoMergeResults {
		if s.Users.sees(ctx, sess, a.Ref) {
			out.AutoMergeResults = append(out.AutoMergeResults, a)
		}
	}
	return &out
}

// adminOnly refuses a request from someone logged in who is not an admin, and tells whether to go on.
func (s *Server) adminOnly(w http.ResponseWriter, r *http.Request) bool {
	sess := SessionFrom(r.Context())
	if s.Users == nil || sess == nil || s.Users.Admin(sess) {
		return true
	}
	fail(w, http.StatusForbidden, inboxapi.CodeForbidden, "only an admin (inbox.auth.admin_groups or owner) may do this")
	return false
}

// failWrite answers a write the person may not make.
func failWrite(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, errNotYours):
		fail(w, http.StatusForbidden, inboxapi.CodeForbidden, err.Error())
	case errors.Is(err, webauth.ErrNoSession):
		fail(w, http.StatusUnauthorized, inboxapi.CodeUnauthenticated, "log in to the web inbox again")
	case err != nil:
		fail(w, http.StatusBadGateway, inboxapi.CodeForge, err.Error())
	default:
		return false
	}
	return true
}

// writeError marks the errors of choosing the writer, so that the handlers answer them as such.
type writeError struct{ err error }

func (e writeError) Error() string { return e.err.Error() }
func (e writeError) Unwrap() error { return e.err }

// writeErr is the error of choosing the writer inside err, or nil.
func writeErr(err error) error {
	var we writeError
	if errors.As(err, &we) {
		return we.err
	}
	return nil
}

// replier is what replying as someone needs of the inbox: *inbox.Inbox posts with the client it is given.
type replier interface {
	ReplyAs(ctx context.Context, f forge.Forge, ref forge.IssueRef, text string, force bool) error
}

// replyAs posts text on ref as the request's person: with the inbox token without the login, else with what writer
// gives.
func (s *Server) replyAs(ctx context.Context, ref forge.IssueRef, text string, force bool) error {
	f, err := s.writer(ctx, ref.Kind())
	if err != nil {
		return writeError{err}
	}
	if f == nil {
		return s.inbox().Reply(ctx, ref, text, force)
	}
	rb, ok := s.inbox().(replier)
	if !ok {
		return errors.New("this inbox cannot reply as someone")
	}
	return rb.ReplyAs(ctx, f, ref, text, force)
}

// errForgotten reports an automatic merge whose asker this run of the inbox service does not know: it restarted.
var errForgotten = errors.New("the inbox service restarted since the merge was asked for")

// ask remembers that s asked for the automatic merge of ref.
func (u *Users) ask(ref string, s *webauth.Session) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.asked == nil {
		u.asked = map[string]*webauth.Session{}
	}
	u.asked[ref] = s
}

// forget drops the asker of ref's automatic merge, once it is over.
func (u *Users) forget(ref string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.asked, ref)
}

// mergesOf gives the client the automatic merge of ref is made with: its asker's. The session keeps its refresh
// token after a logout or its end, so the merge can still be made; errForgotten when no one is known.
func (u *Users) mergesOf(ctx context.Context, ref string) (forge.Merges, error) {
	u.mu.Lock()
	s := u.asked[ref]
	u.mu.Unlock()
	if s == nil {
		return nil, errForgotten
	}
	tok, err := s.Token(ctx)
	if err != nil {
		return nil, err
	}
	return u.GitLab(tok), nil
}

// act records a write made from the web inbox, with who made it, when people log in.
func (s *Server) act(ctx context.Context, action, target string) {
	sess := SessionFrom(ctx)
	if s.Audit == nil || sess == nil {
		return
	}
	s.Audit.Log("web.action", map[string]any{"actor": sess.User, "action": action, "target": target})
}
