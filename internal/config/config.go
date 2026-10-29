// Package config loads and validates the switchboard configuration file.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
}

// Host names this machine; projects whose host matches run here.
type Host struct {
	Name string `yaml:"name"`
}

// Secrets controls where tokens may come from.
type Secrets struct {
	AllowEnv bool `yaml:"allow_env"`
}

// Forges holds the per-forge settings. Only GitHub exists in this version.
type Forges struct {
	GitHub GitHub `yaml:"github"`
}

// GitHub is the GitHub forge setting.
type GitHub struct {
	TokenRef      string `yaml:"token_ref"`
	InboxTokenRef string `yaml:"inbox_token_ref"`
	AgentIdentity string `yaml:"agent_identity"`
	SearchScope   string `yaml:"search_scope"`
}

// TrustedAuthors lists, per forge, the logins whose input is taken as an instruction.
type TrustedAuthors struct {
	GitHub []string `yaml:"github"`
}

// Labels are the names of the four state labels.
type Labels struct {
	Ready   string `yaml:"ready"`
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

// Agent controls the polling loop.
type Agent struct {
	PollInterval time.Duration `yaml:"poll_interval"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxParallel  int           `yaml:"max_parallel"`
	MaxTurns     int           `yaml:"max_turns"`
}

// Project is one repository. Ref is the configured "gh:owner/repo"; Forge, Owner and Repo are parsed from it.
type Project struct {
	Ref                 string   `yaml:"ref"`
	Path                string   `yaml:"path"`
	Host                string   `yaml:"host"`
	BaseBranch          string   `yaml:"base_branch"`
	VerifyCommand       string   `yaml:"verify_command"`
	IncludeUntrusted    bool     `yaml:"include_untrusted"`
	ProtectedPathsExtra []string `yaml:"protected_paths_extra"`

	Forge string `yaml:"-"`
	Owner string `yaml:"-"`
	Repo  string `yaml:"-"`
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

// tokenLike matches values that look like a forge token. Such a value never belongs in the file.
var tokenLike = regexp.MustCompile(`^(ghp_|github_pat_|gho_|ghs_|ghu_|ghr_|glpat-)`)

// Load reads, validates and completes the configuration at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
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
	c.applyDefaults()
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
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

func (c *Config) applyDefaults() {
	set := func(s *string, v string) {
		if *s == "" {
			*s = v
		}
	}
	set(&c.Labels.Ready, "agent:ready")
	set(&c.Labels.Running, "agent:running")
	set(&c.Labels.Waiting, "agent:waiting")
	set(&c.Labels.Done, "agent:done")
	set(&c.Comments.Language, "en")
	set(&c.Claude.Binary, "claude")
	set(&c.Claude.Auth, "subscription")
	if c.Agent.PollInterval == 0 {
		c.Agent.PollInterval = 60 * time.Second
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
	for i := range c.Projects {
		set(&c.Projects[i].BaseBranch, "main")
		c.Projects[i].Path = expandHome(c.Projects[i].Path)
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
	need(c.Forges.GitHub.TokenRef, "forges.github.token_ref")
	need(c.Forges.GitHub.AgentIdentity, "forges.github.agent_identity")
	if len(c.TrustedAuthors.GitHub) == 0 {
		errs = append(errs, errors.New("trusted_authors.github needs at least one login"))
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
	}
	return errors.Join(errs...)
}

var repoRef = regexp.MustCompile(`^gh:([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)$`)

func (p *Project) parseRef() error {
	if strings.HasPrefix(p.Ref, "gl:") {
		return fmt.Errorf("%s: GitLab is not supported in this version", p.Ref)
	}
	m := repoRef.FindStringSubmatch(p.Ref)
	if m == nil {
		return fmt.Errorf("%q is not of the form gh:owner/repo", p.Ref)
	}
	p.Forge, p.Owner, p.Repo = "github", m[1], m[2]
	return nil
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
