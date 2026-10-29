package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/cli"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge/github"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/secret"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

func init() {
	cli.Register("labels sync", labelsSync)
	cli.Register("agent run", agentRun)
	cli.Register("agent status", agentStatus)
}

// parse parses the options of a subcommand: --config and those added by more. It returns the configuration, or an
// exit code when parsing or loading failed.
func parse(name string, args []string, stderr io.Writer, more func(*flag.FlagSet)) (*config.Config, *flag.FlagSet, int) {
	fs := flag.NewFlagSet("switchboard "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", config.DefaultPath(), "configuration file")
	if more != nil {
		more(fs)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, nil, 0
		}
		return nil, nil, 2
	}
	cfg, err := config.Load(*path)
	if err != nil {
		fmt.Fprintf(stderr, "switchboard: %v\n", err)
		return nil, nil, 1
	}
	return cfg, fs, -1
}

// agentForge connects to GitHub with the agent token.
func agentForge(cfg *config.Config) (*github.Client, string, error) {
	r := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}
	token, err := r.Token(cfg.Forges.GitHub.TokenRef)
	if err != nil {
		return nil, "", err
	}
	return github.New(token), token, nil
}

func stateDir() (state.Dir, error) {
	d := state.Dir{Path: state.DefaultDir()}
	return d, d.Ensure()
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "switchboard: %v\n", err)
	return 1
}

func labelsSync(args []string, stdout, stderr io.Writer) int {
	cfg, _, code := parse("labels sync", args, stderr, nil)
	if code >= 0 {
		return code
	}
	f, _, err := agentForge(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	n, err := labels.Sync(context.Background(), f, cfg.Projects, cfg.Labels)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "created %d label(s) in %d project(s)\n", n, len(cfg.Projects))
	return 0
}

func agentRun(args []string, stdout, stderr io.Writer) int {
	cfg, _, code := parse("agent run", args, stderr, nil)
	if code >= 0 {
		return code
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	f, token, err := agentForge(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	d, err := stateDir()
	if err != nil {
		return fail(stderr, err)
	}
	audit := d.Audit()
	j, err := trust.Resolve(ctx, f, d, cfg.TrustedAuthors.GitHub, cfg.Forges.GitHub.AgentIdentity, audit)
	if err != nil {
		return fail(stderr, err)
	}
	a := &agent.Agent{Config: cfg, Forge: f, Judge: j, State: d, Audit: audit, GitToken: token, Log: stderr}
	if err := a.Run(ctx); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func agentStatus(args []string, stdout, stderr io.Writer) int {
	cfg, _, code := parse("agent status", args, stderr, nil)
	if code >= 0 {
		return code
	}
	if err := agent.Status(cfg, state.Dir{Path: state.DefaultDir()}, stdout); err != nil {
		return fail(stderr, err)
	}
	return 0
}
