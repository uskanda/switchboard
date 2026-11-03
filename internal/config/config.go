// Package config loads and validates the switchboard configuration file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/uskanda/switchboard/internal/claudecode"
)

// Config is the whole configuration file. Field comments name the YAML keys.
type Config struct {
	Version        int            `yaml:"version"`
	Host           Host           `yaml:"host"`
	Secrets        Secrets        `yaml:"secrets"`
	Forges         Forges         `yaml:"forges"`
	TrustedAuthors TrustedAuthors `yaml:"trusted_authors"`
	Labels         Labels         `yaml:"labels"`
	Comments       Comments       `yaml:"comments"`
	Claude         Claude         `yaml:"claude"`
	Agent          Agent          `yaml:"agent"`
	Projects       []Project      `yaml:"projects"`
	Relay          Relay          `yaml:"relay"`
	Notify         Notify         `yaml:"notify"`
	Plan           Plan           `yaml:"plan"`
	Inbox          Inbox          `yaml:"inbox"`

	// Path is the file the configuration was loaded from.
	Path string `yaml:"-"`
}

// Host names this machine; projects whose host matches run here.
type Host struct {
	Name string `yaml:"name"`
}

// Secrets controls where tokens may come from.
type Secrets struct {
	AllowEnv bool `yaml:"allow_env"`
}

// Forges holds the per-forge settings. A forge is configured when its token_ref is set; only the forges of the
// configured projects need to be.
type Forges struct {
	GitHub GitHub `yaml:"github"`
	GitLab GitLab `yaml:"gitlab"`
}

// GitHub is the GitHub forge setting.
type GitHub struct {
	TokenRef      string `yaml:"token_ref"`
	InboxTokenRef string `yaml:"inbox_token_ref"`
	AgentIdentity string `yaml:"agent_identity"`
	SearchScope   string `yaml:"search_scope"`
}

// GitLab is the GitLab forge setting. There is no search scope: the inbox reads the configured projects only.
type GitLab struct {
	BaseURL       string `yaml:"base_url"` // https://gitlab.com when empty
	TokenRef      string `yaml:"token_ref"`
	InboxTokenRef string `yaml:"inbox_token_ref"`
	AgentIdentity string `yaml:"agent_identity"`
}

// TrustedAuthors lists, per forge, the logins whose input is taken as an instruction.
type TrustedAuthors struct {
	GitHub []string `yaml:"github"`
	GitLab []string `yaml:"gitlab"`
}

// Labels are the names of the state labels. Ready, Plan and Go are the start labels: each means ready, and the one
// put on an issue decides whether its run plans first.
type Labels struct {
	Ready   string `yaml:"ready"`
	Plan    string `yaml:"plan"`
	Go      string `yaml:"go"`
	Running string `yaml:"running"`
	Waiting string `yaml:"waiting"`
	Done    string `yaml:"done"`
}

// Comments controls the comments the agent posts.
type Comments struct {
	Language     string `yaml:"language"`
	CheckCommand string `yaml:"check_command"`
}

// Claude controls how claude is started.
type Claude struct {
	Binary string `yaml:"binary"`
	Auth   string `yaml:"auth"`
}

// Agent controls the polling loop. With Relay set, the agent follows the relay's events and polls every
// ReconcileInterval while connected, every PollInterval while not.
type Agent struct {
	PollInterval      time.Duration `yaml:"poll_interval"`
	ReconcileInterval time.Duration `yaml:"reconcile_interval"`
	Timeout           time.Duration `yaml:"timeout"`
	MaxParallel       int           `yaml:"max_parallel"`
	// PreferPlan takes the plan runs out of the queue before the other runs (ADR-046).
	PreferPlan bool `yaml:"prefer_plan"`
	MaxTurns   int  `yaml:"max_turns"`
	// MaxFixAttempts caps the fix runs an issue gets from CI failures and rejected pushes before it waits; nil
	// means DefaultFixAttempts, 0 no automatic fixes.
	MaxFixAttempts *int        `yaml:"max_fix_attempts"`
	Relay          *AgentRelay `yaml:"relay"`
	// AllowToolsExtra are Claude Code permission rules added to the allow rules of every run that writes files, in
	// every project.
	AllowToolsExtra []string `yaml:"allow_tools_extra"`
	// Model and Effort are the model and the effort level of every run; Plan and Fix override them for the plan runs
	// and the fix runs. The names stay free of any one coding agent's terms; empty leaves the choice to it.
	Model  string   `yaml:"model"`
	Effort string   `yaml:"effort"`
	Plan   RunModel `yaml:"plan"`
	Fix    RunModel `yaml:"fix"`
}

