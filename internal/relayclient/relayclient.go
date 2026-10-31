// Package relayclient follows the relay's SSE stream (design.md 4, 5): it reconnects with Last-Event-ID, backs off
// between attempts, and reports whether it is connected so that the agent can choose its polling interval.
package relayclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/uskanda/switchboard/internal/relay"
)

// Client subscribes to URL with the bearer Token.
type Client struct {
	URL        string
	Token      string
	HTTP       *http.Client  // http.DefaultClient when nil
	MinBackoff time.Duration // 1s when zero
	MaxBackoff time.Duration // 60s when zero
	Idle       time.Duration // a stream silent this long is dropped; 90s (three pings) when zero
	Logf       func(format string, args ...any)

	lastID int64
}

func (c *Client) grow(d time.Duration) time.Duration {
	max := c.MaxBackoff
	if max == 0 {
		max = 60 * time.Second
	}
	if d *= 2; d > max {
		d = max
	}
	return d
}

// Run follows the stream until ctx ends, passing each event to on. state(true) is called once a connection is up and
// state(false) once it is lost.
func (c *Client) Run(ctx context.Context, on func(relay.Event), state func(connected bool)) {
	min := c.MinBackoff
	if min == 0 {
		min = time.Second
	}
	wait := min
	for ctx.Err() == nil {
		connected, err := c.follow(ctx, on, state)
		if ctx.Err() != nil {
			return
		}
		if connected {
			state(false)
			wait = min
		}
		c.Logf("relay: %v; reconnecting in %s", err, wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = c.grow(wait)
	}
}

// follow makes one connection and reads it until it ends. connected reports whether the stream was opened.
func (c *Client) follow(ctx context.Context, on func(relay.Event), state func(bool)) (connected bool, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "text/event-stream")
	if c.lastID > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatInt(c.lastID, 10))
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("the relay answered %s", resp.Status)
	}
	state(true)
	idle := c.Idle
	if idle == 0 {
		idle = 90 * time.Second
	}
	watchdog := time.AfterFunc(idle, cancel)
	defer watchdog.Stop()

	sc := bufio.NewScanner(resp.Body)
	var id int64
	var data string
	for sc.Scan() {
		watchdog.Reset(idle)
		line := sc.Text()
		switch {
		case line == "":
			if data != "" {
				var e relay.Event
				if err := json.Unmarshal([]byte(data), &e); err != nil {
					c.Logf("relay: skipped an event that is not JSON: %v", err)
				} else {
					on(e)
				}
				if id > 0 {
					c.lastID = id
				}
			}
			id, data = 0, ""
		case strings.HasPrefix(line, "id: "):
			id, _ = strconv.ParseInt(line[4:], 10, 64)
		case strings.HasPrefix(line, "data: "):
			data += line[6:]
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		return true, err
	}
	return true, fmt.Errorf("the stream ended")
}
