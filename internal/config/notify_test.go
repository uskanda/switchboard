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
