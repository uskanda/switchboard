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

// Item is one waiting, done, running or queued issue, as switchboard inbox --json lists it. Waiting and done issues
// have a row number (N), numbered in this order; an open done issue has DoneSince and ChangeURL, the pull or merge
// request its done post names ("" without one); a running one has RunningSince and Progress, the last line of its
// progress comment ("?" without one); a queued one has QueuedSince, and Reason names the limit reached (max-parallel
// or project-max-parallel).
type Item struct {
	N            int        `json:"n,omitempty"`
	State        string     `json:"state"` // waiting, done, running or queued
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
	QueuedSince  *time.Time `json:"queued_since,omitempty"`
	DoneSince    *time.Time `json:"done_since,omitempty"`
	ChangeURL    string     `json:"change_url,omitempty"`
	// ChangeNumber and ChangeSHA are the change request and the commit the marker of the latest done post names; a
	// page offers the merge (POST /v1/merge) only with both.
	ChangeNumber int    `json:"change_number,omitempty"`
	ChangeSHA    string `json:"change_sha,omitempty"`
	// UpdatedAt and Notes are the forge's update time and count of people's comments (GitLab's user_notes_count, 0
	// on GitHub): a page watching one issue reads it again when either moves.
	UpdatedAt *time.Time `json:"updated_at"`
	Notes     int        `json:"notes"`
	// AutoMerge is set on a done issue while the inbox service waits to merge its change request: the person pressed
	// merge, the forge refused it for a conflict, and the merge is made once the agent has fixed it (ADR-048).
	AutoMerge *AutoMerge `json:"auto_merge,omitempty"`
}

// AutoMerge is a merge the inbox service waits to make: the commit the person pressed merge on, and when.
type AutoMerge struct {
	SHA   string    `json:"sha"`
	Since time.Time `json:"since"`
}

// The outcomes of an AutoMergeResult.
const (
	AutoMergeMerged    = "merged"
	AutoMergeCancelled = "cancelled"
)

// AutoMergeResult is a merge the inbox service waited to make and finished: merged, or cancelled for Reason (closed,
// waiting, instruction, not_fix, changed, change_closed, pushed, forbidden, or user for a cancel). Number is the
// change request, At when it finished.
type AutoMergeResult struct {
	Ref     string    `json:"ref"`
	Number  int       `json:"number"`
	Outcome string    `json:"outcome"`
	Reason  string    `json:"reason,omitempty"`
	At      time.Time `json:"at"`
}

// ForgeError is a forge that could not be read while listing.
type ForgeError struct {
	Forge   string `json:"forge"` // "github" or "gitlab"
	Message string `json:"message"`
}

// Inbox is the answer to GET /v1/inbox. Relay is whether the service follows the relay's events (RelayConnected,
// RelayDisconnected, or "" when inbox.relay is not set); RefreshIntervalSeconds is how often it reads the forges
// while a page listens.
type Inbox struct {
	Items                  []Item       `json:"items"`
	Errors                 []ForgeError `json:"errors"`
	FetchedAt              time.Time    `json:"fetched_at"`
	Relay                  string       `json:"relay"`
	RefreshIntervalSeconds int          `json:"refresh_interval_seconds"`
	// AutoMergeResults are the merges the inbox service finished within the last 24 hours, oldest first, for a page
	// to tell the person how they ended.
	AutoMergeResults []AutoMergeResult `json:"auto_merge_results,omitempty"`
}

// The states of Inbox.Relay. While disconnected (or connecting), changes show at the refresh interval.
const (
	RelayConnected    = "connected"
	RelayDisconnected = "disconnected"
)

// EventInbox names the events of GET /v1/events. Each carries a whole Inbox as its data: the listing when the
// stream opens, then each listing that differs from the one before. Lines starting with ":" keep the stream alive.
const EventInbox = "inbox"

