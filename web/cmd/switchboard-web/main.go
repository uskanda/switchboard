// Command switchboard-web serves the web inbox: the waiting issues and a reply box, in a browser on the LAN. It asks
// the core's inbox service (switchboard inbox serve) for everything and holds no forge token.
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
	"github.com/uskanda/switchboard/web/internal/assets"
	"github.com/uskanda/switchboard/web/internal/server"
	"github.com/uskanda/switchboard/web/internal/webconfig"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

const usage = "usage: switchboard-web serve [--config FILE]"

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "serve" {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("switchboard-web serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", webconfig.DefaultPath(), "web configuration file")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	cfg, err := webconfig.Load(*path)
	if err != nil {
		fmt.Fprintf(stderr, "switchboard-web: %v\n", err)
		return 1
	}
	sock := cfg.Socket
	if sock == "" {
		sock = inboxapi.DefaultSocket()
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		fmt.Fprintf(stderr, "switchboard-web: %v\n", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{
		Handler:           &server.Server{Inbox: &inboxapi.Client{Socket: sock}, Listen: cfg.Listen, Assets: assets.FS()},
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shut)
	}()
	fmt.Fprintf(stderr, "switchboard-web: serving http://%s, inbox service at %s\n", cfg.Listen, sock)
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "switchboard-web: %v\n", err)
		return 1
	}
	return 0
}
