package config

import (
	"strings"
	"testing"
)

const notifyTelegram = `notify:
  telegram:
    chat_id: "-1001234567890"
    token_ref: switchboard/telegram
`

func TestNotifyIsOptional(t *testing.T) {
	cfg, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Notify.Telegram != nil || cfg.Notify.WebURL != "" {
		t.Fatalf("notify = %+v", cfg.Notify)
	}
}

func TestNotifyTelegram(t *testing.T) {
	cfg, err := Load(write(t, valid+notifyTelegram+"  web_url: http://192.168.0.82:8783\n"))
	if err != nil {
		t.Fatal(err)
	}
	tg := cfg.Notify.Telegram
	if tg == nil || tg.ChatID != "-1001234567890" || tg.TokenRef != "switchboard/telegram" {
		t.Fatalf("telegram = %+v", tg)
	}
	if cfg.Notify.WebURL != "http://192.168.0.82:8783" {
		t.Fatalf("web_url = %q", cfg.Notify.WebURL)
	}
}

func TestNotifyRejects(t *testing.T) {
	for name, c := range map[string]struct{ body, want string }{
		"chat_id only":   {valid + "notify:\n  telegram:\n    chat_id: \"42\"\n", "notify.telegram"},
		"token_ref only": {valid + "notify:\n  telegram:\n    token_ref: switchboard/telegram\n", "notify.telegram"},
		"relative url":   {valid + notifyTelegram + "  web_url: /inbox\n", "notify.web_url"},
		"ftp url":        {valid + notifyTelegram + "  web_url: ftp://192.168.0.82/\n", "notify.web_url"},
		"bot token":      {valid + strings.Replace(notifyTelegram, "switchboard/telegram", "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0", 1), "looks like a token"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(write(t, c.body))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

const inboxFCM = `inbox:
  notify:
    fcm:
      project_id: switchboard-app
      credentials_ref: switchboard/fcm
`

func TestInboxNotifyFCM(t *testing.T) {
	cfg, err := Load(write(t, valid+inboxFCM))
	if err != nil {
		t.Fatal(err)
	}
	f := cfg.Inbox.Notify.FCM
	if f == nil || f.ProjectID != "switchboard-app" || f.CredentialsRef != "switchboard/fcm" {
		t.Fatalf("fcm = %+v", f)
	}
}

func TestInboxNotifyFCMIsOptional(t *testing.T) {
	cfg, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Inbox.Notify.FCM != nil {
		t.Fatalf("fcm = %+v, want none", cfg.Inbox.Notify.FCM)
	}
}

func TestInboxNotifyFCMRejects(t *testing.T) {
	for name, c := range map[string]struct{ body, want string }{
		"project_id only":      {valid + "inbox:\n  notify:\n    fcm:\n      project_id: p\n", "credentials_ref"},
		"credentials_ref only": {valid + "inbox:\n  notify:\n    fcm:\n      credentials_ref: switchboard/fcm\n", "project_id"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(write(t, c.body))
			if err == nil || !strings.Contains(err.Error(), "inbox.notify.fcm") || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want inbox.notify.fcm and %q", err, c.want)
			}
		})
	}
}
