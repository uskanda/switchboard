package relay

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/fake"
)

type memStore map[string]string

func (m memStore) Set(ref, v string) error { m[ref] = v; return nil }
func (m memStore) Delete(ref string) error { delete(m, ref); return nil }

func adminConfig() *config.Config {
	return &config.Config{
		Projects: []config.Project{
			{Ref: "gl:nas/switchboard-sandbox", Forge: "gl", Owner: "nas", Repo: "switchboard-sandbox"},
			{Ref: "gh:alice/sandbox", Forge: "gh", Owner: "alice", Repo: "sandbox"},
		},
		Relay: config.Relay{WebhookURL: "http://127.0.0.1:8781", SSEListen: "127.0.0.1:8782"},
	}
}

func TestAddHookRegistersWithoutShowingTheSecret(t *testing.T) {
	hooks, store, out := &fake.Hooks{}, memStore{}, &bytes.Buffer{}
	if err := AddHook(context.Background(), adminConfig(), "gl:nas/switchboard-sandbox", false, hooks, store, out); err != nil {
		t.Fatal(err)
	}
	got := hooks.Of("nas", "switchboard-sandbox")
	if len(got) != 1 || !regexp.MustCompile(`^http://127\.0\.0\.1:8781/webhook/gitlab/[A-Za-z0-9_-]{32}$`).MatchString(got[0].URL) {
		t.Fatalf("hooks = %+v", got)
	}
	if !strings.HasPrefix(got[0].Secret, "whsec_") || got[0].SecretToken {
		t.Fatalf("a GitLab hook without a signing token: %+v", got[0])
	}
	key := got[0].URL[strings.LastIndex(got[0].URL, "/")+1:]
	ref := "switchboard/relay-hook-" + key
	if store[ref] != got[0].Secret {
		t.Fatalf("the secret stored under %s is not the one registered", ref)
	}
	s := out.String()
	if strings.Contains(s, got[0].Secret) {
		t.Fatalf("the secret is shown:\n%s", s)
	}
	for _, want := range []string{"- project: gl:nas/switchboard-sandbox", "key: " + key, "secret_ref: " + ref} {
		if !strings.Contains(s, want) {
			t.Errorf("the output lacks %q:\n%s", want, s)
		}
	}
}

func TestAddHookWithoutTheRightShowsTheManualSteps(t *testing.T) {
	for _, c := range []struct {
		project string
		err     error
		want    []string
	}{
		{"gh:alice/sandbox", forge.ErrNotFound, []string{"https://github.com/alice/sandbox/settings/hooks", "application/json", "Issues", "Issue comments"}},
		{"gl:nas/switchboard-sandbox", forge.ErrForbidden, []string{"Settings > Webhooks", "Issues events", "Comments", "signing token", "127.0.0.1:8781", "Outbound requests"}},
	} {
		hooks, store, out := &fake.Hooks{Err: c.err}, memStore{}, &bytes.Buffer{}
		if err := AddHook(context.Background(), adminConfig(), c.project, false, hooks, store, out); err != nil {
			t.Fatalf("%s: %v", c.project, err)
		}
		if len(store) != 1 {
			t.Fatalf("%s: the secret was not kept: %v", c.project, store)
		}
		var secret string
		for _, v := range store {
			secret = v
		}
		s := out.String()
		if strings.Count(s, secret) != 1 {
			t.Errorf("%s: the secret should be shown exactly once:\n%s", c.project, s)
		}
		for _, want := range append(c.want, "- project: "+c.project) {
			if !strings.Contains(s, want) {
				t.Errorf("%s: the output lacks %q:\n%s", c.project, want, s)
			}
		}
	}
}

func TestAddHookFailureDropsTheSecret(t *testing.T) {
	hooks, store := &fake.Hooks{Err: errors.New("gitlab: POST hooks: 422 Invalid url given")}, memStore{}
	err := AddHook(context.Background(), adminConfig(), "gl:nas/switchboard-sandbox", false, hooks, store, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "Outbound requests") || len(store) != 0 {
		t.Fatalf("err = %v, store = %v", err, store)
	}
}