// RunModel is a model and an effort level; an empty field takes the agent's default.
type RunModel struct {
	Model  string `yaml:"model"`
	Effort string `yaml:"effort"`
}

// RunKind tells the runs apart that may use another model.
type RunKind int

const (
	ImplementRun RunKind = iota // a direct run, or the implementation after an approved plan
	PlanRun
	FixRun // fixes a CI failure, a rejected push or a conflict with the base
)

// ModelFor is the model and the effort level of a run of kind: its override, filled from the defaults.
func (a Agent) ModelFor(kind RunKind) RunModel {
	m := RunModel{}
	switch kind {
	case PlanRun:
		m = a.Plan
	case FixRun:
		m = a.Fix
	}
	if m.Model == "" {
		m.Model = a.Model
	}
	if m.Effort == "" {
		m.Effort = a.Effort
	}
	return m
}

// DefaultFixAttempts is agent.max_fix_attempts when unset.
const DefaultFixAttempts = 2

// FixAttempts is how many fix runs an issue may get in a row.
func (a Agent) FixAttempts() int {
	if a.MaxFixAttempts == nil {
		return DefaultFixAttempts
	}
	return *a.MaxFixAttempts
}

// AgentRelay is where the agent subscribes: the relay's SSE URL and the token_ref of this host's bearer token.
type AgentRelay struct {
	URL      string `yaml:"url"`
	TokenRef string `yaml:"token_ref"`
}

// Plan controls the plan-first mode. ApproveWord starts the comment that approves a plan; /ok when unset.
type Plan struct {
	ApproveWord string `yaml:"approve_word"`
}

// Inbox controls the inbox service (switchboard inbox serve). RefreshInterval is how often it reads the forges
// again while a page listens for changes; 30s when unset, at least MinRefreshInterval. ProjectRoot is the directory
// the projects added from the web inbox are cloned under; adding is disabled without it. With Relay set, the
// service also follows the relay's events with a token of its own, and reads the forges again soon after each one.
// With Notify.FCM set, it reads the forges every RefreshInterval even with no page listening, and sends push
// notifications to the registered apps.
type Inbox struct {
	RefreshInterval time.Duration `yaml:"refresh_interval"`
	ProjectRoot     string        `yaml:"project_root"`
	Relay           *AgentRelay   `yaml:"relay"`
	Notify          InboxNotify   `yaml:"notify"`
}

// ProjectAddEnabled reports whether projects may be added through the inbox service.
func (i Inbox) ProjectAddEnabled() bool { return i.ProjectRoot != "" }

// MinRefreshInterval keeps the inbox's reads well under GitHub's search limit of 30 a minute.
const MinRefreshInterval = 10 * time.Second

// Project is one repository. Ref is the configured "gh:owner/repo" or "gl:group/project"; Forge ("gh" or "gl"),
// Owner and Repo are parsed from it. On GitLab, Owner is the namespace and may hold subgroups.
type Project struct {
	Ref                 string   `yaml:"ref"`
	Path                string   `yaml:"path"`
	Host                string   `yaml:"host"`
	BaseBranch          string   `yaml:"base_branch"`
	VerifyCommand       string   `yaml:"verify_command"`
	IncludeUntrusted    bool     `yaml:"include_untrusted"`
	ProtectedPathsExtra []string `yaml:"protected_paths_extra"`
	// PlanFirst makes the ready label start with a plan that waits for approval before any change is made.
	PlanFirst bool `yaml:"plan_first"`
	// AllowToolsExtra are Claude Code permission rules added, after agent.allow_tools_extra, to the allow rules of
	// this project's runs that write files.
	AllowToolsExtra []string `yaml:"allow_tools_extra"`
	// MaxParallel caps the runs of this project at once, under agent.max_parallel; nil leaves only that cap.
	MaxParallel *int `yaml:"max_parallel"`
	// TokenRef and AgentIdentity, set together, replace the forge's agent token and identity for this project, as
	// with a GitLab project access token. The inbox never uses them.
	TokenRef      string `yaml:"token_ref"`
	AgentIdentity string `yaml:"agent_identity"`
	// Icon is the project's icon in the web inbox, in place of the forge's avatar: an https:// URL of an image, the
	// path of an image file (~/ is expanded), or one emoji. IconKind tells which.
	Icon string `yaml:"icon"`

	Forge string `yaml:"-"`
	Owner string `yaml:"-"`
	Repo  string `yaml:"-"`
}

