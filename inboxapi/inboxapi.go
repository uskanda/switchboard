// Package inboxapi is the contract between the core's inbox service (switchboard inbox serve) and the programs
// that show the inbox, such as the web inbox: the request and response types of its HTTP API on a Unix socket, and
// a client for it. It never handles a forge token; only the inbox service does.
package inboxapi

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Item is one waiting issue, as switchboard inbox --json lists it.
type Item struct {
	N            int        `json:"n"`
	Ref          string     `json:"ref"`
	Project      string     `json:"project"`
	Number       int        `json:"number"`
	Title        string     `json:"title"`
	URL          string     `json:"url"`
	Host         string     `json:"host"`
	WaitingSince *time.Time `json:"waiting_since"`
	Reason       string     `json:"reason"`
	Question     string     `json:"question"`
}

// ForgeError is a forge that could not be read while listing.
type ForgeError struct {
	Forge   string `json:"forge"` // "github" or "gitlab"
	Message string `json:"message"`
}

// Inbox is the answer to GET /v1/inbox.
type Inbox struct {
	Items     []Item       `json:"items"`
	Errors    []ForgeError `json:"errors"`
	FetchedAt time.Time    `json:"fetched_at"`
}

// Detail is the answer to GET /v1/issue?ref=: the agent's latest post on the issue, which holds the question and
// the refused operations, with what its marker says.
type Detail struct {
	Ref     string     `json:"ref"`
	Title   string     `json:"title"`
	URL     string     `json:"url"`
	Waiting bool       `json:"waiting"`
	Host    string     `json:"host"`
	State   string     `json:"state"`
	Reason  string     `json:"reason"`
	Since   *time.Time `json:"since"`
	Post    string     `json:"post"`
}

// ReplyRequest is the body of POST /v1/reply.
type ReplyRequest struct {
	Ref   string `json:"ref"`
	Text  string `json:"text"`
	Force bool   `json:"force,omitempty"`
}

// The codes of Error.
const (
	CodeInvalid    = "invalid"     // 400
	CodeNotFound   = "not_found"   // 404
	CodeNotWaiting = "not_waiting" // 409
	CodeForge      = "forge"       // 502
)

// Error is how the API answers a request it could not serve.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// Is lets errors.Is(err, ErrNotWaiting) and the like match an Error by its code.
func (e *Error) Is(target error) bool {
	switch target {
	case ErrInvalid:
		return e.Code == CodeInvalid
	case ErrNotFound:
		return e.Code == CodeNotFound
	case ErrNotWaiting:
		return e.Code == CodeNotWaiting
	}
	return false
}

var (
	ErrInvalid    = errors.New("invalid request")
	ErrNotFound   = errors.New("not found")
	ErrNotWaiting = errors.New("the issue is not waiting")
	// ErrUnavailable is returned when the inbox service cannot be reached at all.
	ErrUnavailable = errors.New("the inbox service is not reachable")
)

// ServerSocket is where switchboard inbox serve listens without --socket: inbox.sock in systemd's
// RUNTIME_DIRECTORY, else $XDG_RUNTIME_DIR/switchboard/inbox.sock. It is "" when neither is set.
func ServerSocket() string {
	if d := os.Getenv("RUNTIME_DIRECTORY"); d != "" {
		return filepath.Join(d, "inbox.sock")
	}
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "switchboard", "inbox.sock")
	}
	return ""
}

// SystemSocket is where the inbox service installed by switchboard service install --inbox listens.
const SystemSocket = "/run/switchboard/inbox.sock"

// DefaultSocket is where a client looks without a configured path: the installed service's socket when it exists,
// else ServerSocket.
func DefaultSocket() string {
	if _, err := os.Stat(SystemSocket); err == nil {
		return SystemSocket
	}
	return ServerSocket()
}
