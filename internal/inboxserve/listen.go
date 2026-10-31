package inboxserve

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Listen listens on the Unix socket at path, readable and writable by this user only: the file's mode is what keeps
// other users from the API. The parent directory is made (0700) when missing. A socket left behind by a server that
// is gone is replaced; a live server's socket and any other file are not.
func Listen(path string) (net.Listener, error) {
	if path == "" {
		return nil, errors.New("no socket path: pass --socket, or run where RUNTIME_DIRECTORY or XDG_RUNTIME_DIR is set")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if st, err := os.Lstat(path); err == nil {
		if st.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("%s exists and is not a socket", path)
		}
		if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
			c.Close()
			return nil, fmt.Errorf("%s is in use by a running inbox service", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}
