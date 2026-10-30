package config

import (
	"fmt"
	"regexp"
)

// Relay is the relay's setting: where it listens, the webhooks it accepts and the hosts that may subscribe. It holds
// no secret: each webhook names its secret with secret_ref, and each host is known by the hash of its token.
type Relay struct {
	WebhookListen string      `yaml:"webhook_listen"` // 127.0.0.1:8781 when empty
	SSEListen     string      `yaml:"sse_listen"`     // 127.0.0.1:8782 when empty
	WebhookURL    string      `yaml:"webhook_url"`    // the base URL registered on the forges; http://<webhook_listen> when empty
	Hooks         []Hook      `yaml:"hooks"`
	Hosts         []RelayHost `yaml:"hosts"`
}

// Hook is one project's webhook. Key is the last segment of its path; GitLabToken accepts X-Gitlab-Token instead of
// a signing token, for a GitLab that cannot sign.
type Hook struct {
	Project     string `yaml:"project"`
	Key         string `yaml:"key"`
	SecretRef   string `yaml:"secret_ref"`
	GitLabToken bool   `yaml:"gitlab_token"`

	Forge string `yaml:"-"`
	Owner string `yaml:"-"`
	Repo  string `yaml:"-"`
}

// FullName is "owner/repo".
func (h Hook) FullName() string { return h.Owner + "/" + h.Repo }

// RelayHost is a host allowed to subscribe, by the hash of its bearer token.
type RelayHost struct {
	Name      string `yaml:"name"`
	TokenHash string `yaml:"token_hash"`
}

// HookByKey finds the webhook whose path ends in key.
func (r Relay) HookByKey(key string) (Hook, bool) {
	for _, h := range r.Hooks {
		if h.Key == key {
			return h, true
		}
	}
	return Hook{}, false
}

var (
	hookKey   = regexp.MustCompile(`^[A-Za-z0-9_-]{16,}$`)
	tokenHash = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

func (c *Config) validateRelay() []error {
	var errs []error
	keys := map[string]bool{}
	for i := range c.Relay.Hooks {
		h := &c.Relay.Hooks[i]
		key := fmt.Sprintf("relay.hooks[%d]", i)
		p, ok := c.project(h.Project)
		if !ok {
			errs = append(errs, fmt.Errorf("%s.project: %q is not a configured project", key, h.Project))
		} else {
			h.Forge, h.Owner, h.Repo = p.Forge, p.Owner, p.Repo
		}
		if !hookKey.MatchString(h.Key) {
			errs = append(errs, fmt.Errorf("%s.key must be at least 16 of A-Z a-z 0-9 _ -", key))
		} else if keys[h.Key] {
			errs = append(errs, fmt.Errorf("%s.key is used twice", key))
		}
		keys[h.Key] = true
		if h.SecretRef == "" {
			errs = append(errs, fmt.Errorf("%s.secret_ref is required", key))
		}
		if h.GitLabToken && ok && p.Forge != "gl" {
			errs = append(errs, fmt.Errorf("%s.gitlab_token applies to GitLab projects only", key))
		}
	}
	for i, h := range c.Relay.Hosts {
		key := fmt.Sprintf("relay.hosts[%d]", i)
		if h.Name == "" {
			errs = append(errs, fmt.Errorf("%s.name is required", key))
		}
		if !tokenHash.MatchString(h.TokenHash) {
			errs = append(errs, fmt.Errorf("%s.token_hash must be sha256:<64 hex digits>, the hash `relay token add` prints, never the token", key))
		}
	}
	return errs
}

// project finds the configured project with ref.
func (c *Config) project(ref string) (Project, bool) {
	for _, p := range c.Projects {
		if p.Ref == ref {
			return p, true
		}
	}
	return Project{}, false
}
