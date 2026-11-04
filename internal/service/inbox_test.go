package service

import (
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/config"
)

func refsOf(creds []Credential) string {
	var refs []string
	for _, c := range creds {
		refs = append(refs, c.Ref)
	}
	return strings.Join(refs, " ")
}

// The inbox service loads each forge's inbox token, and no agent token.
func TestInboxCredentialsAreTheInboxTokens(t *testing.T) {
	if got := refsOf(InboxCredentials(cfg())); got != "switchboard/github-inbox switchboard/gitlab-inbox" {
		t.Fatalf("credentials = %s", got)
	}
}

// With inbox.relay, the inbox service also loads its bearer token for the relay.
func TestInboxCredentialsIncludeTheRelayToken(t *testing.T) {
	c := cfg()
	c.Inbox.Relay = &config.AgentRelay{URL: "http://127.0.0.1:8782/events", TokenRef: "switchboard/relay-ubuntu-a-inbox"}
	if got := refsOf(InboxCredentials(c)); got != "switchboard/github-inbox switchboard/gitlab-inbox switchboard/relay-ubuntu-a-inbox" {
		t.Fatalf("credentials = %s", got)
	}
}

// With inbox.notify.fcm, the inbox service also loads the Firebase service account key; the agent's service does not.
func TestInboxCredentialsIncludeTheFCMKey(t *testing.T) {
	c := cfg()
	c.Inbox.Notify.FCM = &config.FCM{ProjectID: "p", CredentialsRef: "switchboard/fcm"}
	if got := refsOf(InboxCredentials(c)); got != "switchboard/github-inbox switchboard/gitlab-inbox switchboard/fcm" {
		t.Fatalf("credentials = %s", got)
	}
	if got := refsOf(Credentials(c)); strings.Contains(got, "switchboard/fcm") {
		t.Fatalf("the agent's credentials hold the FCM key: %s", got)
	}
}

// With inbox.auth, the inbox service also loads the client secret of the GitLab login; the agent's service does not.
func TestInboxCredentialsIncludeTheClientSecret(t *testing.T) {
	c := cfg()
	c.Inbox.Auth = &config.InboxAuth{ClientID: "cid", ClientSecretRef: "switchboard/gitlab-oidc", MemberGroups: []string{"team"}}
	if got := refsOf(InboxCredentials(c)); got != "switchboard/github-inbox switchboard/gitlab-inbox switchboard/gitlab-oidc" {
		t.Fatalf("credentials = %s", got)
	}
	if got := refsOf(Credentials(c)); strings.Contains(got, "switchboard/gitlab-oidc") {
		t.Fatalf("the agent's credentials hold the client secret: %s", got)
	}
}

// The agent's service loads the Telegram bot token when it notifies; the inbox tokens stay out.
func TestAgentCredentialsIncludeTheTelegramToken(t *testing.T) {
	c := cfg()
	c.Notify.Telegram = &config.Telegram{ChatID: "1", TokenRef: "switchboard/telegram"}
	got := refsOf(Credentials(c))
	if !strings.Contains(got, "switchboard/telegram") || strings.Contains(got, "inbox") {
		t.Fatalf("credentials = %s", got)
	}
}

func TestInboxUnit(t *testing.T) {
	s := setup()
	s.Kind, s.Creds = KindInbox, InboxCredentials(cfg())
	u := s.Unit()
	for _, want := range []string{
		"Description=switchboard inbox\n",
		"ExecStart=/home/kanda/.local/bin/switchboard inbox serve --config /home/kanda/.config/switchboard/config.yaml\n",
		"RuntimeDirectory=switchboard\nRuntimeDirectoryMode=0700\n",
		"LoadCredentialEncrypted=switchboard_github_inbox:/etc/credstore.encrypted/switchboard.switchboard_github_inbox\n",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("the unit lacks %q:\n%s", want, u)
		}
	}
	if s.UnitName() != "switchboard-inbox.service" || s.UnitPath() != "/etc/systemd/system/switchboard-inbox.service" {
		t.Fatalf("unit = %s %s", s.UnitName(), s.UnitPath())
	}
}

func TestWebUnit(t *testing.T) {
	s := Setup{User: "kanda", Home: "/home/kanda", Path: "/usr/bin", Kind: KindWeb,
		Binary: "/home/kanda/.local/bin/switchboard-web", Config: "/home/kanda/.config/switchboard/web.yaml"}
	u := s.Unit()
	for _, want := range []string{
		"Description=switchboard web inbox\n",
		"After=network-online.target switchboard-inbox.service\nWants=network-online.target switchboard-inbox.service\n",
		"ExecStart=/home/kanda/.local/bin/switchboard-web serve --config /home/kanda/.config/switchboard/web.yaml\n",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("the unit lacks %q:\n%s", want, u)
		}
	}
	// Its own RuntimeDirectory would be removed with it when it stops, taking the inbox's socket along.
	for _, not := range []string{"RuntimeDirectory", "LoadCredentialEncrypted"} {
		if strings.Contains(u, not) {
			t.Errorf("the web unit has %s:\n%s", not, u)
		}
	}
	if s.UnitName() != "switchboard-web.service" {
		t.Fatalf("unit = %s", s.UnitName())
	}
}
