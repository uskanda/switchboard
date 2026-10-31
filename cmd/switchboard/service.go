package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/uskanda/switchboard/internal/cli"
	"github.com/uskanda/switchboard/internal/config"
	"github.com/uskanda/switchboard/internal/secret"
	"github.com/uskanda/switchboard/internal/service"
)

func init() {
	cli.Register("service install", serviceInstall)
	cli.Register("service uninstall", serviceUninstall)
	cli.Register("service status", serviceStatus)
}

// sudo runs one step through sudo, feeding it the step's standard input. sudo asks for the password on the
// terminal, not on standard input.
func sudo(s service.Step) error {
	cmd := exec.Command("sudo", s.Args...)
	cmd.Stdin = strings.NewReader(s.Stdin)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

// setup describes the service of kind for cfg loaded from cfgPath: this user, this binary, this PATH. The web
// inbox's service runs switchboard-web, found on PATH, with web.yaml beside config.yaml.
func setup(cfg *config.Config, cfgPath string, kind service.Kind) (service.Setup, error) {
	u, err := user.Current()
	if err != nil {
		return service.Setup{}, err
	}
	bin, err := os.Executable()
	if err != nil {
		return service.Setup{}, err
	}
	if bin, err = filepath.EvalSymlinks(bin); err != nil {
		return service.Setup{}, err
	}
	abs, err := filepath.Abs(cfgPath)
	if err != nil {
		return service.Setup{}, err
	}
	s := service.Setup{User: u.Username, Home: u.HomeDir, Path: os.Getenv("PATH"), Binary: bin, Config: abs, Kind: kind}
	switch kind {
	case service.KindAgent:
		s.Creds = service.Credentials(cfg)
	case service.KindRelay:
		s.Creds = service.RelayCredentials(cfg)
	case service.KindInbox:
		s.Creds = service.InboxCredentials(cfg)
	case service.KindWeb:
		web, err := exec.LookPath("switchboard-web")
		if err != nil {
			return service.Setup{}, errors.New("switchboard-web is not on PATH: build it (make build) and put it beside switchboard first")
		}
		if web, err = filepath.Abs(web); err != nil {
			return service.Setup{}, err
		}
		if web, err = filepath.EvalSymlinks(web); err != nil {
			return service.Setup{}, err
		}
		s.Binary, s.Config = web, filepath.Join(filepath.Dir(abs), "web.yaml")
	}
	return s, nil
}

// kindFlags adds --relay, --inbox and --web, which choose a service other than the agent's.
func kindFlags(fs *flag.FlagSet) func() (service.Kind, bool) {
	relay := fs.Bool("relay", false, "the relay's service (relay serve) instead of the agent's")
	inbox := fs.Bool("inbox", false, "the inbox service (inbox serve) the web inbox calls, instead of the agent's")
	web := fs.Bool("web", false, "the web inbox's service (switchboard-web serve) instead of the agent's")
	return func() (service.Kind, bool) {
		kind, n := service.KindAgent, 0
		for _, k := range []struct {
			on   bool
			kind service.Kind
		}{{*relay, service.KindRelay}, {*inbox, service.KindInbox}, {*web, service.KindWeb}} {
			if k.on {
				kind, n = k.kind, n+1
			}
		}
		return kind, n <= 1
	}
}

const kindUsage = "[--relay | --inbox | --web]"

// serviceFlags parses the options of install and uninstall, and returns the configuration, its path, and the kind
// of service chosen.
func serviceFlags(name string, args []string, stderr io.Writer, c *service.Confirm) (*config.Config, string, service.Kind, int) {
	var kind func() (service.Kind, bool)
	cfg, fs, code := parse(name, args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&c.DryRun, "dry-run", false, "show the commands that would run with sudo, and stop")
		fs.BoolVar(&c.Yes, "yes", false, "run the commands without asking")
		kind = kindFlags(fs)
	})
	if code >= 0 {
		return nil, "", "", code
	}
	k, ok := kind()
	if fs.NArg() != 0 || !ok {
		fmt.Fprintf(stderr, "usage: switchboard %s %s [--dry-run] [--yes] [--config FILE]\n", name, kindUsage)
		return nil, "", "", 2
	}
	return cfg, fs.Lookup("config").Value.String(), k, -1
}