// ParallelLimit is the project's own cap on runs at once, or 0 when it has none.
func (p Project) ParallelLimit() int {
	if p.MaxParallel == nil {
		return 0
	}
	return *p.MaxParallel
}

// FullName is "owner/repo".
func (p Project) FullName() string { return p.Owner + "/" + p.Repo }

// DefaultPath is $XDG_CONFIG_HOME/switchboard/config.yaml, with ~/.config when XDG_CONFIG_HOME is unset.
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "switchboard", "config.yaml")
}

// tokenLike matches values that look like a forge token or a Telegram bot token. Such a value never belongs in the file.
var tokenLike = regexp.MustCompile(`^(ghp_|github_pat_|gho_|ghs_|ghu_|ghr_|glpat-|[0-9]{6,}:[A-Za-z0-9_-]{30,}$)`)

// Load reads, validates and completes the configuration at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	return parse(path, data)
}

// parse validates and completes the configuration data read from path.
func parse(path string, data []byte) (*Config, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if key := findTokenLike(&root, ""); key != "" {
		return nil, fmt.Errorf("%s: %s holds what looks like a token; never write a token in the configuration, store it in the Secret Service and name it with token_ref", path, key)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var c Config
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, nameKeys(err, &root))
	}
	_, written := scalarAt(&root, "plan", "approve_word")
	c.applyDefaults(written)
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.Path = path
	return &c, nil
}

var errLine = regexp.MustCompile(`line (\d+): `)

// nameKeys rewrites the decoder's "line N: ..." messages so that each also names the key at that line.
func nameKeys(err error, root *yaml.Node) error {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return err
	}
	msgs := make([]string, 0, len(te.Errors))
	for _, m := range te.Errors {
		if sub := errLine.FindStringSubmatch(m); sub != nil {
			var line int
			fmt.Sscanf(sub[1], "%d", &line)
			if key := keyAtLine(root, "", line); key != "" {
				m += " (" + key + ")"
			}
		}
		msgs = append(msgs, m)
	}
	return errors.New(strings.Join(msgs, "; "))
}

// keyAtLine returns the key path whose key or scalar value sits on line, or "".
func keyAtLine(n *yaml.Node, path string, line int) string {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			if k := keyAtLine(c, path, line); k != "" {
				return k
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if path != "" {
				key = path + "." + key
			}
			v := n.Content[i+1]
			if v.Kind == yaml.ScalarNode && (v.Line == line || n.Content[i].Line == line) {
				return key
			}
			if k := keyAtLine(v, key, line); k != "" {
				return k
			}
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			if k := keyAtLine(c, fmt.Sprintf("%s[%d]", path, i), line); k != "" {
				return k
			}
		}
	}
	return ""
}

// scalarAt returns the scalar at the key path under the document root, and whether it is there.
func scalarAt(root *yaml.Node, keys ...string) (string, bool) {
	n := root
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	for _, k := range keys {
		if n.Kind != yaml.MappingNode {
			return "", false
		}
		var next *yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == k {
				next = n.Content[i+1]
			}
		}
		if next == nil {
			return "", false
		}
		n = next
	}
	return n.Value, n.Kind == yaml.ScalarNode
}

// findTokenLike returns the key path of the first scalar that looks like a token, or "".
func findTokenLike(n *yaml.Node, path string) string {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			if k := findTokenLike(c, path); k != "" {
				return k
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if path != "" {
				key = path + "." + key
			}
			if k := findTokenLike(n.Content[i+1], key); k != "" {
				return k
			}
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			if k := findTokenLike(c, fmt.Sprintf("%s[%d]", path, i)); k != "" {
				return k
			}
		}
	case yaml.ScalarNode:
		if tokenLike.MatchString(n.Value) {
			return path
		}
	}
	return ""
}

