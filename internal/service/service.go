// Package service runs the agent as a systemd system service under the user's own account (User=), with the agent
// tokens as credentials encrypted with the host key (ADR-032). Everything that needs root is a Step run through
// sudo, shown in full and confirmed before any of it runs (ADR-034).
package service

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/secret"
)

// The places the services use.
const (
	Name          = "switchboard-agent.service"
	UnitPath      = "/etc/systemd/system/" + Name
	RelayName     = "switchboard-relay.service"
	RelayUnitPath = "/etc/systemd/system/" + RelayName
	InboxName     = "switchboard-inbox.service"
	WebName       = "switchboard-web.service"
	CredDir       = "/etc/credstore.encrypted"
)

// Kind is which service a Setup describes.
type Kind string

const (
	KindAgent Kind = ""      // agent run
	KindRelay Kind = "relay" // relay serve
	KindInbox Kind = "inbox" // inbox serve, with the inbox tokens
	KindWeb   Kind = "web"   // switchboard-web serve, with no credential
)

// Credential is one agent token, by its token_ref and its credential name.
type Credential struct {
	Ref  string
	Name string
}

// Path is where the encrypted credential is kept.
func (c Credential) Path() string { return CredDir + "/switchboard." + c.Name }

// Credentials lists the agent tokens of this host's projects: the forges' token_ref and the projects' overrides, and
// the relay bearer token when the agent follows a relay, and the Telegram bot token when it notifies. The inbox
// tokens are left out; the agent's service never reads or replies as the user.
func Credentials(cfg *config.Config) []Credential {
	var refs []string
	for _, p := range cfg.ProjectsFor(cfg.Host.Name) {
		refs = append(refs, cfg.AgentTokenRef(p))
	}
	if r := cfg.Agent.Relay; r != nil {
		refs = append(refs, r.TokenRef)
	}
	if t := cfg.Notify.Telegram; t != nil {
		refs = append(refs, t.TokenRef)
	}
	return credentials(refs)
}

// InboxCredentials lists the inbox tokens of the forges the configuration uses: the inbox service reads and
// replies as the user, and needs no agent token.
func InboxCredentials(cfg *config.Config) []Credential {
	var refs []string
	for _, kind := range []string{"gh", "gl"} {
		if cfg.Uses(kind) {
			refs = append(refs, cfg.InboxTokenRef(kind))
		}
	}
	return credentials(refs)
}

// RelayCredentials lists the webhook secrets of the relay. It holds no forge token (ADR-004).
func RelayCredentials(cfg *config.Config) []Credential {
	var refs []string
	for _, h := range cfg.Relay.Hooks {
		refs = append(refs, h.SecretRef)
	}
	return credentials(refs)
}

func credentials(refs []string) []Credential {
	var out []Credential
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, Credential{Ref: ref, Name: secret.CredentialName(ref)})
	}
	return out
}

// Setup is what the unit is made from.
type Setup struct {
	User, Home, Path string // the account the service runs as, its home, and the PATH that finds claude and git
	// Binary and Config are absolute paths: of switchboard and config.yaml, or for KindWeb of switchboard-web and
	// web.yaml.
	Binary, Config string
	Creds          []Credential
	Kind           Kind
}

// UnitName is the name of the service's unit.
func (s Setup) UnitName() string {
	switch s.Kind {
	case KindRelay:
		return RelayName
	case KindInbox:
		return InboxName
	case KindWeb:
		return WebName
	}
	return Name
}

// UnitPath is where the service's unit is placed.
func (s Setup) UnitPath() string { return "/etc/systemd/system/" + s.UnitName() }

// Unit is the systemd unit of the service.
func (s Setup) Unit() string {
	var b strings.Builder
	what, command, after := "agent", "agent run", "network-online.target"
	switch s.Kind {
	case KindRelay:
		what, command = "relay", "relay serve"
	case KindInbox:
		what, command = "inbox", "inbox serve"
	case KindWeb:
		what, command, after = "web inbox", "serve", after+" "+InboxName
	}
	fmt.Fprintf(&b, "[Unit]\nDescription=switchboard %s\nAfter=%s\nWants=%s\n\n", what, after, after)
	fmt.Fprintf(&b, "[Service]\nUser=%s\nEnvironment=HOME=%s\nEnvironment=PATH=%s\n", s.User, s.Home, s.Path)
	fmt.Fprintf(&b, "ExecStart=%s %s --config %s\n", s.Binary, command, s.Config)
	if s.Kind == KindInbox {
		// Only the inbox's unit owns the directory: systemd removes a unit's RuntimeDirectory when the unit stops,
		// and the web inbox, running as the same user, opens the socket in it.
		fmt.Fprintf(&b, "RuntimeDirectory=switchboard\nRuntimeDirectoryMode=0700\n")
	}
	for _, c := range s.Creds {
		fmt.Fprintf(&b, "LoadCredentialEncrypted=%s:%s\n", c.Name, c.Path())
	}
	fmt.Fprintf(&b, "Restart=on-failure\nRestartSec=30s\n\n[Install]\nWantedBy=multi-user.target\n")
	return b.String()
}

