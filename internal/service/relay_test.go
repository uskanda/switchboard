package service

import (
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
)

func relayCfg() *config.Config {
	c := cfg()
	c.Agent.Relay = &config.AgentRelay{URL: "http://127.0.0.1:8782/events", TokenRef: "switchboard/relay-ubuntu-a"}
	c.Relay.Hooks = []config.Hook{
		{Project: "gl:nas/a", Key: "k1", SecretRef: "switchboard/relay-hook-k1"},
		{Project: "gh:o/r", Key: "k2", SecretRef: "switchboard/relay-hook-k2"},
	}
	return c
}

// The agent's service also loads the relay bearer token; the inbox tokens stay out.
func TestAgentCredentialsIncludeTheRelayToken(t *testing.T) {
	var refs []string
	for _, c := range Credentials(relayCfg()) {
		refs = append(refs, c.Ref)
	}
	got := strings.Join(refs, " ")
	if !strings.Contains(got, "switchboard/relay-ubuntu-a") || strings.Contains(got, "inbox") || strings.Contains(got, "relay-hook") {
		t.Fatalf("credentials = %s", got)
	}
}

// The relay's service loads the webhook secrets only: no forge token.
func TestRelayCredentialsAreTheWebhookSecrets(t *testing.T) {
	var refs []string
	for _, c := range RelayCredentials(relayCfg()) {
		refs = append(refs, c.Ref+"="+c.Name)
	}
	if got := strings.Join(refs, " "); got != "switchboard/relay-hook-k1=switchboard_relay_hook_k1 switchboard/relay-hook-k2=switchboard_relay_hook_k2" {
		t.Fatalf("credentials = %s", got)
	}
}

func relaySetup() Setup {
	s := setup()
	s.Kind = KindRelay
	s.Creds = RelayCredentials(relayCfg())
	return s
}

func TestRelayUnit(t *testing.T) {
	u := relaySetup().Unit()
	for _, want := range []string{
		"Description=switchboard relay\n",
		"ExecStart=/home/kanda/.local/bin/switchboard relay serve --config /home/kanda/.config/switchboard/config.yaml\n",
		"LoadCredentialEncrypted=switchboard_relay_hook_k1:/etc/credstore.encrypted/switchboard.switchboard_relay_hook_k1\n",
		"Restart=on-failure\n",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("the unit lacks %q:\n%s", want, u)
		}
	}
	if strings.Contains(u, "agent run") {
		t.Errorf("the relay unit runs the agent:\n%s", u)
	}
}

func TestRelayStepsUseTheRelayUnit(t *testing.T) {
	steps, err := relaySetup().InstallSteps("/tmp/unit", tokens)
	if err != nil {
		t.Fatal(err)
	}
	var shown []string
	for _, s := range steps {
		shown = append(shown, s.String())
	}
	out := strings.Join(shown, "\n")
	for _, want := range []string{
		"sudo install -m 0644 /tmp/unit /etc/systemd/system/switchboard-relay.service",
		"sudo systemctl enable switchboard-relay.service",
		"sudo systemctl restart switchboard-relay.service",
		"sudo systemd-creds encrypt --name=switchboard_relay_hook_k1 - /etc/credstore.encrypted/switchboard.switchboard_relay_hook_k1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the steps lack %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "switchboard-agent.service") {
		t.Errorf("the relay steps touch the agent:\n%s", out)
	}
	var un []string
	for _, s := range relaySetup().UninstallSteps() {
		un = append(un, s.String())
	}
	if got := strings.Join(un, "\n"); !strings.Contains(got, "sudo systemctl disable --now switchboard-relay.service") || !strings.Contains(got, "rm -f /etc/systemd/system/switchboard-relay.service") {
		t.Errorf("uninstall:\n%s", got)
	}
}
