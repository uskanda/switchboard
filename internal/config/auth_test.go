package config

import (
	"strings"
	"testing"
)

const inboxAuth = `inbox:
  auth:
    client_id: abc123
    client_secret_ref: switchboard/gitlab-oidc
    member_groups: [team]
    admin_groups: [team/leads]
    owner: alice
`

func TestInboxAuthIsOptional(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Inbox.Auth != nil {
		t.Errorf("inbox.auth = %+v, want nil", c.Inbox.Auth)
	}
}

func TestInboxAuthIsRead(t *testing.T) {
	c, err := Load(write(t, withBot(withGitLab(valid, "gl:team/app"))+inboxAuth))
	if err != nil {
		t.Fatal(err)
	}
	a := c.Inbox.Auth
	if a == nil || a.ClientID != "abc123" || a.ClientSecretRef != "switchboard/gitlab-oidc" || a.Owner != "alice" ||
		len(a.MemberGroups) != 1 || a.MemberGroups[0] != "team" || len(a.AdminGroups) != 1 || a.AdminGroups[0] != "team/leads" {
		t.Fatalf("inbox.auth = %+v", a)
	}
}

func TestInboxAuthNeedsGitLab(t *testing.T) {
	_, err := Load(write(t, valid+inboxAuth))
	if err == nil || !strings.Contains(err.Error(), "inbox.auth needs forges.gitlab") {
		t.Fatalf("err = %v, want inbox.auth refused without GitLab", err)
	}
}

func TestInboxAuthNeedsTheApplication(t *testing.T) {
	for want, drop := range map[string]string{
		"inbox.auth.client_id":         "    client_id: abc123\n",
		"inbox.auth.client_secret_ref": "    client_secret_ref: switchboard/gitlab-oidc\n",
	} {
		body := withBot(withGitLab(valid, "gl:team/app")) + strings.Replace(inboxAuth, drop, "", 1)
		if _, err := Load(write(t, body)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want %s required", err, want)
		}
	}
}

func TestInboxAuthNeedsSomeoneToLogIn(t *testing.T) {
	body := withBot(withGitLab(valid, "gl:team/app")) + "inbox:\n  auth:\n    client_id: abc123\n    client_secret_ref: switchboard/gitlab-oidc\n"
	_, err := Load(write(t, body))
	if err == nil || !strings.Contains(err.Error(), "member_groups, admin_groups or owner") {
		t.Fatalf("err = %v, want inbox.auth refused with nobody to log in", err)
	}
}

// withBot moves the agent of both forges to bot, so that it is no one in trusted_authors.
func withBot(body string) string {
	return strings.ReplaceAll(body, "agent_identity: alice", "agent_identity: bot")
}

func TestInboxAuthNeedsABotAgent(t *testing.T) {
	// valid's GitHub agent and withGitLab's GitLab agent are alice, who is trusted on both forges.
	_, err := Load(write(t, withGitLab(valid, "gl:team/app")+inboxAuth))
	if err == nil || !strings.Contains(err.Error(), "agent_identity") || !strings.Contains(err.Error(), "trusted_authors") {
		t.Fatalf("err = %v, want the agent refused as a trusted author", err)
	}
	if _, err := Load(write(t, withBot(withGitLab(valid, "gl:team/app"))+inboxAuth)); err != nil {
		t.Fatalf("with a bot agent: %v", err)
	}
}

func TestInboxAuthChecksTheAgentOfEachProject(t *testing.T) {
	body := withBot(withGitLab(valid, "gl:team/app")) + "    token_ref: switchboard/gitlab-app\n    agent_identity: alice\n" + inboxAuth
	_, err := Load(write(t, body))
	if err == nil || !strings.Contains(err.Error(), "projects[") {
		t.Fatalf("err = %v, want the project's agent refused", err)
	}
}

func TestWithoutInboxAuthTheAgentMayBeTrusted(t *testing.T) {
	if _, err := Load(write(t, withGitLab(valid, "gl:team/app"))); err != nil {
		t.Fatalf("without inbox.auth: %v", err)
	}
}