// Step is one command run through sudo. Stdin, when set, is fed to it and never shown: it carries a token.
type Step struct {
	Args  []string
	Stdin string
}

// String is the command as it runs, without what goes in on standard input.
func (s Step) String() string { return "sudo " + strings.Join(s.Args, " ") }

// InstallSteps places the credentials and the unit (written to unitFile beforehand), then enables and restarts the
// service. token reads each agent token, from the Secret Service.
func (s Setup) InstallSteps(unitFile string, token func(ref string) (string, error)) ([]Step, error) {
	steps := []Step{{Args: []string{"install", "-d", "-m", "0700", CredDir}}}
	for _, c := range s.Creds {
		t, err := token(c.Ref)
		if err != nil {
			return nil, err
		}
		steps = append(steps, Step{Args: []string{"systemd-creds", "encrypt", "--name=" + c.Name, "-", c.Path()}, Stdin: t})
	}
	return append(steps,
		Step{Args: []string{"install", "-m", "0644", unitFile, s.UnitPath()}},
		Step{Args: []string{"systemctl", "daemon-reload"}},
		Step{Args: []string{"systemctl", "enable", s.UnitName()}},
		Step{Args: []string{"systemctl", "restart", s.UnitName()}},
	), nil
}

// UninstallSteps stops and disables the service, and removes the unit and the credentials.
func (s Setup) UninstallSteps() []Step {
	steps := []Step{
		{Args: []string{"systemctl", "disable", "--now", s.UnitName()}},
		{Args: []string{"rm", "-f", s.UnitPath()}},
	}
	for _, c := range s.Creds {
		steps = append(steps, Step{Args: []string{"rm", "-f", c.Path()}})
	}
	return append(steps, Step{Args: []string{"systemctl", "daemon-reload"}})
}

// ErrDeclined reports that the user did not answer yes.
var ErrDeclined = errors.New("nothing was run")

// Confirm is how Apply asks before it runs anything.
type Confirm struct {
	In     io.Reader
	Out    io.Writer
	TTY    bool // standard input is a terminal
	Yes    bool // --yes: run without asking
	DryRun bool // --dry-run: show and stop
}

// Apply shows every step, asks, and runs the steps one by one with run, stopping at the first failure.
func Apply(steps []Step, c Confirm, run func(Step) error) error {
	fmt.Fprintln(c.Out, "switchboard runs these commands with sudo:")
	for _, s := range steps {
		fmt.Fprintf(c.Out, "  %s\n", s)
	}
	switch {
	case c.DryRun:
		fmt.Fprintln(c.Out, "dry run: nothing was run")
		return nil
	case c.Yes:
	case !c.TTY:
		return errors.New("standard input is not a terminal; pass --yes to run these commands without asking")
	default:
		fmt.Fprint(c.Out, "Run them? [y/N] ")
		answer, _ := bufio.NewReader(c.In).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			return ErrDeclined
		}
	}
	for _, s := range steps {
		if err := run(s); err != nil {
			return fmt.Errorf("%s: %w", s, err)
		}
	}
	return nil
}

// Status writes whether the unit is installed, which credentials it loads, and what systemctl says of the service.
// It reads the unit with read and asks systemctl (is-enabled, is-active) as the user: nothing needs root.
func Status(w io.Writer, s Setup, read func(path string) (string, error), systemctl func(args ...string) string) {
	unit, err := read(s.UnitPath())
	if err != nil {
		fmt.Fprintln(w, "unit: not installed")
	} else {
		fmt.Fprintf(w, "unit: %s\n", s.UnitPath())
		var names []string
		for _, l := range strings.Split(unit, "\n") {
			if v, ok := strings.CutPrefix(l, "LoadCredentialEncrypted="); ok {
				names = append(names, strings.SplitN(v, ":", 2)[0])
			}
		}
		fmt.Fprintf(w, "credentials: %s\n", strings.Join(names, ", "))
	}
	fmt.Fprintf(w, "enabled: %s\n", systemctl("is-enabled", s.UnitName()))
	fmt.Fprintf(w, "active: %s\n", systemctl("is-active", s.UnitName()))
}
