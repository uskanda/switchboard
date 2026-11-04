package inboxserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// LabelsTTL is how long the labels read of a project are reused by GET /v1/projects.
const LabelsTTL = time.Minute

// Projects serves /v1/projects: the projects of this machine and their state, and adding one to the configuration
// file. It reads the forges with the inbox token only; the clone and the labels are the agent's to make.
type Projects struct {
	// Config is the configuration in use; Server.Reload replaces it.
	Config *config.Config
	// Repos reads repositories with each forge's inbox token, by forge kind (forge.GitHub, forge.GitLab).
	Repos map[string]forge.Repos
	// Reload takes the configuration file after a project was added, so that the API serves the project at once.
	Reload func(*config.Config)
	// IsClone reports whether dir holds a git clone; git is asked when nil.
	IsClone func(dir string) bool

	mu     sync.Mutex
	labels map[string]labelsRead // by project ref
}

type labelsRead struct {
	names []string
	at    time.Time
}

func (s *Server) projectConfig() *config.Config {
	s.vmu.RLock()
	defer s.vmu.RUnlock()
	return s.Projects.Config
}

// projects serves GET (list) and POST (add one) of /v1/projects.
func (s *Server) projects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.projectConfig()
		out := inboxapi.Projects{Host: cfg.Host.Name, Enabled: cfg.Inbox.ProjectAddEnabled(), Items: []inboxapi.Project{}}
		for _, p := range cfg.ProjectsFor(cfg.Host.Name) {
			out.Items = append(out.Items, s.Projects.state(r.Context(), cfg, p, s.now()))
		}
		send(w, http.StatusOK, out)
	case http.MethodPost:
		s.addProject(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		fail(w, http.StatusMethodNotAllowed, inboxapi.CodeInvalid, "use GET or POST")
	}
}

// addProject adds the requested project to the configuration file, after checking it against the file as it is
// now and reading the repository with the inbox token.
func (s *Server) addProject(w http.ResponseWriter, r *http.Request) {
	var req inboxapi.AddProjectRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, "the body is not a project to add")
		return
	}
	p, err := config.ParseRef(strings.TrimSpace(req.Ref))
	if err != nil {
		fail(w, http.StatusBadRequest, inboxapi.CodeInvalid, err.Error())
		return
	}
	e, err := config.OpenEditor(s.projectConfig().Path)
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, inboxapi.CodeConfigInvalid, err.Error())
		return
	}
	cfg := e.Config()
	if !cfg.Inbox.ProjectAddEnabled() {
		fail(w, http.StatusForbidden, inboxapi.CodeAddingDisabled, "adding projects is disabled; set inbox.project_root in the configuration to enable it")
		return
	}
	used := false
	for _, q := range cfg.Projects {
		if strings.EqualFold(q.Ref, p.Ref) {
			fail(w, http.StatusConflict, inboxapi.CodeProjectExists, p.Ref+" is in the configuration already")
			return
		}
		used = used || (q.Host == cfg.Host.Name && q.Forge == p.Forge)
	}
	repos := s.Projects.Repos[p.Forge]
	if !used || repos == nil {
		fail(w, http.StatusUnprocessableEntity, inboxapi.CodeForgeNotUsed,
			"no project of this machine is on the forge of "+p.Ref+"; add it to the configuration by hand and run switchboard service install again, so that the agent gets the forge's token")
		return
	}
	dir := filepath.Join(cfg.Inbox.ProjectRoot, p.Repo)
	for _, q := range cfg.Projects {
		if filepath.Clean(q.Path) == dir {
			fail(w, http.StatusConflict, inboxapi.CodePathTaken, q.Ref+" is cloned at "+dir+" already")
			return
		}
	}
	branch, err := repos.DefaultBranch(r.Context(), p.Owner, p.Repo)
	switch {
	case errors.Is(err, forge.ErrNotFound), errors.Is(err, forge.ErrForbidden):
		fail(w, http.StatusUnprocessableEntity, inboxapi.CodeRepoUnreadable, p.Ref+" cannot be read with the inbox token: "+err.Error())
		return
	case err != nil:
		fail(w, http.StatusBadGateway, inboxapi.CodeForge, err.Error())
		return
	}
	err = e.AddProject(config.NewProject{Ref: p.Ref, Path: e.ProjectPath(p.Repo), Host: cfg.Host.Name, BaseBranch: branch, PlanFirst: req.PlanFirst})
	switch {
	case errors.Is(err, config.ErrConfigChanged):
		fail(w, http.StatusConflict, inboxapi.CodeConfigChanged, err.Error())
		return
	case errors.Is(err, config.ErrProjectExists):
		fail(w, http.StatusConflict, inboxapi.CodeProjectExists, err.Error())
		return
	case err != nil:
		fail(w, http.StatusUnprocessableEntity, inboxapi.CodeConfigInvalid, err.Error())
		return
	}
	next := e.Config()
	if s.Projects.Reload != nil {
		s.Projects.Reload(next)
	}
	out := inboxapi.AddProjectResponse{}
	for _, q := range next.Projects {
		if q.Ref == p.Ref {
			out.Project = s.Projects.state(r.Context(), next, q, s.now())
		}
	}
	// Only GitLab, on the relay's machine or its LAN, can send to the relay (ADR-007, ADR-060); a GitHub project is polled.
	if next.Agent.Relay != nil && p.Forge == forge.GitLab {
		out.Webhook = "switchboard relay hook add " + p.Ref
	}
	send(w, http.StatusCreated, out)
}

// state is what the API says of project p: whether it is cloned, and which state labels it lacks.
func (pr *Projects) state(ctx context.Context, cfg *config.Config, p config.Project, now time.Time) inboxapi.Project {
	out := inboxapi.Project{Ref: p.Ref, Path: p.Path, PlanFirst: p.PlanFirst, Cloned: pr.isClone(p.Path), MissingLabels: []string{}}
	names, err := pr.labelNames(ctx, p, now)
	if err != nil {
		out.LabelsError = err.Error()
		return out
	}
	have := map[string]bool{}
	for _, n := range names {
		have[n] = true
	}
	n := cfg.Labels
	for _, l := range []string{n.Ready, n.Plan, n.Go, n.Running, n.Waiting, n.Done} {
		if l != "" && !have[l] {
			out.MissingLabels = append(out.MissingLabels, l)
		}
	}
	return out
}

// labelNames reads the labels of p's repository, reusing a read younger than LabelsTTL. A failed read is not kept.
func (pr *Projects) labelNames(ctx context.Context, p config.Project, now time.Time) ([]string, error) {
	pr.mu.Lock()
	if l, ok := pr.labels[p.Ref]; ok && now.Sub(l.at) < LabelsTTL {
		pr.mu.Unlock()
		return l.names, nil
	}
	pr.mu.Unlock()
	repos := pr.Repos[p.Forge]
	if repos == nil {
		return nil, errors.New("no client for the forge")
	}
	names, err := repos.LabelNames(ctx, p.Owner, p.Repo)
	if err != nil {
		return nil, err
	}
	pr.mu.Lock()
	if pr.labels == nil {
		pr.labels = map[string]labelsRead{}
	}
	pr.labels[p.Ref] = labelsRead{names: names, at: now}
	pr.mu.Unlock()
	return names, nil
}

// isClone reports whether dir is the top of a git clone; a directory inside some other clone is not.
func (pr *Projects) isClone(dir string) bool {
	if pr.IsClone != nil {
		return pr.IsClone(dir)
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false
	}
	want, err := filepath.EvalSymlinks(dir)
	return err == nil && strings.TrimSpace(string(out)) == want
}