// applyDefaults fills what the file left out. approveWritten says whether plan.approve_word was written, even as "",
// so that an empty word is refused rather than replaced.
func (c *Config) applyDefaults(approveWritten bool) {
	set := func(s *string, v string) {
		if *s == "" {
			*s = v
		}
	}
	set(&c.Labels.Ready, "agent:ready")
	set(&c.Labels.Plan, "agent:plan")
	set(&c.Labels.Go, "agent:go")
	set(&c.Labels.Running, "agent:running")
	set(&c.Labels.Waiting, "agent:waiting")
	set(&c.Labels.Done, "agent:done")
	set(&c.Comments.Language, "en")
	if !approveWritten {
		c.Plan.ApproveWord = "/ok"
	}
	set(&c.Claude.Binary, "claude")
	set(&c.Claude.Auth, "subscription")
	set(&c.Forges.GitLab.BaseURL, "https://gitlab.com")
	c.Forges.GitLab.BaseURL = strings.TrimRight(c.Forges.GitLab.BaseURL, "/")
	if c.Inbox.RefreshInterval == 0 {
		c.Inbox.RefreshInterval = 30 * time.Second
	}
	c.Inbox.ProjectRoot = expandHome(c.Inbox.ProjectRoot)
	if c.Agent.PollInterval == 0 {
		c.Agent.PollInterval = 60 * time.Second
	}
	if c.Agent.ReconcileInterval == 0 {
		c.Agent.ReconcileInterval = 10 * time.Minute
	}
	if c.Agent.Timeout == 0 {
		c.Agent.Timeout = 45 * time.Minute
	}
	if c.Agent.MaxParallel == 0 {
		c.Agent.MaxParallel = 1
	}
	if c.Agent.MaxTurns == 0 {
		c.Agent.MaxTurns = 80
	}
	set(&c.Relay.WebhookListen, "127.0.0.1:8781")
	set(&c.Relay.SSEListen, "127.0.0.1:8782")
	set(&c.Relay.WebhookURL, "http://"+c.Relay.WebhookListen)
	c.Relay.WebhookURL = strings.TrimRight(c.Relay.WebhookURL, "/")
	for i := range c.Projects {
		set(&c.Projects[i].BaseBranch, "main")
		c.Projects[i].Path = expandHome(c.Projects[i].Path)
		if strings.HasPrefix(c.Projects[i].Icon, "~/") {
			c.Projects[i].Icon = expandHome(c.Projects[i].Icon)
		}
	}
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return filepath.Join(os.Getenv("HOME"), strings.TrimPrefix(p, "~"))
	}
	return p
}

