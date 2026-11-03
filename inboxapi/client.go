package inboxapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client calls the inbox service on its Unix socket.
type Client struct {
	// Socket is the inbox service's socket. When empty, DefaultSocket is looked up on each request, so the client
	// reaches an inbox service that made its socket after the client started.
	Socket string

	once sync.Once
	http *http.Client
}

func (c *Client) client() *http.Client {
	c.once.Do(func() {
		var d net.Dialer
		c.http = &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				sock := c.Socket
				if sock == "" {
					sock = DefaultSocket()
				}
				return d.DialContext(ctx, "unix", sock)
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

// Merge marks the change request of a done issue ready and merges it with the inbox token, only at sha. It is
// refused with ErrStale, ErrNotMergeable or ErrForbidden.
func (c *Client) Merge(ctx context.Context, ref, sha string) error {
	return c.do(ctx, http.MethodPost, "/v1/merge", MergeRequest{Ref: ref, SHA: sha}, nil)
}

// RegisterDevice adds an app's FCM token to the devices the inbox service sends push notifications to.
func (c *Client) RegisterDevice(ctx context.Context, token string) error {
	return c.do(ctx, http.MethodPost, "/v1/devices", RegisterDeviceRequest{Token: token}, nil)
}

// Approve posts the approve word on an issue waiting on a plan, only while the wait started at since; else it is
// refused with ErrStale.
func (c *Client) Approve(ctx context.Context, ref string, since time.Time) error {
	return c.do(ctx, http.MethodPost, "/v1/approve", ApproveRequest{Ref: ref, Since: since}, nil)
}

// Instructions returns the additional instructions of both kinds, as the inbox service's machine has them.
func (c *Client) Instructions(ctx context.Context) (*Instructions, error) {
	var out Instructions
	return &out, c.do(ctx, http.MethodGet, "/v1/instructions", nil, &out)
}

// SaveInstructions replaces the instructions of req's target ("" for every project) and kind with its content. It is
// refused with ErrTooLarge, ErrSecret, ErrUnknownProject, or ErrInvalid for an unknown kind.
func (c *Client) SaveInstructions(ctx context.Context, req SaveInstructionsRequest) error {
	return c.do(ctx, http.MethodPut, "/v1/instructions", req, nil)
}

// Projects returns the projects of the inbox service's machine and their state.
func (c *Client) Projects(ctx context.Context) (*Projects, error) {
	var out Projects
	return &out, c.do(ctx, http.MethodGet, "/v1/projects", nil, &out)
}

// AddProject adds a project to the configuration of the inbox service's machine. It is refused with
// ErrAddingDisabled, ErrInvalid, ErrProjectExists, ErrForgeNotUsed, ErrRepoUnreadable, ErrPathTaken,
// ErrConfigInvalid or ErrConfigChanged.
func (c *Client) AddProject(ctx context.Context, req AddProjectRequest) (*AddProjectResponse, error) {
	var out AddProjectResponse
	if err := c.do(ctx, http.MethodPost, "/v1/projects", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// IssueTargets returns the projects an issue can be opened in and the start labels it can carry.
func (c *Client) IssueTargets(ctx context.Context) (*IssueTargets, error) {
	var out IssueTargets
	return &out, c.do(ctx, http.MethodGet, "/v1/issue-targets", nil, &out)
}

// CreateIssue opens an issue with the inbox token. It is refused with ErrUnknownProject, ErrInvalid or ErrTooLarge.
func (c *Client) CreateIssue(ctx context.Context, req CreateIssueRequest) (*CreateIssueResponse, error) {
	var out CreateIssueResponse
	if err := c.do(ctx, http.MethodPost, "/v1/issues", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProjectIcons returns the emoji icons of the configuration, by project reference.
func (c *Client) ProjectIcons(ctx context.Context) (ProjectIcons, error) {
	out := ProjectIcons{}
	return out, c.do(ctx, http.MethodGet, "/v1/project-icons", nil, &out)
}

// ProjectIcon returns the icon image of a project, named as gh:owner/repo or gl:group/project. It is ErrNotFound when
// the project has none that can be read.
func (c *Client) ProjectIcon(ctx context.Context, ref string) (*Icon, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://inbox/v1/project-icon?ref="+url.QueryEscape(ref), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, answerError(resp)
	}
	// The service sends images of at most 256 KiB; the bound only keeps a broken answer from filling memory.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return &Icon{Data: data, ContentType: resp.Header.Get("Content-Type")}, nil
}

// Events follows GET /v1/events: it calls fn with the listing the inbox service sends first, then with each listing
// that differs from the one before, until ctx ends (ctx.Err()) or the stream breaks (ErrUnavailable). Reconnecting
// is the caller's.
func (c *Client) Events(ctx context.Context, fn func(*Inbox)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://inbox/v1/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.client().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return answerError(resp)
	}
	r := bufio.NewReader(resp.Body)
	var event, data string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("%w: the event stream ended", ErrUnavailable)
			}
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if event == EventInbox && data != "" {
				var in Inbox
				if err := json.Unmarshal([]byte(data), &in); err != nil {
					return fmt.Errorf("the inbox service sent a listing that is not JSON: %w", err)
				}
				fn(&in)
			}
			event, data = "", ""
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		}
	}
}

// answerError is the Error of a response the API refused.
func answerError(resp *http.Response) error {
	e := &Error{}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(e) != nil || e.Code == "" {
		e = &Error{Code: CodeForge, Message: fmt.Sprintf("the inbox service answered %s", resp.Status)}
	}
	return e
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
		return answerError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
