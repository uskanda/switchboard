package config

import (
	"fmt"
	"net/url"
)

// Notify is where the agent tells the user that an issue is waiting or done. Without Telegram nothing is sent.
type Notify struct {
	Telegram *Telegram `yaml:"telegram"`
	// WebURL is the web inbox, linked from each notification when set.
	WebURL string `yaml:"web_url"`
}

// Telegram names the chat to send to and the token_ref of the bot's token.
type Telegram struct {
	ChatID   string `yaml:"chat_id"`
	TokenRef string `yaml:"token_ref"`
}

func (c *Config) validateNotify() []error {
	var errs []error
	if t := c.Notify.Telegram; t != nil && (t.ChatID == "" || t.TokenRef == "") {
		errs = append(errs, fmt.Errorf("notify.telegram needs both chat_id and token_ref"))
	}
	if u := c.Notify.WebURL; u != "" {
		p, err := url.Parse(u)
		if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Host == "" {
			errs = append(errs, fmt.Errorf("notify.web_url must be an absolute http or https URL, got %q", u))
		}
	}
	return errs
}
