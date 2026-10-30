//go:build !linux

package main

import "os"

// isTerminal reports whether f is a character device. Off Linux, which switchboard does not support yet, this is the
// best guess without reading the terminal settings.
func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
