package inboxserve

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "il") // short: a socket path is limited to about 100 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestListenMakesAPrivateSocket(t *testing.T) {
	sock := filepath.Join(tempDir(t), "switchboard", "inbox.sock")
	ln, err := Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if ln.Addr().Network() != "unix" {
		t.Fatalf("network = %s", ln.Addr().Network())
	}
	st, err := os.Stat(sock)
	if err != nil || st.Mode()&os.ModeSocket == 0 || st.Mode().Perm() != 0o600 {
		t.Fatalf("socket mode = %v, %v", st.Mode(), err)
	}
	if d, _ := os.Stat(filepath.Dir(sock)); d.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %v", d.Mode())
	}
}

func TestListenReplacesAStaleSocket(t *testing.T) {
	sock := filepath.Join(tempDir(t), "inbox.sock")
	old, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	old.(*net.UnixListener).SetUnlinkOnClose(false)
	old.Close() // leaves the file behind, as a killed server does
	ln, err := Listen(sock)
	if err != nil {
		t.Fatalf("stale socket: %v", err)
	}
	ln.Close()
}

func TestListenRefusesAnotherFileAndALiveServer(t *testing.T) {
	dir := tempDir(t)
	plain := filepath.Join(dir, "plain")
	os.WriteFile(plain, []byte("x"), 0o600)
	if _, err := Listen(plain); err == nil {
		t.Fatal("a regular file was replaced")
	}
	sock := filepath.Join(dir, "inbox.sock")
	live, err := Listen(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	if _, err := Listen(sock); err == nil {
		t.Fatal("a second server took the socket of a live one")
	}
}
