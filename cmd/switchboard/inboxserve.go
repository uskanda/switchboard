package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/automerge"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/inboxserve"
	"github.com/uskanda/switchboard/internal/instructions"
	"github.com/uskanda/switchboard/internal/push"
	"github.com/uskanda/switchboard/internal/relay"
	"github.com/uskanda/switchboard/internal/relayclient"
	"github.com/uskanda/switchboard/internal/secret"
	"github.com/uskanda/switchboard/internal/state"
)

// serveInbox is the inbox the API serves: every configured forge, each read and written with its inbox token.
func serveInbox(c *conn) (*inbox.Inbox, error) {
	forges, err := c.inboxForges("")
	if err != nil {
		return nil, err
	}
	return newInbox(c.cfg, forges), nil
}

func newInbox(cfg *config.Config, forges map[string]forge.Forge) *inbox.Inbox {
	return &inbox.Inbox{Config: cfg, Forges: forges, Now: time.Now}
}

// socketPath is --socket, else where inboxapi.ServerSocket puts it.
func socketPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return inboxapi.ServerSocket()
}

// inboxAPI is the inbox service's handler for cfg: the inbox over forges (the inbox token's clients), the
// instructions in store, the projects, the opening of new issues, the merges of done change requests and what the
// merge dialog reads of them, and the interval the forges are read at while a page listens. reload takes
// the projects of a new configuration file; the other settings stay as cfg has them until a restart.
func inboxAPI(cfg *config.Config, forges map[string]forge.Forge, store instructions.Store) (api *inboxserve.Server, reload func([]config.Project)) {
	repos := map[string]forge.Repos{}
	issues := map[string]forge.Issues{}
	merges := map[string]forge.Merges{}
	ci := map[string]forge.CI{}
	for kind, f := range forges {
		if r, ok := f.(forge.Repos); ok {
			repos[kind] = r
		}
		if i, ok := f.(forge.Issues); ok {
			issues[kind] = i
		}
		if m, ok := f.(forge.Merges); ok {
			merges[kind] = m
		}
		if c, ok := f.(forge.CI); ok {
			ci[kind] = c
		}
	}
	api = &inboxserve.Server{Inbox: newInbox(cfg, forges), Interval: cfg.Inbox.RefreshInterval, Relay: cfg.Inbox.Relay != nil,
		Instructions: &inboxserve.Instructions{Store: store, Projects: refsOf(cfg.Projects), Host: cfg.Host.Name},
		Projects:     &inboxserve.Projects{Config: cfg, Repos: repos},
		NewIssues:    &inboxserve.NewIssues{Config: cfg, Issues: issues},
		Merges:       merges,
		CI:           ci,
	}
	reload = func(ps []config.Project) {
		next := *cfg
		next.Projects = ps
		api.Reload(&next, newInbox(&next, forges))
	}
	api.Projects.Reload = func(c *config.Config) { reload(c.Projects) }
	return api, reload
}

func refsOf(projects []config.Project) []string {
	var out []string
	for _, p := range projects {
		out = append(out, p.Ref)
	}
	return out
}

// inboxRelay is the client the inbox service follows the relay with, using its own token (inbox.relay.token_ref).
func inboxRelay(cfg *config.Config, r *config.AgentRelay, stderr io.Writer) (*relayclient.Client, error) {
	res := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}
	tok, err := res.Token(r.TokenRef)
	if err != nil {
		return nil, err
	}
	return &relayclient.Client{URL: r.URL, Token: tok, Logf: func(format string, args ...any) {
		fmt.Fprintf(stderr, "switchboard: inbox: "+format+"\n", args...)
	}}, nil
}

// inboxPush is the notifier of the inbox service's push notifications, with the Firebase service account key read
// by token (inbox.notify.fcm.credentials_ref); nil without inbox.notify.fcm. Its errors name the key, never its content.
func inboxPush(cfg *config.Config, token func(ref string) (string, error), d state.Dir) (*push.Notifier, error) {
	f := cfg.Inbox.Notify.FCM
	if f == nil {
		return nil, nil
	}
	key, err := token(f.CredentialsRef)
	if err != nil {
		return nil, fmt.Errorf("inbox.notify.fcm: %w", err)
	}
	creds, err := push.ParseCredentials([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("inbox.notify.fcm: the key %s: %w", f.CredentialsRef, err)
	}
	return &push.Notifier{
		Sender:  &push.Client{ProjectID: f.ProjectID, Credentials: creds},
		Devices: &push.Devices{Dir: d},
		Dir:     d,
		Audit:   d.Audit(),
	}, nil
}

// followRelay passes the relay's events and state to api until ctx ends. An event is only a cue to read the forges
// again; which issue it names does not matter, since the listing is read whole.
func followRelay(ctx context.Context, rc *relayclient.Client, api *inboxserve.Server, stderr io.Writer) {
	rc.Run(ctx, func(relay.Event) { api.RelayEvent() }, func(up bool) {
		if up {
			fmt.Fprintln(stderr, "switchboard: inbox: relay: connected")
		} else {
			fmt.Fprintln(stderr, "switchboard: inbox: relay: disconnected")
		}
		api.RelayState(up)
	})
}

func inboxServe(args []string, stdout, stderr io.Writer) int {
	var sock *string
	cfg, fs, code := parse("inbox serve", args, stderr, func(fs *flag.FlagSet) {
		sock = fs.String("socket", "", "Unix socket to listen on (default: $RUNTIME_DIRECTORY/inbox.sock, else $XDG_RUNTIME_DIR/switchboard/inbox.sock)")
	})
	if code >= 0 {
		return code
	} else if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: switchboard inbox serve [--socket PATH] [--config FILE]")
		return 2
	}
	forges, err := connect(cfg).inboxForges("")
	if err != nil {
		return fail(stderr, err)
	}
	path := socketPath(*sock)
	ln, err := inboxserve.Listen(path)
	if err != nil {
		return fail(stderr, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	d, err := stateDir()
	if err != nil {
		return fail(stderr, err)
	}
	api, reload := inboxAPI(cfg, forges, instructions.For(cfg.Path, d.Audit()))
	// The merges refused for a conflict are kept in the state directory and made once the agent fixed them (ADR-048),
	// whether or not a page listens.
	if api.AutoMerge, err = automerge.Open(d); err != nil {
		return fail(stderr, err)
	}
	api.Audit = d.Audit()
	api.Logf = func(format string, args ...any) { fmt.Fprintf(stderr, "switchboard: inbox: "+format+"\n", args...) }
	go api.RunAutoMerge(ctx)
	w, err := config.NewWatcher(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	go w.Run(ctx, stderr, func(ch *config.Change) { reload(ch.Projects) })
	res := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}
	notifier, err := inboxPush(cfg, res.Token, d)
	if err != nil {
		return fail(stderr, err)
	}
	if notifier != nil {
		api.Devices, api.Observer = notifier.Devices, notifier
		api.Watch(ctx)
		fmt.Fprintln(stderr, "switchboard: inbox: push notifications on: reading the forges every", cfg.Inbox.RefreshInterval)
	}
	if r := cfg.Inbox.Relay; r != nil {
		rc, err := inboxRelay(cfg, r, stderr)
		if err != nil {
			return fail(stderr, err)
		}
		go followRelay(ctx, rc, api, stderr)
	}
	// Requests end with ctx, so the event streams, which never go idle, close on a signal instead of holding the
	// shutdown.
	srv := &http.Server{Handler: api, ReadHeaderTimeout: 10 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shut)
	}()
	fmt.Fprintf(stderr, "switchboard: inbox: serving on %s\n", path)
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fail(stderr, err)
	}
	return 0
}