// Detail is the answer to GET /v1/issue?ref=: the agent's latest post on the issue, which holds the question and
// the refused operations, with what its marker says. While the issue runs, State is "running" and Post is the
// progress comment of the run (the claim before there is one); a reply to it is refused as not waiting. While the
// issue waits for a slot, State is "queued" and Since is when it was queued; Post is then the last wait's post while
// the issue still waits for a reply (which is still taken), else the queued comment. Done is set on an open done
// issue, which takes a reply as a waiting one does (ADR-042).
type Detail struct {
	Ref     string     `json:"ref"`
	Title   string     `json:"title"`
	URL     string     `json:"url"`
	Waiting bool       `json:"waiting"`
	Done    bool       `json:"done"`
	Host    string     `json:"host"`
	State   string     `json:"state"`
	Reason  string     `json:"reason"`
	Since   *time.Time `json:"since"`
	Post    string     `json:"post"`
	// ApproveWord is the word whose reply approves a plan, such as /ok.
	ApproveWord string `json:"approve_word,omitempty"`
	// ChangeURL, ChangeNumber and ChangeSHA are set on an open done issue, as on its Item.
	ChangeURL    string `json:"change_url,omitempty"`
	ChangeNumber int    `json:"change_number,omitempty"`
	ChangeSHA    string `json:"change_sha,omitempty"`
	// AutoMerge is set as on its Item.
	AutoMerge *AutoMerge `json:"auto_merge,omitempty"`
	// Author, CreatedAt and Body are the issue's own; Comments are all its comments, oldest first.
	Author    string     `json:"author"`
	CreatedAt *time.Time `json:"created_at"`
	Body      string     `json:"body"`
	Comments  []Comment  `json:"comments"`
}

// Comment is one comment of an issue. Agent is set on a post of the agent; Kind is then the state of its marker
// (running for the claim of a run, progress, queued, waiting, done) and Body is without the marker. A person's comment is
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

// Instruction is one file of the additional instructions. Target is "" for the instructions of every project, else a
// project as gh:owner/repo or gl:group/project. Kind is "plan" for the plan runs' instructions, "run" for those of
// the runs that write files, "implement" for those of the runs that implement an issue (a direct run or the run of an
// approved plan). Content is "" when there is no file.
type Instruction struct {
	Target  string `json:"target"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

// Instructions is the answer to GET /v1/instructions: the global instructions, then those of each configured project,
// in the order plan, run, implement for each, all read on Host, the machine of the inbox service.
type Instructions struct {
	Host  string        `json:"host"`
	Items []Instruction `json:"items"`
}

// SaveInstructionsRequest is the body of PUT /v1/instructions. An empty Kind saves the plan runs' instructions.
type SaveInstructionsRequest struct {
	Target  string `json:"target"`
	Kind    string `json:"kind,omitempty"`
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

// The kinds of start label an issue can be opened with: the configuration's labels.ready, labels.plan and labels.go,
// and StartNone, no label at all, which the agent does not take up until a start label is put on (ADR-049).
const (
	StartReady = "ready"
	StartPlan  = "plan"
	StartGo    = "go"
	StartNone  = "none"
)

// IssueTarget is a project an issue can be opened in, with Host, the machine that runs it, and PlanFirst, whether its
// ready label starts with a plan. An older inbox service leaves PlanFirst out.
type IssueTarget struct {
	Ref       string `json:"ref"`
	Host      string `json:"host"`
	PlanFirst bool   `json:"plan_first"`
}

// IssueStart is a kind of start label (StartReady, StartPlan, StartGo, StartNone) and the label it stands for, which
// is empty for StartNone.
type IssueStart struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// IssueTargets is the answer to GET /v1/issue-targets: every configured project, whatever machine runs it, in the
// order of the configuration, and the start labels the configuration names, then StartNone.
type IssueTargets struct {
	Items  []IssueTarget `json:"items"`
	Starts []IssueStart  `json:"starts"`
}

// MaxIssueTitle and MaxIssueBody bound what POST /v1/issues takes: characters of the title, bytes of the body.
const (
	MaxIssueTitle = 256
	MaxIssueBody  = 1 << 20
)

// CreateIssueRequest is the body of POST /v1/issues. Start is a kind of IssueStart; the inbox service puts the label
// it stands for on the issue, and no other, or no label for StartNone. An empty Start is refused.
type CreateIssueRequest struct {
	Ref   string `json:"ref"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Start string `json:"start"`
}

// CreateIssueResponse is the answer to a POST /v1/issues that opened the issue: its reference and its page.
type CreateIssueResponse struct {
	Ref string `json:"ref"`
	URL string `json:"url"`
}

// MergeRequest is the body of POST /v1/merge: the issue, and the commit of its change request the page showed
// (ChangeSHA). The inbox service merges the change request the agent's latest done post names, only at that commit.
type MergeRequest struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

