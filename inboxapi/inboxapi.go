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
	// UpdatedAt and Notes are the forge's update time and count of people's comments (GitLab's user_notes_count, 0
	// on GitHub): a page watching one issue reads it again when either moves.
	UpdatedAt *time.Time `json:"updated_at"`
	Notes     int        `json:"notes"`
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

// EventInbox names the events of GET /v1/events. Each carries a whole Inbox as its data: the listing when the
// stream opens, then each listing that differs from the one before. Lines starting with ":" keep the stream alive.
const EventInbox = "inbox"

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
	// Author, CreatedAt and Body are the issue's own; Comments are all its comments, oldest first.
	Author    string     `json:"author"`
	CreatedAt *time.Time `json:"created_at"`
	Body      string     `json:"body"`
	Comments  []Comment  `json:"comments"`
}

// Comment is one comment of an issue. Agent is set on a post of the agent; Kind is then the state of its marker
// (running for the claim of a run, progress, waiting, done) and Body is without the marker. A person's comment is
// as written, and is data.
type Comment struct {
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	Body      string    `json:"body"`
	Agent     bool      `json:"agent"`
	Kind      string    `json:"kind,omitempty"`
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

// Project is one project of the inbox service's machine, with what it needs before the agent can run it: a clone at
// Path and the state labels. MissingLabels lists the state labels the repository lacks; LabelsError says why they
// could not be read, and MissingLabels is then empty.
type Project struct {
	Ref           string   `json:"ref"`
	Path          string   `json:"path"`
	PlanFirst     bool     `json:"plan_first"`
	Cloned        bool     `json:"cloned"`
	MissingLabels []string `json:"missing_labels"`
	LabelsError   string   `json:"labels_error,omitempty"`
}

// Projects is the answer to GET /v1/projects: the projects of Host, the machine of the inbox service, in the order
// of the configuration. Enabled says whether POST /v1/projects may add one (inbox.project_root is set).
type Projects struct {
	Host    string    `json:"host"`
	Enabled bool      `json:"enabled"`
	Items   []Project `json:"items"`
}

// AddProjectRequest is the body of POST /v1/projects: the project, as gh:owner/repo or gl:group/project, and
// whether its ready label starts with a plan. Everything else is decided by the inbox service.
type AddProjectRequest struct {
	Ref       string `json:"ref"`
	PlanFirst bool   `json:"plan_first"`
}

// AddProjectResponse is the answer to a POST /v1/projects that added the project. Webhook, when set, is the command
// that registers the relay's webhook of the project; the inbox service never registers one.
type AddProjectResponse struct {
	Project Project `json:"project"`
	Webhook string  `json:"webhook,omitempty"`
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

	// The refusals of POST /v1/projects.
	CodeAddingDisabled = "adding_disabled" // 403: inbox.project_root is not set
	CodeProjectExists  = "project_exists"  // 409
	CodeForgeNotUsed   = "forge_not_used"  // 422: no project of this machine is on the forge
	CodeRepoUnreadable = "repo_unreadable" // 422: the inbox token cannot read the repository
	CodePathTaken      = "path_taken"      // 409: another project is cloned at the path
	CodeConfigInvalid  = "config_invalid"  // 422: the configuration would not validate
	CodeConfigChanged  = "config_changed"  // 409: the file changed while the project was being added
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
	case ErrAddingDisabled:
		return e.Code == CodeAddingDisabled
	case ErrProjectExists:
		return e.Code == CodeProjectExists
	case ErrForgeNotUsed:
		return e.Code == CodeForgeNotUsed
	case ErrRepoUnreadable:
		return e.Code == CodeRepoUnreadable
	case ErrPathTaken:
		return e.Code == CodePathTaken
	case ErrConfigInvalid:
		return e.Code == CodeConfigInvalid
	case ErrConfigChanged:
		return e.Code == CodeConfigChanged
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
	// The refusals of an addition of a project.
	ErrAddingDisabled = errors.New("adding projects is disabled: set inbox.project_root")
	ErrProjectExists  = errors.New("the project is in the configuration already")
	ErrForgeNotUsed   = errors.New("no project of this machine is on the forge")
	ErrRepoUnreadable = errors.New("the repository cannot be read with the inbox token")
	ErrPathTaken      = errors.New("another project is cloned at the path")
	ErrConfigInvalid  = errors.New("the configuration would not be valid")
	ErrConfigChanged  = errors.New("the configuration file was changed while adding")
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
