package relay

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
)

// SecretWriter stores and removes secrets in the Secret Service.
type SecretWriter interface {
	Set(ref, value string) error
	Delete(ref string) error
}

// random returns n random bytes in unpadded base64url.
func random(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported systems
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func forgePath(kind string) string {
	if kind == "gh" {
		return "github"
	}
	return "gitlab"
}

// AddHook makes a hook key and a secret for project, stores the secret, and registers the webhook with hooks (the
// inbox token). When the token may not, it prints how to register the webhook by hand, with the secret: the only time
// the secret is shown. Either way it ends by printing the entry to add to relay.hooks; it never edits the file.
func AddHook(ctx context.Context, cfg *config.Config, ref string, secretToken bool, hooks forge.Hooks, store SecretWriter, out io.Writer) error {
	p, ok := cfg.Project(ref)
	if !ok {
		return fmt.Errorf("%s is not a configured project", ref)
	}
	if secretToken && p.Forge != "gl" {
		return errors.New("--gitlab-token applies to GitLab projects only")
	}
	for _, h := range cfg.Relay.Hooks {
		if h.Project == ref {
			return fmt.Errorf("%s already has a webhook (key %s); remove it first with `switchboard relay hook remove %s`", ref, h.Key, ref)
		}
	}
	key := random(24)
	secret := random(32)
	if p.Forge == "gl" && !secretToken {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		secret = "whsec_" + base64.StdEncoding.EncodeToString(b) // a signing token (V16)
	}
	secretRef := "switchboard/relay-hook-" + key
	hookURL := cfg.Relay.WebhookURL + "/webhook/" + forgePath(p.Forge) + "/" + key
	if err := store.Set(secretRef, secret); err != nil {
		return fmt.Errorf("store the webhook secret in the Secret Service: %w", err)
	}
	_, err := hooks.CreateHook(ctx, p.Owner, p.Repo, forge.Hook{URL: hookURL, Secret: secret, SecretToken: secretToken})
	switch {
	case err == nil:
		fmt.Fprintf(out, "Registered the webhook of %s: %s\n", ref, hookURL)
		if p.Forge == "gl" {
			fmt.Fprint(out, allowlistNote(cfg))
		}
	case errors.Is(err, forge.ErrForbidden) || errors.Is(err, forge.ErrNotFound):
		fmt.Fprintf(out, "The inbox token may not register webhooks on %s (%v). Register it by hand:\n\n", ref, err)
		manual(out, cfg, p, hookURL, secret, secretToken)
	default:
		_ = store.Delete(secretRef)
		hint := ""
		if p.Forge == "gl" {
			hint = "\n" + allowlistNote(cfg)
		}
		return fmt.Errorf("register the webhook of %s: %w%s", ref, err, hint)
	}
	gitlabToken := ""
	if secretToken {
		gitlabToken = "      gitlab_token: true\n"
	}
	fmt.Fprintf(out, "\nAdd this to relay.hooks in the configuration, then install the relay again (switchboard service install --relay):\n\n"+
		"    - project: %s\n      key: %s\n      secret_ref: %s\n%s", ref, key, secretRef, gitlabToken)
	return nil
}

// allowlistNote tells how to let GitLab send to a relay on its own host or network.
func allowlistNote(cfg *config.Config) string {
	host := cfg.Relay.WebhookURL
	if u, err := url.Parse(cfg.Relay.WebhookURL); err == nil && u.Host != "" {
		host = u.Host
	}
	return fmt.Sprintf("If the relay is on GitLab's host or network, GitLab sends to it only once %s is in "+
		"Admin > Settings > Network > Outbound requests > \"Local IP addresses and domain names that hooks and integrations can access\". "+
		"Keep \"Allow requests to the local network\" off. After a change, GitLab's background jobs may refuse the address for a few minutes.\n", host)
}

func manual(out io.Writer, cfg *config.Config, p config.Project, hookURL, secret string, secretToken bool) {
	if p.Forge == "gh" {
		fmt.Fprintf(out, "  1. Open https://github.com/%s/settings/hooks and choose \"Add webhook\".\n"+
			"  2. Payload URL: %s\n  3. Content type: application/json\n  4. Secret: %s\n"+
			"  5. Choose \"Let me select individual events\" and select only Issues and Issue comments.\n", p.FullName(), hookURL, secret)
		return
	}
	field := "the signing token"
	if secretToken {
		field = "Secret token"
	}
	fmt.Fprintf(out, "  1. Open %s/%s/-/hooks (Settings > Webhooks) and choose \"Add new webhook\".\n"+
		"  2. URL: %s\n  3. Set %s to: %s\n"+
		"  4. Select only Issues events, Confidential issues events, Comments and Confidential comments.\n\n%s",
		cfg.Forges.GitLab.BaseURL, p.FullName(), hookURL, field, secret, allowlistNote(cfg))
}

// RemoveHook removes the webhook of project registered by AddHook and its secret, and prints the entry to drop from
// relay.hooks.
func RemoveHook(ctx context.Context, cfg *config.Config, ref string, hooks forge.Hooks, store SecretWriter, out io.Writer) error {
	var hook config.Hook
	found := false
	for _, h := range cfg.Relay.Hooks {
		if h.Project == ref {
			hook, found = h, true
		}
	}
	if !found {
		return fmt.Errorf("relay.hooks has no webhook for %s", ref)
	}
	hookURL := cfg.Relay.WebhookURL + "/webhook/" + forgePath(hook.Forge) + "/" + hook.Key
	gone, err := hooks.DeleteHook(ctx, hook.Owner, hook.Repo, hookURL)
	switch {
	case err == nil && gone:
		fmt.Fprintf(out, "Removed the webhook of %s.\n", ref)
	case err == nil:
		fmt.Fprintf(out, "%s has no webhook sent to %s.\n", ref, hookURL)
	case errors.Is(err, forge.ErrForbidden) || errors.Is(err, forge.ErrNotFound):
		fmt.Fprintf(out, "The inbox token may not remove webhooks on %s (%v). Remove the one sent to %s by hand.\n", ref, err, hookURL)
	default:
		return fmt.Errorf("remove the webhook of %s: %w", ref, err)
	}
	if err := store.Delete(hook.SecretRef); err != nil {
		return fmt.Errorf("remove the webhook secret %s: %w", hook.SecretRef, err)
	}
	fmt.Fprintf(out, "Removed the secret %s. Remove the entry of %s (key %s) from relay.hooks, then install the relay again.\n", hook.SecretRef, ref, hook.Key)
	return nil
}

var hostName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// AddToken makes the bearer token of host, stores it under switchboard/relay-<host>, and prints the entries to add
// to relay.hosts (its hash) and to the host's agent.relay (its token_ref). The token itself is never shown.
func AddToken(cfg *config.Config, host string, store SecretWriter, out io.Writer) error {
	if !hostName.MatchString(host) {
		return fmt.Errorf("%q is not a host name", host)
	}
	tok := random(32)
	ref := "switchboard/relay-" + host
	if err := store.Set(ref, tok); err != nil {
		return fmt.Errorf("store the token in the Secret Service: %w", err)
	}
	sum := sha256.Sum256([]byte(tok))
	fmt.Fprintf(out, "Stored the token of %s as %s. A token stored there before is replaced.\n\n"+
		"Add this to relay.hosts in the relay's configuration, then install the relay again:\n\n"+
		"    - name: %s\n      token_hash: sha256:%s\n\n"+
		"Add this to agent in the configuration of %s, then install the agent again (switchboard service install):\n\n"+
		"  relay:\n    url: http://%s/events\n    token_ref: %s\n", host, ref, host, hex.EncodeToString(sum[:]), host, cfg.Relay.SSEListen, ref)
	return nil
}
