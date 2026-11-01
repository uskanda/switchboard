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
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
	"github.com/uskanda/switchboard/internal/inboxserve"
	"github.com/uskanda/switchboard/internal/instructions"
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
// instructions in store, the projects, and the interval the forges are read at while a page listens. reload takes
// the projects of a new configuration file; the other settings stay as cfg has them until a restart.
func inboxAPI(cfg *config.Config, forges map[string]forge.Forge, store instructions.Store) (api *inboxserve.Server, reload func([]config.Project)) {
	repos := map[string]forge.Repos{}
	for kind, f := range forges {
		if r, ok := f.(forge.Repos); ok {
			repos[kind] = r
		}
	}
	api = &inboxserve.Server{Inbox: newInbox(cfg, forges), Interval: cfg.Inbox.RefreshInterval,
		Instructions: &inboxserve.Instructions{Store: store, Projects: refsOf(cfg.Projects), Host: cfg.Host.Name},
		Projects:     &inboxserve.Projects{Config: cfg, Repos: repos},
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
	w, err := config.NewWatcher(cfg)
	if err != nil {
		return fail(stderr, err)
	}
	go w.Run(ctx, stderr, reload)
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
