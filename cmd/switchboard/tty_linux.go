package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// isTerminal reports whether f is a terminal: its terminal settings can be read. /dev/null is a character device
// too, so the file mode alone cannot tell.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	return err == nil
}