// ProjectIcons is the answer to GET /v1/project-icons: the emoji of the projects whose icon in the configuration is
// one, by project reference (gh:owner/repo). The other projects' icons are images, read with GET
// /v1/project-icon?ref=, which answers 404 (CodeNotFound) when a project has none.
type ProjectIcons map[string]string

// Icon is the answer to GET /v1/project-icon: a PNG, JPEG, GIF or WebP image and its content type.
type Icon struct {
	Data        []byte
	ContentType string
}

// MergeStatus is the answer to GET /v1/merge-status: where the change request of a done issue stands before the merge
// dialog's merge (ADR-050). It is information only; the forge still decides whether the merge goes through.
type MergeStatus struct {
	// State is the change request's state on the forge: "open", "closed" or "merged".
	State string `json:"state"`
	// HeadMoved is set when the change request's head is no longer the commit the page showed.
	HeadMoved bool `json:"head_moved"`
	// Mergeable is MergeClean, MergeConflict or MergeComputing.
	Mergeable string `json:"mergeable"`
	// CI is CIRead, CINone or CIForbidden; Checks are empty unless it is CIRead.
	CI     string        `json:"ci"`
	Checks []CheckStatus `json:"checks"`
}

// CheckStatus is one check of the commit. Name is the forge's data, to be shown as text only.
type CheckStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"` // CheckSuccess, CheckFailure, CheckRunning or CheckOther
	URL    string `json:"url"`
}

// The values of MergeStatus.Mergeable, MergeStatus.CI and CheckStatus.Status.
const (
	MergeClean     = "clean"
	MergeConflict  = "conflict"
	MergeComputing = "computing" // the forge still computes whether it conflicts

	CIRead      = "ok"        // the commit has checks, listed in Checks
	CINone      = "none"      // the commit has no checks
	CIForbidden = "forbidden" // the inbox token may not read CI

	CheckSuccess = "success"
	CheckFailure = "failure"
	CheckRunning = "running"
	CheckOther   = "other"
)

// MergeAnswer is the body of a POST /v1/merge answered with 202: Code is CodeAutoMergePending, and the inbox service
// merges the change request once the agent has fixed its conflict (ADR-048).
type MergeAnswer struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CodeAutoMergePending is the code of a merge the forge refused for a conflict, which the inbox service makes later.
const CodeAutoMergePending = "auto_merge_pending" // 202

// MergeResult is what a merge came to: AutoMerge is set when the inbox service waits to make it.
type MergeResult struct {
	AutoMerge bool
}

// CancelAutoMergeRequest is the body of POST /v1/automerge/cancel: the issue whose waiting merge to cancel.
type CancelAutoMergeRequest struct {
	Ref string `json:"ref"`
}

// RegisterDeviceRequest is the body of POST /v1/devices: the FCM token of an app that wants the push notifications.
type RegisterDeviceRequest struct {
	Token string `json:"token"`
}

// ApproveRequest is the body of POST /v1/approve: the issue, and the start of the wait the notification reported
// (its waiting_since). The inbox service posts the approve word only while the issue still waits on that plan.
type ApproveRequest struct {
	Ref   string    `json:"ref"`
	Since time.Time `json:"since"`
}

// The codes of Error.
const (
	CodeInvalid        = "invalid"         // 400
	CodeNotFound       = "not_found"       // 404
	CodeNotWaiting     = "not_waiting"     // 409
	CodeStale          = "stale"           // 409: the issue or its change request is not what the page showed
	CodeNotMergeable   = "not_mergeable"   // 409: the forge refused the merge
	CodeForbidden      = "forbidden"       // 403: the inbox token may not merge
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
	case ErrStale:
		return e.Code == CodeStale
	case ErrNotMergeable:
		return e.Code == CodeNotMergeable
	case ErrForbidden:
		return e.Code == CodeForbidden
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
	// ErrStale, ErrNotMergeable and ErrForbidden are the refusals of a merge.
	ErrStale        = errors.New("the issue changed since the page read it")
	ErrNotMergeable = errors.New("the forge refused the merge")
	ErrForbidden    = errors.New("the inbox token may not do this")
	// ErrTooLarge, ErrSecret and ErrUnknownProject are the refusals of a save of instructions; ErrTooLarge and
	// ErrUnknownProject also refuse a new issue.
	ErrTooLarge       = errors.New("too large")
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
