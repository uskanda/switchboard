// Package cli parses the switchboard command line and dispatches to the subcommands.
package cli

import (
	"fmt"
	"io"
	"strings"
)

// Handler runs one subcommand with the arguments that follow its name and returns the process exit code.
type Handler func(args []string, stdout, stderr io.Writer) int

// command is one entry of the command table: its words on the command line, a one-line summary, and the handler.
type command struct {
	words   []string
	summary string
	run     Handler
}

// commands is the command table. Handlers are filled in by the packages that implement them through Register.
var commands = []*command{
	{words: []string{"labels", "sync"}, summary: "create the state labels in every configured project"},
	{words: []string{"agent", "run"}, summary: "poll the projects assigned to this host and run claude on their issues"},
	{words: []string{"agent", "status"}, summary: "show the projects of this host and the issues being run"},
	{words: []string{"inbox"}, summary: "list the waiting issues of every configured project"},
	{words: []string{"show"}, summary: "show the last question of one waiting issue"},
	{words: []string{"reply"}, summary: "post a reply to one waiting issue"},
	{words: []string{"doctor"}, summary: "check the configuration, the tokens and the claude CLI"},
	{words: []string{"service", "install"}, summary: "run the agent as a systemd service, after showing what sudo will run"},
	{words: []string{"service", "uninstall"}, summary: "stop the systemd service and remove its unit and credentials"},
	{words: []string{"service", "status"}, summary: "show whether the systemd service is installed and running"},
}

// Register sets the handler of the command named by words, such as "agent run". It panics on an unknown name,
// which is a programming error.
func Register(name string, h Handler) {
	for _, c := range commands {
		if strings.Join(c.words, " ") == name {
			c.run = h
			return
		}
	}
	panic("cli: unknown command " + name)
}

// Run parses args (without the program name), runs the matching subcommand, and returns the exit code:
// 0 on success, 1 when the command failed, 2 for a usage error.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage(stdout)
		return 0
	}
	for _, c := range commands {
		n := len(c.words)
		if len(args) >= n && equal(args[:n], c.words) {
			if c.run == nil {
				fmt.Fprintf(stderr, "switchboard: %s is not implemented yet\n", strings.Join(c.words, " "))
				return 1
			}
			return c.run(args[n:], stdout, stderr)
		}
	}
	if groups()[args[0]] {
		fmt.Fprintf(stderr, "switchboard: %s needs a subcommand\n", args[0])
	} else {
		fmt.Fprintf(stderr, "switchboard: unknown command %q\n", args[0])
	}
	usage(stderr)
	return 2
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: switchboard <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	names := make([]string, 0, len(commands))
	width := 0
	for _, c := range commands {
		name := strings.Join(c.words, " ")
		names = append(names, name)
		if len(name) > width {
			width = len(name)
		}
	}
	for i, c := range commands {
		fmt.Fprintf(w, "  %-*s  %s\n", width, names[i], c.summary)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Every command reads the configuration from --config, else $XDG_CONFIG_HOME/switchboard/config.yaml.")
}

// groups returns the first words of the commands that take a second word ("labels", "agent").
func groups() map[string]bool {
	g := map[string]bool{}
	for _, c := range commands {
		if len(c.words) > 1 {
			g[c.words[0]] = true
		}
	}
	return g
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
