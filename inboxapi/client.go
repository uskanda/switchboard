package inboxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
)

// Client calls the inbox service on its Unix socket.
type Client struct {
	Socket string

	once sync.Once
	http *http.Client
}

func (c *Client) client() *http.Client {
	c.once.Do(func() {
		var d net.Dialer
		c.http = &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return d.DialContext(ctx, "unix", c.Socket)
			},
		}}
	})
	return c.http
}

// List returns the waiting issues and the forges that could not be read.
func (c *Client) List(ctx context.Context) (*Inbox, error) {
	var out Inbox
	return &out, c.do(ctx, http.MethodGet, "/v1/inbox", nil, &out)
}

// Issue returns one issue, named as gh:owner/repo#n or gl:group/project#n.
func (c *Client) Issue(ctx context.Context, ref string) (*Detail, error) {
	var out Detail
	return &out, c.do(ctx, http.MethodGet, "/v1/issue?ref="+url.QueryEscape(ref), nil, &out)
}

// Reply posts text on the issue. Unless force is set, the issue must still be waiting (ErrNotWaiting).
func (c *Client) Reply(ctx context.Context, ref, text string, force bool) error {
	return c.do(ctx, http.MethodPost, "/v1/reply", ReplyRequest{Ref: ref, Text: text, Force: force}, nil)
}

// Instructions returns the additional instructions of the plan runs, as the inbox service's machine has them.
func (c *Client) Instructions(ctx context.Context) (*Instructions, error) {
	var out Instructions
	return &out, c.do(ctx, http.MethodGet, "/v1/instructions", nil, &out)
}

// SaveInstructions replaces the instructions of target ("" for every project) with content. It is refused with
// ErrTooLarge, ErrSecret or ErrUnknownProject.
func (c *Client) SaveInstructions(ctx context.Context, target, content string) error {
	return c.do(ctx, http.MethodPut, "/v1/instructions", SaveInstructionsRequest{Target: target, Content: content}, nil)
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://inbox"+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		e := &Error{}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(e) != nil || e.Code == "" {
			e = &Error{Code: CodeForge, Message: fmt.Sprintf("the inbox service answered %s", resp.Status)}
		}
		return e
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