func serviceInstall(args []string, stdout, stderr io.Writer) int {
	c := service.Confirm{In: os.Stdin, Out: stdout, TTY: isTerminal(os.Stdin)}
	cfg, cfgPath, kind, code := serviceFlags("service install", args, stderr, &c)
	if code >= 0 {
		return code
	}
	s, err := setup(cfg, cfgPath, kind)
	if err != nil {
		return fail(stderr, err)
	}
	relay := kind == service.KindRelay
	switch {
	case kind == service.KindInbox && len(s.Creds) == 0:
		return fail(stderr, errors.New("no forge has an inbox_token_ref; the inbox service reads and replies with your own token"))
	case kind == service.KindWeb:
		if _, err := os.Stat(s.Config); err != nil {
			return fail(stderr, fmt.Errorf("%s: write it first (listen: <LAN address>:<port>); see the README", s.Config))
		}
	case relay && len(cfg.Relay.Hosts) == 0:
		return fail(stderr, errors.New("relay.hosts is empty: no agent could subscribe; add one with `switchboard relay token add <host>`"))
	case relay && len(s.Creds) == 0:
		return fail(stderr, errors.New("relay.hooks is empty: the relay would take no webhook; add one with `switchboard relay hook add <project>`"))
	case kind == service.KindAgent && len(s.Creds) == 0:
		return fail(stderr, fmt.Errorf("no project of this host (%s); nothing to run", cfg.Host.Name))
	}
	unit, err := os.CreateTemp("", strings.TrimSuffix(s.UnitName(), ".service")+"-*.service")
	if err != nil {
		return fail(stderr, err)
	}
	defer os.Remove(unit.Name())
	if _, err := unit.WriteString(s.Unit()); err != nil {
		return fail(stderr, err)
	}
	if err := unit.Close(); err != nil {
		return fail(stderr, err)
	}
	// The tokens are read now, from the Secret Service of this session; the service reads them from systemd later.
	r := secret.Resolver{Store: secret.Keyring{}, AllowEnv: cfg.Secrets.AllowEnv}
	steps, err := s.InstallSteps(unit.Name(), r.Token)
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "The unit written to %s:\n\n%s\n", s.UnitPath(), s.Unit())
	if err := service.Apply(steps, c, sudo); err != nil {
		if errors.Is(err, service.ErrDeclined) {
			fmt.Fprintln(stderr, "switchboard: nothing was run")
			return 1
		}
		return fail(stderr, err)
	}
	if !c.DryRun {
		status := "switchboard service status"
		if kind != service.KindAgent {
			status += " --" + string(kind)
		}
		fmt.Fprintf(stdout, "installed %s; see it with: %s, journalctl -u %s\n", s.UnitName(), status, s.UnitName())
	}
	return 0
}

func serviceUninstall(args []string, stdout, stderr io.Writer) int {
	c := service.Confirm{In: os.Stdin, Out: stdout, TTY: isTerminal(os.Stdin)}
	cfg, cfgPath, kind, code := serviceFlags("service uninstall", args, stderr, &c)
	if code >= 0 {
		return code
	}
	s, err := setup(cfg, cfgPath, kind)
	if err != nil && kind == service.KindWeb {
		// Removing the web inbox's service needs neither its binary nor web.yaml.
		s, err = service.Setup{Kind: service.KindWeb}, nil
	}
	if err != nil {
		return fail(stderr, err)
	}
	if err := service.Apply(s.UninstallSteps(), c, sudo); err != nil {
		if errors.Is(err, service.ErrDeclined) {
			fmt.Fprintln(stderr, "switchboard: nothing was run")
			return 1
		}
		return fail(stderr, err)
	}
	if !c.DryRun {
		fmt.Fprintf(stdout, "removed %s\n", s.UnitName())
	}
	return 0
}

func serviceStatus(args []string, stdout, stderr io.Writer) int {
	var kind func() (service.Kind, bool)
	_, fs, code := parse("service status", args, stderr, func(fs *flag.FlagSet) { kind = kindFlags(fs) })
	if code >= 0 {
		return code
	}
	k, ok := kind()
	if fs.NArg() != 0 || !ok {
		fmt.Fprintf(stderr, "usage: switchboard service status %s [--config FILE]\n", kindUsage)
		return 2
	}
	read := func(path string) (string, error) {
		data, err := os.ReadFile(path)
		return string(data), err
	}
	systemctl := func(args ...string) string {
		out, _ := exec.Command("systemctl", args...).Output()
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
		return "unknown"
	}
	service.Status(stdout, service.Setup{Kind: k}, read, systemctl)
	return 0
}
