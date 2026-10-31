package config

import (
	"strings"
	"testing"
	"time"
)

const relayHash = "sha256:" + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// withRelay adds a GitLab project and a relay section to the valid configuration.
func withRelay(extra string) string {
	return withGitLab(valid, "gl:nas/switchboard-sandbox") + `relay:
  hooks:
    - project: gl:nas/switchboard-sandbox
      key: Zm9vYmFyYmF6cXV4cXV1eDEyMzQ1
      secret_ref: switchboard/relay-hook-1
  hosts:
    - name: ubuntu-a
      token_hash: ` + relayHash + "\n" + extra
}

func TestRelayIsOptional(t *testing.T) {
	cfg, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.Relay != nil || len(cfg.Relay.Hooks) != 0 {
		t.Fatalf("relay = %+v, agent.relay = %+v", cfg.Relay, cfg.Agent.Relay)
	}
}

func TestRelayDefaultsAndHooks(t *testing.T) {
	cfg, err := Load(write(t, withRelay("")))
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.Relay
	if r.WebhookListen != "127.0.0.1:8781" || r.SSEListen != "127.0.0.1:8782" || r.WebhookURL != "http://127.0.0.1:8781" {
		t.Fatalf("defaults: %+v", r)
	}
	h := r.Hooks[0]
	if h.Forge != "gl" || h.Owner != "nas" || h.Repo != "switchboard-sandbox" || h.GitLabToken {
		t.Fatalf("hook: %+v", h)
	}
	if got, ok := r.HookByKey("Zm9vYmFyYmF6cXV4cXV1eDEyMzQ1"); !ok || got.Project != "gl:nas/switchboard-sandbox" {
		t.Fatalf("HookByKey: %+v %v", got, ok)
	}
}

func TestRelayRejects(t *testing.T) {
	base := withRelay("")
	for name, c := range map[string]struct{ body, want string }{
		"duplicate key":          {strings.Replace(base, "  hosts:\n", "    - project: gh:alice/sandbox\n      key: Zm9vYmFyYmF6cXV4cXV1eDEyMzQ1\n      secret_ref: switchboard/relay-hook-2\n  hosts:\n", 1), "used twice"},
		"unknown project":        {strings.Replace(base, "project: gl:nas/switchboard-sandbox", "project: gl:nas/elsewhere", 1), "not a configured project"},
		"short key":              {strings.Replace(base, "Zm9vYmFyYmF6cXV4cXV1eDEyMzQ1", "abc", 1), "key"},
		"bare hash":              {strings.Replace(base, relayHash, strings.TrimPrefix(relayHash, "sha256:"), 1), "token_hash"},
		"no secret_ref":          {strings.Replace(base, "      secret_ref: switchboard/relay-hook-1\n", "", 1), "secret_ref"},
		"gitlab_token on github": {strings.Replace(base, "    - project: gl:nas/switchboard-sandbox\n", "    - project: gh:alice/sandbox\n      gitlab_token: true\n", 1), "gitlab_token"},
	} {
		_, err := Load(write(t, c.body))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want %q", name, err, c.want)
		}
	}
}

func TestAgentRelayMode(t *testing.T) {
	body := strings.Replace(valid, "projects:\n", "agent:\n  poll_interval: 30s\n  relay:\n    url: http://127.0.0.1:8782/events\n    token_ref: switchboard/relay-ubuntu-a\nprojects:\n", 1)
	cfg, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.Relay == nil || cfg.Agent.Relay.URL != "http://127.0.0.1:8782/events" || cfg.Agent.Relay.TokenRef != "switchboard/relay-ubuntu-a" {
		t.Fatalf("agent.relay = %+v", cfg.Agent.Relay)
	}
	if cfg.Agent.ReconcileInterval != 10*time.Minute {
		t.Fatalf("reconcile_interval = %s", cfg.Agent.ReconcileInterval)
	}
	short := strings.Replace(body, "  relay:\n", "  reconcile_interval: 10s\n  relay:\n", 1)
	if _, err := Load(write(t, short)); err == nil || !strings.Contains(err.Error(), "reconcile_interval") {
		t.Fatalf("a reconcile_interval below poll_interval: %v", err)
	}
	noRef := strings.Replace(body, "    token_ref: switchboard/relay-ubuntu-a\n", "", 1)
	if _, err := Load(write(t, noRef)); err == nil || !strings.Contains(err.Error(), "agent.relay.token_ref") {
		t.Fatalf("agent.relay without token_ref: %v", err)
	}
}

func TestRelayCovers(t *testing.T) {
	cfg, err := Load(write(t, withRelay("")))
	if err != nil {
		t.Fatal(err)
	}
	for ref, want := range map[string]bool{
		"gl:nas/switchboard-sandbox": true,
		"gl:NAS/Switchboard-Sandbox": true, // forge paths are case-insensitive
		"gh:alice/sandbox":           false,
		"gl:nas/elsewhere":           false,
	} {
		if got := cfg.Relay.Covers(ref); got != want {
			t.Errorf("Covers(%q) = %v, want %v", ref, got, want)
		}
	}
}
