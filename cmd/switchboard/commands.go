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
	"github.com/uskanda/switchboard/internal/doctor"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/forge/github"
	"github.com/uskanda/switchboard/internal/forge/gitlab"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/labels"
	"github.com/uskanda/switchboard/internal/notify"
	"github.com/uskanda/switchboard/internal/relayclient"
	"github.com/uskanda/switchboard/internal/secret"
	"github.com/uskanda/switchboard/internal/state"
)

func init() {
	cli.Register("labels sync", labelsSync)
	cli.Register("agent run", agentRun)
	cli.Register("agent status", agentStatus)
	cli.Register("inbox serve", inboxServe)
	cli.Register("inbox", inboxList)
	cli.Register("show", inboxShow)
	cli.Register("reply", inboxReply)
	cli.Register("doctor", runDoctor)
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

// connect reads tokens from the Secret Service (and the environment when allow_env is set).
func connect(cfg *config.Config) *conn {
	r := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}
	return newConn(cfg, r.Token)
}

// userInbox connects to each forge with the user's own token (inbox_token_ref). only, "github" or "gitlab",
// limits it to one forge.
func userInbox(cfg *config.Config, only string) (*inbox.Inbox, error) {
	forges, err := connect(cfg).inboxForges(only)
	if err != nil {
		return nil, err
	}
	b := &inbox.Inbox{Config: cfg, Forges: forges, Cache: inbox.DefaultCache(), Now: time.Now}
	switch only {
	case "github":
		b.Only = forge.GitHub
	case "gitlab":
		b.Only = forge.GitLab
	}
	return b, nil
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
	c := connect(cfg)
	n := 0
	for _, p := range cfg.Projects {
		r, err := c.agentClient(p)
		if err != nil {
			return fail(stderr, err)
		}
		created, err := labels.Sync(context.Background(), r.Forge, []config.Project{p}, cfg.Labels)
		n += created
		if err != nil {
			return fail(stderr, err)
		}
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
	c := connect(cfg)
	routes, err := c.agentRoutes()
	if err != nil {
		return fail(stderr, err)
	}
	d, err := stateDir()
	if err != nil {
		return fail(stderr, err)
	}
	audit := d.Audit()
	for _, kind := range []string{forge.GitHub, forge.GitLab} {
		if err := c.judge(ctx, kind, routes, d, audit); err != nil {
			return fail(stderr, err)
		}
	}
	a := &agent.Agent{Config: cfg, Routes: routes, State: d, Audit: audit, Log: stderr}
	if r := cfg.Agent.Relay; r != nil {
		res := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}
		tok, err := res.Token(r.TokenRef)
		if err != nil {
			return fail(stderr, err)
		}
		a.Relay = &relayclient.Client{URL: r.URL, Token: tok, Logf: func(format string, args ...any) {
			fmt.Fprintf(stderr, "switchboard: "+format+"\n", args...)
		}}
	}
	if t := cfg.Notify.Telegram; t != nil {
		res := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}
		tok, err := res.Token(t.TokenRef)
		if err != nil {
			return fail(stderr, err)
		}
		a.Notifier = &notify.Telegram{Token: tok, ChatID: t.ChatID}
	}
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
	var only string
	cfg, fs, code := parse("inbox", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&asJSON, "json", false, "write the listing as JSON")
		fs.StringVar(&only, "forge", "", "list one forge only: github or gitlab")
	})
	if code >= 0 {
		return code
	}
	if fs.NArg() != 0 || (only != "" && only != "github" && only != "gitlab") {
		fmt.Fprintln(stderr, "usage: switchboard inbox [--json] [--forge github|gitlab] [--config FILE]")
		return 2
	}
	b, err := userInbox(cfg, only)
	if err != nil {
		return fail(stderr, err)
	}
	// A forge that cannot be read still leaves the others' listing, followed by the error.
	items, listErr := b.List(context.Background())
	if asJSON {
		if err := inbox.PrintJSON(stdout, items); err != nil {
			return fail(stderr, err)
		}
	} else {
		inbox.Print(stdout, items, time.Now())
	}
	if listErr != nil {
		return fail(stderr, listErr)
	}
	return 0
}

func inboxShow(args []string, stdout, stderr io.Writer) int {
	cfg, fs, code := parse("show", args, stderr, nil)
	if code >= 0 {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: switchboard show <row|gh:owner/repo#n|gl:group/project#n> [--config FILE]")
		return 2
	}
	b, err := userInbox(cfg, "")
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
		fmt.Fprintln(stderr, "usage: switchboard reply <row|gh:owner/repo#n|gl:group/project#n> <text>  |  switchboard reply --editor <row|ref>")
		return 2
	}
	b, err := userInbox(cfg, "")
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
	fmt.Fprintf(stdout, "posted the reply to %s\n", ref)
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	cfg, _, code := parse("doctor", args, stderr, nil)
	if code >= 0 {
		return code
	}
	d := &doctor.Doctor{
		Config:  cfg,
		State:   state.Dir{Path: state.DefaultDir()},
		Secrets: secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv},
		Forge: func(kind, token string) forge.Forge {
			if kind == forge.GitLab {
				return gitlab.New(token, cfg.Forges.GitLab.BaseURL)
			}
			return github.New(token)
		},
		Out: stdout,
	}
	if n := d.Run(context.Background()); n > 0 {
		fmt.Fprintf(stderr, "switchboard: doctor found %d problem(s)\n", n)
		return 1
	}
	return 0
}
