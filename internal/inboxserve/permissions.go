package inboxserve

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/permissions"
)

// Permissions is where the API reads and writes the allow rules granted from the web inbox (add-permission-grants):
// the store, the configured projects in order, and the name of this machine, whose agent alone reads them.
type Permissions struct {
	Store    permissions.Store
	Projects []string
	Host     string
}

// The scopes of a save of permissions.
const (
	scopeGlobal  = "global"
	scopeProject = "project"
	scopeIssue   = "issue"
)

// permissionsRoute serves GET (the rules of every scope, and of an issue) and POST (add and remove the rules of one
// scope) of /v1/permissions, to an admin alone when people log in.
func (s *Server) permissionsRoute(w http.ResponseWriter, r *http.Request) {
	if !s.adminOnly(w, r) {
		return
	}
	pm := s.Permissions
	s.vmu.RLock()
	projects := slices.Clone(pm.Projects)
	s.vmu.RUnlock()
	switch r.Method {
	case http.MethodGet:
		out := inboxapi.Permissions{Host: pm.Host, Items: []inboxapi.PermissionScope{}}
		add := func(scope, ref string, sc permissions.Scope) bool {
			rules, err := pm.Store.Read(sc)
			if err != nil {
				fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
				return false
			}
			out.Items = append(out.Items, inboxapi.PermissionScope{Scope: scope, Ref: ref, Rules: rules})
			return true
		}
		if !add(scopeGlobal, "", permissions.Scope{}) {
			return
		}
		for _, p := range projects {
			if !add(scopeProject, p, permissions.Scope{Project: p}) {
				return
			}
		}
		if issue := r.URL.Query().Get("issue"); issue != "" {
			ref, ok := s.issueRef(w, issue, projects)
			if !ok || !add(scopeIssue, ref.String(), permissions.Scope{Project: ref.Kind() + ":" + ref.Project(), Issue: ref.Number}) {
				return
			}
		}
		send(w, http.StatusOK, out)
	case http.MethodPost:
		var req inboxapi.SavePermissionsRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not a save of permissions")
			return
		}
		var sc permissions.Scope
		target := scopeGlobal
		switch req.Scope {
		case scopeGlobal:
		case scopeProject:
			if !slices.Contains(projects, req.Ref) {
				fail(w, http.StatusUnprocessableEntity, inboxapi.CodeUnknownProject, req.Ref+" is not a project in the configuration")
				return
			}
			sc, target = permissions.Scope{Project: req.Ref}, req.Ref
		case scopeIssue:
			ref, ok := s.issueRef(w, req.Ref, projects)
			if !ok {
				return
			}
			// The issue must be one the agent has posted on: rules for a number nobody uses are refused.
			if _, err := s.inbox().Detail(r.Context(), ref); err != nil {
				failWith(w, err)
				return
			}
			sc, target = permissions.Scope{Project: ref.Kind() + ":" + ref.Project(), Issue: ref.Number}, ref.String()
		default:
			fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the scope is global, project or issue")
			return
		}
		if len(req.Add) > 0 {
			var re *permissions.RuleError
			switch err := pm.Store.Add(sc, req.Add); {
			case errors.As(err, &re):
				e := inboxapi.Error{Code: inboxapi.CodeRuleRefused, Message: re.Error()}
				for _, rr := range re.Rules {
					e.Rules = append(e.Rules, inboxapi.RefusedRule{Rule: rr.Rule, Reason: rr.Reason})
				}
				send(w, http.StatusUnprocessableEntity, e)
				return
			case err != nil:
				fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
				return
			}
		}
		if len(req.Remove) > 0 {
			if err := pm.Store.Remove(sc, req.Remove); err != nil {
				fail(w, http.StatusInternalServerError, inboxapi.CodeForge, err.Error())
				return
			}
		}
		s.act(r.Context(), "permissions.save", target)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, http.StatusMethodNotAllowed, inboxapi.CodeInvalid, "use GET or POST")
	}
}

// issueRef is the issue arg names, which must be of a configured project; it answers the request itself and reports
// false when it is not.
func (s *Server) issueRef(w http.ResponseWriter, arg string, projects []string) (forge.IssueRef, bool) {
	ref, err := s.inbox().ParseRef(arg)
	if err != nil {
		failWith(w, err)
		return forge.IssueRef{}, false
	}
	if project := ref.Kind() + ":" + ref.Project(); !slices.Contains(projects, project) {
		fail(w, http.StatusUnprocessableEntity, inboxapi.CodeUnknownProject, project+" is not a project in the configuration")
		return forge.IssueRef{}, false
	}
	return ref, true
}
