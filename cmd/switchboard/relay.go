package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/uskanda/switchboard/internal/cli"
	"github.com/uskanda/switchboard/internal/relay"
	"github.com/uskanda/switchboard/internal/secret"
)

func init() {
	cli.Register("relay serve", relayServe)
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