func (c *Config) validate() error {
	var errs []error
	need := func(v, key string) {
		if strings.TrimSpace(v) == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
	}
	if c.Version != 1 {
		errs = append(errs, fmt.Errorf("version must be 1, got %d", c.Version))
	}
	need(c.Host.Name, "host.name")
	if c.Inbox.RefreshInterval < MinRefreshInterval {
		errs = append(errs, fmt.Errorf("inbox.refresh_interval must be at least %v, got %v", MinRefreshInterval, c.Inbox.RefreshInterval))
	}
	if r := c.Inbox.ProjectRoot; r != "" && !filepath.IsAbs(r) {
		errs = append(errs, fmt.Errorf("inbox.project_root must be an absolute path or start with ~/, got %q", r))
	}
	if w := c.Plan.ApproveWord; strings.TrimSpace(w) == "" || strings.ContainsAny(w, " \t\r\n") {
		errs = append(errs, fmt.Errorf("plan.approve_word must be one word without spaces, got %q", w))
	}
	switch c.Comments.Language {
	case "en", "ja":
	default:
		errs = append(errs, fmt.Errorf("comments.language must be en or ja, got %q", c.Comments.Language))
	}
	switch c.Claude.Auth {
	case "subscription", "api_key":
	default:
		errs = append(errs, fmt.Errorf("claude.auth must be subscription or api_key, got %q", c.Claude.Auth))
	}
	if c.Agent.MaxParallel < 1 || c.Agent.MaxTurns < 1 || c.Agent.PollInterval < time.Second || c.Agent.Timeout < time.Minute {
		errs = append(errs, errors.New("agent: max_parallel and max_turns must be at least 1, poll_interval at least 1s, timeout at least 1m"))
	}
	if n := c.Agent.MaxFixAttempts; n != nil && *n < 0 {
		errs = append(errs, fmt.Errorf("agent.max_fix_attempts must be at least 0, got %d", *n))
	}
	errs = append(errs, checkAllowTools("agent.allow_tools_extra", c.Agent.AllowToolsExtra)...)
	for _, f := range []struct{ key, effort string }{
		{"agent.effort", c.Agent.Effort}, {"agent.plan.effort", c.Agent.Plan.Effort}, {"agent.fix.effort", c.Agent.Fix.Effort},
	} {
		if key, e := f.key, f.effort; e != "" && !slices.Contains(claudecode.Efforts, e) {
			errs = append(errs, fmt.Errorf("%s must be one of %s, got %q", key, strings.Join(claudecode.Efforts, ", "), e))
		}
	}
	if len(c.Projects) == 0 {
		errs = append(errs, errors.New("projects needs at least one project"))
	}
	for i := range c.Projects {
		p := &c.Projects[i]
		key := fmt.Sprintf("projects[%d]", i)
		need(p.Ref, key+".ref")
		need(p.Path, key+".path")
		need(p.Host, key+".host")
		if p.Ref == "" {
			continue
		}
		if err := p.parseRef(); err != nil {
			errs = append(errs, fmt.Errorf("%s.ref: %w", key, err))
		}
		errs = append(errs, checkAllowTools(key+".allow_tools_extra", p.AllowToolsExtra)...)
		if p.MaxParallel != nil && *p.MaxParallel < 1 {
			errs = append(errs, fmt.Errorf("%s.max_parallel must be at least 1, got %d", key, *p.MaxParallel))
		}
		if _, err := iconKind(p.Icon); err != nil {
			errs = append(errs, fmt.Errorf("%s.icon %w, got %q", key, err, p.Icon))
		}
		if (p.TokenRef == "") != (p.AgentIdentity == "") {
			errs = append(errs, fmt.Errorf("%s: token_ref and agent_identity override the agent together; set both or neither", key))
		}
	}
	if r := c.Agent.Relay; r != nil {
		need(r.URL, "agent.relay.url")
		need(r.TokenRef, "agent.relay.token_ref")
		if c.Agent.ReconcileInterval < c.Agent.PollInterval {
			errs = append(errs, errors.New("agent.reconcile_interval must not be shorter than poll_interval"))
		}
	}
	if r := c.Inbox.Relay; r != nil {
		need(r.URL, "inbox.relay.url")
		need(r.TokenRef, "inbox.relay.token_ref")
	}
	errs = append(errs, c.validateRelay()...)
	errs = append(errs, c.validateNotify()...)
	// A forge's settings are required once a project is on it.
	if c.Uses("gh") {
		need(c.Forges.GitHub.TokenRef, "forges.github.token_ref (a project is on GitHub)")
		need(c.Forges.GitHub.AgentIdentity, "forges.github.agent_identity")
		if len(c.TrustedAuthors.GitHub) == 0 {
			errs = append(errs, errors.New("trusted_authors.github needs at least one login"))
		}
	}
	if c.Uses("gl") {
		need(c.Forges.GitLab.TokenRef, "forges.gitlab.token_ref (a project is on GitLab)")
		need(c.Forges.GitLab.AgentIdentity, "forges.gitlab.agent_identity")
		if len(c.TrustedAuthors.GitLab) == 0 {
			errs = append(errs, errors.New("trusted_authors.gitlab needs at least one login"))
		}
	}
	return errors.Join(errs...)
}

var (
	repoRef    = regexp.MustCompile(`^gh:([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)$`)
	projectRef = regexp.MustCompile(`^gl:((?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)$`)
)

