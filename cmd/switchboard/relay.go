package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/uskanda/switchboard/internal/cli"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/relay"
	"github.com/uskanda/switchboard/internal/secret"
)

func init() {
	cli.Register("relay serve", relayServe)
	cli.Register("relay hook add", relayHookAdd)
	cli.Register("relay hook remove", relayHookRemove)
	cli.Register("relay token add", relayTokenAdd)
}

// keyring is where the relay commands keep secrets; tests replace it.
var keyring relay.SecretWriter = secret.Keyring{}

// hooksFor is the webhook API of a project's forge, reached with the inbox token.
func hooksFor(cfg *config.Config, ref string) (forge.Hooks, error) {
	p, ok := cfg.Project(ref)
	if !ok {
		return nil, fmt.Errorf("%s is not a configured project", ref)
	}
	tokenRef := cfg.InboxTokenRef(p.Forge)
	if tokenRef == "" {
		return nil, fmt.Errorf("the forge of %s has no inbox_token_ref; webhooks are registered with the inbox token", ref)
	}
	f, _, err := connect(cfg).client(p.Forge, tokenRef)
	if err != nil {
		return nil, err
	}
	h, ok := f.(forge.Hooks)
	if !ok {
		return nil, fmt.Errorf("the client of %s cannot manage webhooks", ref)
	}
	return h, nil
}

func relayHookAdd(args []string, stdout, stderr io.Writer) int {
	var gitlabToken bool
	cfg, fs, code := parse("relay hook add", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&gitlabToken, "gitlab-token", false, "send the secret as X-Gitlab-Token, for a GitLab without signing tokens")
	})
	if code >= 0 {
		return code
	} else if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: switchboard relay hook add [--gitlab-token] <project> [--config FILE]")
		return 2
	}
	hooks, err := hooksFor(cfg, fs.Arg(0))
	if err != nil {
		return fail(stderr, err)
	}
	if err := relay.AddHook(context.Background(), cfg, fs.Arg(0), gitlabToken, hooks, keyring, stdout); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func relayHookRemove(args []string, stdout, stderr io.Writer) int {
	cfg, fs, code := parse("relay hook remove", args, stderr, nil)
	if code >= 0 {
		return code
	} else if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: switchboard relay hook remove <project> [--config FILE]")
		return 2
	}
	hooks, err := hooksFor(cfg, fs.Arg(0))
	if err != nil {
		return fail(stderr, err)
	}
	if err := relay.RemoveHook(context.Background(), cfg, fs.Arg(0), hooks, keyring, stdout); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func relayTokenAdd(args []string, stdout, stderr io.Writer) int {
	cfg, fs, code := parse("relay token add", args, stderr, nil)
	if code >= 0 {
		return code
	} else if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: switchboard relay token add <host> [--config FILE]")
		return 2
	}
	if err := relay.AddToken(cfg, fs.Arg(0), keyring, stdout); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func relayServe(args []string, stdout, stderr io.Writer) int {
	cfg, fs, code := parse("relay serve", args, stderr, nil)
	if code >= 0 {
		return code
	} else if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: switchboard relay serve [--config FILE]")
		return 2
	}
	r := cfg.Relay
	if len(r.Hosts) == 0 {
		return fail(stderr, fmt.Errorf("relay.hosts is empty: no agent could subscribe; add one with `switchboard relay token add <host>`"))
	}
	whLn, err := net.Listen("tcp", r.WebhookListen)
	if err != nil {
		return fail(stderr, err)
	}
	sseLn, err := net.Listen("tcp", r.SSEListen)
	if err != nil {
		whLn.Close()
		return fail(stderr, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	res := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}
	logf := func(format string, a ...any) { fmt.Fprintf(stderr, "switchboard: "+format+"\n", a...) }
	if err := relay.Serve(ctx, r, whLn, sseLn, res.Token, logf); err != nil {
		return fail(stderr, err)
	}
	return 0
}
