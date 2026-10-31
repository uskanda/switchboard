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

// Item is one waiting or running issue, as switchboard inbox --json lists it. Only waiting issues have a row
// number (N); a running one has RunningSince and Progress, the last line of its progress comment ("?" without one).
type Item struct {
	N            int        `json:"n,omitempty"`
	State        string     `json:"state"` // waiting or running
	Ref          string     `json:"ref"`
	Project      string     `json:"project"`
	Number       int        `json:"number"`
	Title        string     `json:"title"`
	URL          string     `json:"url"`
	Host         string     `json:"host"`
	WaitingSince *time.Time `json:"waiting_since"`
	Reason       string     `json:"reason"`
	Question     string     `json:"question"`
	RunningSince *time.Time `json:"running_since,omitempty"`
	Progress     string     `json:"progress,omitempty"`
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
// the refused operations, with what its marker says. While the issue runs, State is "running" and Post is the
// progress comment of the run (the claim before there is one); a reply to it is refused as not waiting.
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
	// ApproveWord is the word whose reply approves a plan, such as /ok.
	ApproveWord string `json:"approve_word,omitempty"`
}

// ReplyRequest is the body of POST /v1/reply.
type ReplyRequest struct {
	Ref   string `json:"ref"`
	Text  string `json:"text"`
	Force bool   `json:"force,omitempty"`
}

// Instruction is one file of the additional instructions of the plan runs. Target is "" for the instructions of
// every project, else a project as gh:owner/repo or gl:group/project. Content is "" when there is no file.
type Instruction struct {
	Target  string `json:"target"`
	Content string `json:"content"`
}

// Instructions is the answer to GET /v1/instructions: the global instructions, then one item per configured
// project, all read on Host, the machine of the inbox service.
type Instructions struct {
	Host  string        `json:"host"`
	Items []Instruction `json:"items"`
}

// SaveInstructionsRequest is the body of PUT /v1/instructions.
type SaveInstructionsRequest struct {
	Target  string `json:"target"`
	Content string `json:"content"`
}

// The codes of Error.
const (
	CodeInvalid        = "invalid"         // 400
	CodeNotFound       = "not_found"       // 404
	CodeNotWaiting     = "not_waiting"     // 409
	CodeTooLarge       = "too_large"       // 413
	CodeSecret         = "secret"          // 422
	CodeUnknownProject = "unknown_project" // 422
	CodeForge          = "forge"           // 502
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
	case ErrTooLarge:
		return e.Code == CodeTooLarge
	case ErrSecret:
		return e.Code == CodeSecret
	case ErrUnknownProject:
		return e.Code == CodeUnknownProject
	}
	return false
}

var (
	ErrInvalid    = errors.New("invalid request")
	ErrNotFound   = errors.New("not found")
	ErrNotWaiting = errors.New("the issue is not waiting")
	// ErrTooLarge, ErrSecret and ErrUnknownProject are the refusals of a save of instructions.
	ErrTooLarge       = errors.New("the instructions are too large")
	ErrSecret         = errors.New("the instructions look like they hold a secret")
	ErrUnknownProject = errors.New("the project is not in the configuration")
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

// systemSocket is SystemSocket, replaced in tests.
var systemSocket = SystemSocket

// DefaultSocket is where a client looks without a configured path: the installed service's socket when it exists,
// else ServerSocket, else the installed service's socket even though it does not exist yet. A client started by
// systemd without XDG_RUNTIME_DIR, before the inbox service made its socket, still knows where it will be.
func DefaultSocket() string {
	if _, err := os.Stat(systemSocket); err == nil {
		return systemSocket
	}
	if s := ServerSocket(); s != "" {
		return s
	}
	return systemSocket
}