// ParseRef reads a project reference, gh:owner/repo or gl:group/project, into a Project with Ref, Forge, Owner and
// Repo set.
func ParseRef(ref string) (Project, error) {
	p := Project{Ref: ref}
	err := p.parseRef()
	return p, err
}

func (p *Project) parseRef() error {
	if strings.HasPrefix(p.Ref, "gl:") {
		m := projectRef.FindStringSubmatch(p.Ref)
		if m == nil {
			return fmt.Errorf("%q is not of the form gl:group/project or gl:group/sub/project", p.Ref)
		}
		p.Forge, p.Owner, p.Repo = "gl", m[1], m[2]
		return nil
	}
	m := repoRef.FindStringSubmatch(p.Ref)
	if m == nil {
		return fmt.Errorf("%q is not of the form gh:owner/repo or gl:group/project", p.Ref)
	}
	p.Forge, p.Owner, p.Repo = "gh", m[1], m[2]
	return nil
}

// Uses reports whether a configured project is on the forge ("gh" or "gl").
func (c *Config) Uses(forge string) bool {
	for _, p := range c.Projects {
		if p.Forge == forge {
			return true
		}
	}
	return false
}

// AgentTokenRef is the token the agent uses for p: the project's override, else its forge's token_ref.
func (c *Config) AgentTokenRef(p Project) string {
	if p.TokenRef != "" {
		return p.TokenRef
	}
	if p.Forge == "gl" {
		return c.Forges.GitLab.TokenRef
	}
	return c.Forges.GitHub.TokenRef
}

// AgentIdentity is the login the agent posts as on p: the project's override, else its forge's agent_identity.
func (c *Config) AgentIdentity(p Project) string {
	if p.AgentIdentity != "" {
		return p.AgentIdentity
	}
	if p.Forge == "gl" {
		return c.Forges.GitLab.AgentIdentity
	}
	return c.Forges.GitHub.AgentIdentity
}

// InboxTokenRef is the forge's inbox token, which no project overrides.
func (c *Config) InboxTokenRef(forge string) string {
	if forge == "gl" {
		return c.Forges.GitLab.InboxTokenRef
	}
	return c.Forges.GitHub.InboxTokenRef
}

// TrustedAuthorsOf lists the trusted logins of the forge.
func (c *Config) TrustedAuthorsOf(forge string) []string {
	if forge == "gl" {
		return c.TrustedAuthors.GitLab
	}
	return c.TrustedAuthors.GitHub
}

// ProjectsFor returns the projects assigned to host.
func (c *Config) ProjectsFor(host string) []Project {
	var out []Project
	for _, p := range c.Projects {
		if p.Host == host {
			out = append(out, p)
		}
	}
	return out
}

// deniedCommands are the commands every run refuses; an extra allow rule may not name them.
var deniedCommands = [][]string{{"git", "push"}, {"git", "rebase"}, {"gh"}}

// checkAllowTools refuses the extra allow rules under key that are blank, allow every command, or name what every
// run refuses. A broader rule such as Bash(git *) passes: the deny rules of the run still take precedence over it.
func checkAllowTools(key string, rules []string) []error {
	var errs []error
	for _, r := range rules {
		rule := strings.TrimSpace(r)
		bad := ""
		switch {
		case rule == "":
			bad = "is blank"
		case strings.HasPrefix(rule, "WebFetch"):
			bad = "allows WebFetch, which every run refuses"
		case rule == "Bash":
			bad = "allows every command"
		case strings.HasPrefix(rule, "Bash(") && strings.HasSuffix(rule, ")"):
			cmd := strings.TrimSuffix(strings.TrimPrefix(rule, "Bash("), ")")
			for _, suffix := range []string{":*", " *", "*"} {
				cmd = strings.TrimSuffix(cmd, suffix)
			}
			words := strings.Fields(cmd)
			if len(words) == 0 {
				bad = "allows every command"
				break
			}
			for _, d := range deniedCommands {
				if len(words) >= len(d) && slices.Equal(words[:len(d)], d) {
					bad = fmt.Sprintf("allows %s, which every run refuses", strings.Join(d, " "))
				}
			}
		}
		if bad != "" {
			errs = append(errs, fmt.Errorf("%s: %q %s", key, r, bad))
		}
	}
	return errs
}
