package agent

import (
	"context"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

// A grandchild that keeps the output pipe open must not keep shell from returning after the timeout or a cancel.
func TestShellReturnsWhenAGrandchildHoldsTheOutputOpen(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.Timeout = 300 * time.Millisecond
	a := &Agent{Config: cfg}
	for _, cancelEarly := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelEarly {
			time.AfterFunc(100*time.Millisecond, cancel)
		}
		start := time.Now()
		done := make(chan error, 1)
		go func() {
			_, err := a.shell(ctx, t.TempDir(), nil, "sleep 30 & sleep 30")
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Errorf("cancelEarly=%v: shell succeeded, want an error", cancelEarly)
			}
			if d := time.Since(start); d > 5*time.Second {
				t.Errorf("cancelEarly=%v: shell took %v", cancelEarly, d)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("cancelEarly=%v: shell did not return", cancelEarly)
		}
		cancel()
	}
}
