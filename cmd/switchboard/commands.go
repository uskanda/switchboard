package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/uskanda/switchboard/internal/agent"
	"github.com/uskanda/switchboard/internal/cli"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge/github"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/secret"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/trust"
)

func init() {
	cli.Register("labels sync", labelsSync)
	cli.Register("agent run", agentRun)
	cli.Register("agent status", agentStatus)
	cli.Register("inbox", inboxList)
	cli.Register("show", inboxShow)
	cli.Register("reply", inboxReply)
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
	if err := parseInterspersed(fs, args); err != nil {
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

// parseInterspersed parses fs from args, allowing options after the positional arguments, as in
// `reply 1 "text" --force`. The positional arguments end up in fs.Args(); "--" ends the options.
func parseInterspersed(fs *flag.FlagSet, args []string) error {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		if len(args) > len(rest) && args[len(args)-len(rest)-1] == "--" {
			positional = append(positional, rest...)
			break
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
	return fs.Parse(append([]string{"--"}, positional...))
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

// userInbox connects to GitHub with the user's own token (inbox_token_ref).
func userInbox(cfg *config.Config) (*inbox.Inbox, error) {
	ref := cfg.Forges.GitHub.InboxTokenRef
	if ref == "" {
		return nil, errors.New("forges.github.inbox_token_ref is not set; the inbox reads and replies with your own token")
	}
	token, err := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}.Token(ref)
	if err != nil {
		return nil, err
	}
	return &inbox.Inbox{Config: cfg, Forge: github.New(token), Cache: inbox.DefaultCache(), Now: time.Now}, nil
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

func inboxList(args []string, stdout, stderr io.Writer) int {
	var asJSON bool
	cfg, fs, code := parse("inbox", args, stderr, func(fs *flag.FlagSet) { fs.BoolVar(&asJSON, "json", false, "write the listing as JSON") })
	if code >= 0 {
		return code
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: switchboard inbox [--json] [--config FILE]")
		return 2
	}
	b, err := userInbox(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	items, err := b.List(context.Background())
	if err != nil {
		return fail(stderr, err)
	}
	if asJSON {
		if err := inbox.PrintJSON(stdout, items); err != nil {
			return fail(stderr, err)
		}
		return 0
	}
	inbox.Print(stdout, items, time.Now())
	return 0
}

func inboxShow(args []string, stdout, stderr io.Writer) int {
	cfg, fs, code := parse("show", args, stderr, nil)
	if code >= 0 {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: switchboard show <row|gh:owner/repo#n> [--config FILE]")
		return 2
	}
	b, err := userInbox(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	ref, err := b.Resolve(fs.Arg(0))
	if err != nil {
		return fail(stderr, err)
	}
	if err := b.Show(context.Background(), ref, stdout); err != nil {
		return fail(stderr, err)
	}
	return 0
}

func inboxReply(args []string, stdout, stderr io.Writer) int {
	var useEditor, force bool
	cfg, fs, code := parse("reply", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&useEditor, "editor", false, "write the reply in $EDITOR")
		fs.BoolVar(&force, "force", false, "post even when the issue is not waiting")
	})
	if code >= 0 {
		return code
	}
	if fs.NArg() < 1 || (useEditor == (fs.NArg() > 1)) {
		fmt.Fprintln(stderr, "usage: switchboard reply <row|gh:owner/repo#n> <text>  |  switchboard reply --editor <row|gh:owner/repo#n>")
		return 2
	}
	b, err := userInbox(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	ref, err := b.Resolve(fs.Arg(0))
	if err != nil {
		return fail(stderr, err)
	}
	ctx := context.Background()
	text := strings.Join(fs.Args()[1:], " ")
	if useEditor {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		var q strings.Builder
		if err := b.Show(ctx, ref, &q); err != nil {
			return fail(stderr, err)
		}
		if text, err = inbox.EditorText(editor, fs.Arg(0), q.String()); err != nil {
			return fail(stderr, err)
		}
	}
	if err := b.Reply(ctx, ref, text, force); err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "posted the reply to gh:%s/%s#%d\n", ref.Owner, ref.Repo, ref.Number)
	return 0
}
