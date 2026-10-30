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

// The places the service uses.
const (
	Name     = "switchboard-agent.service"
	UnitPath = "/etc/systemd/system/" + Name
	CredDir  = "/etc/credstore.encrypted"
)

// Credential is one agent token, by its token_ref and its credential name.
type Credential struct {
	Ref  string
	Name string
}

// Path is where the encrypted credential is kept.
func (c Credential) Path() string { return CredDir + "/switchboard." + c.Name }

// Credentials lists the agent tokens of this host's projects: the forges' token_ref and the projects' overrides.
// The inbox tokens are left out; the service never reads or replies as the user.
func Credentials(cfg *config.Config) []Credential {
	var out []Credential
	seen := map[string]bool{}
	for _, p := range cfg.ProjectsFor(cfg.Host.Name) {
		ref := cfg.AgentTokenRef(p)
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
	User, Home, Path string // the account the agent runs as, its home, and the PATH that finds claude and git
	Binary, Config   string // absolute paths of switchboard and of the configuration
	Creds            []Credential
}

// Unit is the systemd unit of the service.
func (s Setup) Unit() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Unit]\nDescription=switchboard agent\nAfter=network-online.target\nWants=network-online.target\n\n")
	fmt.Fprintf(&b, "[Service]\nUser=%s\nEnvironment=HOME=%s\nEnvironment=PATH=%s\n", s.User, s.Home, s.Path)
	fmt.Fprintf(&b, "ExecStart=%s agent run --config %s\n", s.Binary, s.Config)
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
		Step{Args: []string{"install", "-m", "0644", unitFile, UnitPath}},
		Step{Args: []string{"systemctl", "daemon-reload"}},
		Step{Args: []string{"systemctl", "enable", Name}},
		Step{Args: []string{"systemctl", "restart", Name}},
	), nil
}

// UninstallSteps stops and disables the service, and removes the unit and the credentials.
func (s Setup) UninstallSteps() []Step {
	steps := []Step{
		{Args: []string{"systemctl", "disable", "--now", Name}},
		{Args: []string{"rm", "-f", UnitPath}},
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
func Status(w io.Writer, read func(path string) (string, error), systemctl func(args ...string) string) {
	unit, err := read(UnitPath)
	if err != nil {
		fmt.Fprintln(w, "unit: not installed")
	} else {
		fmt.Fprintf(w, "unit: %s\n", UnitPath)
		var names []string
		for _, l := range strings.Split(unit, "\n") {
			if v, ok := strings.CutPrefix(l, "LoadCredentialEncrypted="); ok {
				names = append(names, strings.SplitN(v, ":", 2)[0])
			}
		}
		fmt.Fprintf(w, "credentials: %s\n", strings.Join(names, ", "))
	}
	fmt.Fprintf(w, "enabled: %s\n", systemctl("is-enabled", Name))
	fmt.Fprintf(w, "active: %s\n", systemctl("is-active", Name))
}
