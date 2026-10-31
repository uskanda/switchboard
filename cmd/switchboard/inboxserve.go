package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
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
	return &inbox.Inbox{Config: c.cfg, Forges: forges, Now: time.Now}, nil
}

// socketPath is --socket, else where inboxapi.ServerSocket puts it.
func socketPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return inboxapi.ServerSocket()
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
	b, err := serveInbox(connect(cfg))
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
	var projects []string
	for _, p := range cfg.Projects {
		projects = append(projects, p.Ref)
	}
	api := &inboxserve.Server{Inbox: b, Instructions: &inboxserve.Instructions{
		Store: instructions.For(cfg.Path, d.Audit()), Projects: projects, Host: cfg.Host.Name,
	}}
	srv := &http.Server{Handler: api, ReadHeaderTimeout: 10 * time.Second}
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
