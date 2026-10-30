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

// setup describes the agent's service, or the relay's, for cfg loaded from cfgPath: this user, this binary, this PATH.
func setup(cfg *config.Config, cfgPath string, relay bool) (service.Setup, error) {
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
	creds := service.Credentials(cfg)
	if relay {
		creds = service.RelayCredentials(cfg)
	}
	return service.Setup{User: u.Username, Home: u.HomeDir, Path: os.Getenv("PATH"), Binary: bin, Config: abs, Creds: creds, Relay: relay}, nil
}

// serviceFlags parses the options of install and uninstall, and returns the configuration and its path.
// It also reports whether --relay chose the relay's service.
func serviceFlags(name string, args []string, stderr io.Writer, c *service.Confirm) (*config.Config, string, bool, int) {
	var relay bool
	cfg, fs, code := parse(name, args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&c.DryRun, "dry-run", false, "show the commands that would run with sudo, and stop")
		fs.BoolVar(&c.Yes, "yes", false, "run the commands without asking")
		fs.BoolVar(&relay, "relay", false, "the relay's service (relay serve) instead of the agent's")
	})
	if code >= 0 {
		return nil, "", false, code
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "usage: switchboard %s [--relay] [--dry-run] [--yes] [--config FILE]\n", name)
		return nil, "", false, 2
	}
	return cfg, fs.Lookup("config").Value.String(), relay, -1
}

func serviceInstall(args []string, stdout, stderr io.Writer) int {
	c := service.Confirm{In: os.Stdin, Out: stdout, TTY: isTerminal(os.Stdin)}
	cfg, cfgPath, relay, code := serviceFlags("service install", args, stderr, &c)
	if code >= 0 {
		return code
	}
	s, err := setup(cfg, cfgPath, relay)
	if err != nil {
		return fail(stderr, err)
	}
	switch {
	case relay && len(cfg.Relay.Hosts) == 0:
		return fail(stderr, errors.New("relay.hosts is empty: no agent could subscribe; add one with `switchboard relay token add <host>`"))
	case relay && len(s.Creds) == 0:
		return fail(stderr, errors.New("relay.hooks is empty: the relay would take no webhook; add one with `switchboard relay hook add <project>`"))
	case !relay && len(s.Creds) == 0:
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
		if relay {
			status += " --relay"
		}
		fmt.Fprintf(stdout, "installed %s; see it with: %s, journalctl -u %s\n", s.UnitName(), status, s.UnitName())
	}
	return 0
}

func serviceUninstall(args []string, stdout, stderr io.Writer) int {
	c := service.Confirm{In: os.Stdin, Out: stdout, TTY: isTerminal(os.Stdin)}
	cfg, cfgPath, relay, code := serviceFlags("service uninstall", args, stderr, &c)
	if code >= 0 {
		return code
	}
	s, err := setup(cfg, cfgPath, relay)
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
	var relay bool
	if _, fs, code := parse("service status", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&relay, "relay", false, "the relay's service instead of the agent's")
	}); code >= 0 {
		return code
	} else if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: switchboard service status [--relay] [--config FILE]")
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
	service.Status(stdout, service.Setup{Relay: relay}, read, systemctl)
	return 0
}