func TestAddHookRefusals(t *testing.T) {
	cfg := adminConfig()
	if err := AddHook(context.Background(), cfg, "gl:nas/elsewhere", false, &fake.Hooks{}, memStore{}, &bytes.Buffer{}); err == nil {
		t.Error("an unconfigured project was accepted")
	}
	if err := AddHook(context.Background(), cfg, "gh:alice/sandbox", true, &fake.Hooks{}, memStore{}, &bytes.Buffer{}); err == nil {
		t.Error("--gitlab-token on a GitHub project was accepted")
	}
	cfg.Relay.Hooks = []config.Hook{{Project: "gh:alice/sandbox", Key: "k", SecretRef: "r"}}
	if err := AddHook(context.Background(), cfg, "gh:alice/sandbox", false, &fake.Hooks{}, memStore{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "already") {
		t.Errorf("a second webhook for a project: %v", err)
	}
}

func TestAddHookWithASecretToken(t *testing.T) {
	hooks := &fake.Hooks{}
	if err := AddHook(context.Background(), adminConfig(), "gl:nas/switchboard-sandbox", true, hooks, memStore{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	h := hooks.Of("nas", "switchboard-sandbox")[0]
	if !h.SecretToken || strings.HasPrefix(h.Secret, "whsec_") {
		t.Fatalf("hook = %+v", h)
	}
}

func TestRemoveHook(t *testing.T) {
	cfg := adminConfig()
	hooks, store, out := &fake.Hooks{}, memStore{}, &bytes.Buffer{}
	if err := AddHook(context.Background(), cfg, "gl:nas/switchboard-sandbox", false, hooks, store, out); err != nil {
		t.Fatal(err)
	}
	h := hooks.Of("nas", "switchboard-sandbox")[0]
	key := h.URL[strings.LastIndex(h.URL, "/")+1:]
	cfg.Relay.Hooks = []config.Hook{{Project: "gl:nas/switchboard-sandbox", Key: key, SecretRef: "switchboard/relay-hook-" + key, Forge: "gl", Owner: "nas", Repo: "switchboard-sandbox"}}
	out.Reset()
	if err := RemoveHook(context.Background(), cfg, "gl:nas/switchboard-sandbox", hooks, store, out); err != nil {
		t.Fatal(err)
	}
	if len(hooks.Of("nas", "switchboard-sandbox")) != 0 || len(store) != 0 {
		t.Fatalf("left: hooks %v, store %v", hooks.Of("nas", "switchboard-sandbox"), store)
	}
	if !strings.Contains(out.String(), "relay.hooks") {
		t.Fatalf("output:\n%s", out.String())
	}
	if err := RemoveHook(context.Background(), adminConfig(), "gl:nas/switchboard-sandbox", hooks, store, out); err == nil {
		t.Error("removing a webhook the configuration does not list was accepted")
	}
}

func TestAddTokenShowsOnlyTheHash(t *testing.T) {
	store, out := memStore{}, &bytes.Buffer{}
	if err := AddToken(adminConfig(), "ubuntu-b", store, out); err != nil {
		t.Fatal(err)
	}
	tok := store["switchboard/relay-ubuntu-b"]
	if len(tok) < 40 {
		t.Fatalf("token %q", tok)
	}
	s := out.String()
	if strings.Contains(s, tok) {
		t.Fatalf("the token is shown:\n%s", s)
	}
	for _, want := range []string{"- name: ubuntu-b", "token_hash: " + hashOf(tok), "url: http://127.0.0.1:8782/events", "token_ref: switchboard/relay-ubuntu-b"} {
		if !strings.Contains(s, want) {
			t.Errorf("the output lacks %q:\n%s", want, s)
		}
	}
	if err := AddToken(adminConfig(), "bad name!", store, out); err == nil {
		t.Error("a host name with a space was accepted")
	}
}
