// Command switchboard lists the issues on which Claude Code waits for a human, across forges and machines, and runs
// Claude Code on them. See README.md.
package main

import (
	"os"

	"github.com/uskanda/switchboard/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
