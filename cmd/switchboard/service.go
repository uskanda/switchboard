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

// setup describes the service for cfg loaded from cfgPath: this user, this binary, this PATH.
func setup(cfg *config.Config, cfgPath string) (service.Setup, error) {
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
	return service.Setup{User: u.Username, Home: u.HomeDir, Path: os.Getenv("PATH"), Binary: bin, Config: abs, Creds: service.Credentials(cfg)}, nil
}

// serviceFlags parses the options of install and uninstall, and returns the configuration and its path.
func serviceFlags(name string, args []string, stderr io.Writer, c *service.Confirm) (*config.Config, string, int) {
	var cfgPath string
	cfg, fs, code := parse(name, args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&c.DryRun, "dry-run", false, "show the commands that would run with sudo, and stop")
		fs.BoolVar(&c.Yes, "yes", false, "run the commands without asking")
	})
	if code >= 0 {
		return nil, "", code
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "usage: switchboard %s [--dry-run] [--yes] [--config FILE]\n", name)
		return nil, "", 2
	}
	cfgPath = fs.Lookup("config").Value.String()
	return cfg, cfgPath, -1
}

func serviceInstall(args []string, stdout, stderr io.Writer) int {
	c := service.Confirm{In: os.Stdin, Out: stdout, TTY: isTerminal(os.Stdin)}
	cfg, cfgPath, code := serviceFlags("service install", args, stderr, &c)
	if code >= 0 {
		return code
	}
	s, err := setup(cfg, cfgPath)
	if err != nil {
		return fail(stderr, err)
	}
	if len(s.Creds) == 0 {
		return fail(stderr, fmt.Errorf("no project of this host (%s); nothing to run", cfg.Host.Name))
	}
	unit, err := os.CreateTemp("", "switchboard-agent-*.service")
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
	fmt.Fprintf(stdout, "The unit written to %s:\n\n%s\n", service.UnitPath, s.Unit())
	if err := service.Apply(steps, c, sudo); err != nil {
		if errors.Is(err, service.ErrDeclined) {
			fmt.Fprintln(stderr, "switchboard: nothing was run")
			return 1
		}
		return fail(stderr, err)
	}
	if !c.DryRun {
		fmt.Fprintf(stdout, "installed %s; see it with: switchboard service status, journalctl -u %s\n", service.Name, service.Name)
	}
	return 0
}

func serviceUninstall(args []string, stdout, stderr io.Writer) int {
	c := service.Confirm{In: os.Stdin, Out: stdout, TTY: isTerminal(os.Stdin)}
	cfg, cfgPath, code := serviceFlags("service uninstall", args, stderr, &c)
	if code >= 0 {
		return code
	}
	s, err := setup(cfg, cfgPath)
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
		fmt.Fprintf(stdout, "removed %s\n", service.Name)
	}
	return 0
}

func serviceStatus(args []string, stdout, stderr io.Writer) int {
	if _, fs, code := parse("service status", args, stderr, nil); code >= 0 {
		return code
	} else if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: switchboard service status [--config FILE]")
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
	service.Status(stdout, read, systemctl)
	return 0
}
