package relay

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

// Serve runs the receiver on webhookLn and the SSE hub on sseLn until ctx ends. The two never share a listener, so
// the webhook port can be published without exposing the stream (design.md 4.7).
func Serve(ctx context.Context, cfg config.Relay, webhookLn, sseLn net.Listener, secret func(ref string) (string, error), logf func(string, ...any)) error {
	hosts := map[string]string{}
	for _, h := range cfg.Hosts {
		hosts[h.Name] = h.TokenHash
	}
	hub := &Hub{Hosts: hosts, Logf: logf}
	rec := &Receiver{Hooks: cfg.Hooks, Secret: secret, Now: time.Now, Publish: hub.Publish, Logf: logf}
	base := func(net.Listener) context.Context { return ctx } // ending ctx also ends the open streams
	servers := []*http.Server{
		{Handler: rec, BaseContext: base, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second},
		{Handler: hub, BaseContext: base, ReadHeaderTimeout: 10 * time.Second},
	}
	errs := make(chan error, 2)
	for i, ln := range []net.Listener{webhookLn, sseLn} {
		go func(s *http.Server, ln net.Listener) {
			if err := s.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
				errs <- err
				return
			}
			errs <- nil
		}(servers[i], ln)
	}
	logf("relay: webhooks on %s, events on %s, %d webhook(s), %d host(s)", webhookLn.Addr(), sseLn.Addr(), len(cfg.Hooks), len(hosts))
	var first error
	select {
	case <-ctx.Done():
	case first = <-errs:
	}
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, s := range servers {
		s.Shutdown(shut)
	}
	return first
}
