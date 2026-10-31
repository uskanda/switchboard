// Package editor opens a file in the user's editor, as git does: $EDITOR is a shell command line, vi when unset.
package editor

import (
	"fmt"
	"os"
	"os/exec"
)

// Default is $EDITOR, or vi when it is unset.
func Default() string {
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

// Run opens path in editor and waits for it, on the terminal of this process.
func Run(editor, path string) error {
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %s: %w", editor, err)
	}
	return nil
}
