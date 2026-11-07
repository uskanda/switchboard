package inboxserve

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/marker"
)

// NewIssues serves /v1/issue-targets and /v1/issues: opening an issue, with a start label, in any configured project.
// The issue is opened with the inbox token, so it is the person's own and the agent takes it up (ADR-040); one opened
// without a start label waits until the person puts one on it (ADR-049).
type NewIssues struct {
	// Config is the configuration in use; Server.Reload replaces it.
	Config *config.Config
	// Issues opens issues with each forge's inbox token, by forge kind (forge.GitHub, forge.GitLab).
	Issues map[string]forge.Issues
}

func (s *Server) newIssuesConfig() *config.Config {
	s.vmu.RLock()
	defer s.vmu.RUnlock()
	return s.NewIssues.Config
}

// starts are the start labels the configuration names, as kinds a request can choose from, then StartNone, which
// stands for no label and is always there (ADR-049).
func starts(l config.Labels) []inboxapi.IssueStart {
	out := []inboxapi.IssueStart{}
	for _, st := range []inboxapi.IssueStart{
		{Kind: inboxapi.StartReady, Label: l.Ready}, {Kind: inboxapi.StartPlan, Label: l.Plan}, {Kind: inboxapi.StartGo, Label: l.Go},
	} {
		if st.Label != "" {
			out = append(out, st)
		}
	}
	return append(out, inboxapi.IssueStart{Kind: inboxapi.StartNone})
}

// issueTargets serves GET /v1/issue-targets.
func (s *Server) issueTargets(w http.ResponseWriter, r *http.Request) {
	cfg := s.newIssuesConfig()
	out := inboxapi.IssueTargets{Items: []inboxapi.IssueTarget{}, Starts: starts(cfg.Labels)}
	for _, p := range cfg.Projects {
		if !s.sees(r.Context(), p.Ref) {
			continue
		}
		out.Items = append(out.Items, inboxapi.IssueTarget{Ref: p.Ref, Host: p.Host, PlanFirst: p.PlanFirst.Plans(), PlanFirstMode: string(p.PlanFirst.Mode())})
	}
	send(w, http.StatusOK, out)
}

// createIssue serves POST /v1/issues. The browser names a kind of start; the label is the configuration's.
func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) {
	var req inboxapi.CreateIssueRequest
	// JSON may escape every byte of the body six times over; the body itself is measured after decoding.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 6*inboxapi.MaxIssueBody+1<<16)).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			fail(w, http.StatusRequestEntityTooLarge, inboxapi.CodeTooLarge, "the request is too large")
			return
		}
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not an issue to open")
		return
	}
	cfg := s.newIssuesConfig()
	var p *config.Project
	for i := range cfg.Projects {
		if cfg.Projects[i].Ref == strings.TrimSpace(req.Ref) {
			p = &cfg.Projects[i]
		}
	}
	if p == nil {
		fail(w, http.StatusUnprocessableEntity, inboxapi.CodeUnknownProject, req.Ref+" is not a project in the configuration")
		return
	}
	title := strings.TrimSpace(req.Title)
	switch {
	case title == "":
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the title is empty")
		return
	case utf8.RuneCountInString(title) > inboxapi.MaxIssueTitle:
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, fmt.Sprintf("the title is longer than %d characters", inboxapi.MaxIssueTitle))
		return
	case len(req.Body) > inboxapi.MaxIssueBody:
		fail(w, http.StatusRequestEntityTooLarge, inboxapi.CodeTooLarge, fmt.Sprintf("the body is larger than %d bytes", inboxapi.MaxIssueBody))
		return
	}
	if _, ok := marker.Parse(strings.TrimSpace(req.Body)); ok {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body ends in a switchboard marker line; remove it")
		return
	}
	// The kind is looked up, never taken as empty: a request that leaves it out is refused, not opened without a label.
	var labels []string
	known := false
	for _, st := range starts(cfg.Labels) {
		if st.Kind == req.Start {
			known = true
			if st.Label != "" {
				labels = []string{st.Label}
			}
		}
	}
	if !known {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, fmt.Sprintf("%q is not a start label of the configuration", req.Start))
		return
	}
	if !s.sees(r.Context(), p.Ref) {
		fail(w, http.StatusUnprocessableEntity, inboxapi.CodeUnknownProject, req.Ref+" is not a project you may see")
		return
	}
	issues := s.NewIssues.Issues[p.Forge]
	if f, err := s.writer(r.Context(), p.Forge); failWrite(w, err) {
		return
	} else if f != nil {
		issues = f
	}
	if issues == nil {
		fail(w, http.StatusBadGateway, inboxapi.CodeForge, "no client for the forge of "+p.Ref)
		return
	}
	is, err := issues.CreateIssue(r.Context(), p.Owner, p.Repo, title, req.Body, labels)
	if err != nil {
		fail(w, http.StatusBadGateway, inboxapi.CodeForge, err.Error())
		return
	}
	s.act(r.Context(), "issue.create", is.Ref.String())
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
	s.readAgain()
	send(w, http.StatusCreated, inboxapi.CreateIssueResponse{Ref: is.Ref.String(), URL: is.URL})
}
