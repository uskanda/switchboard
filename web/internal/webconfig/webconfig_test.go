package webconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "web.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad(t *testing.T) {
	cfg, err := Load(write(t, "listen: 192.168.0.82:8783\nsocket: /tmp/inbox.sock\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "192.168.0.82:8783" || cfg.Socket != "/tmp/inbox.sock" {
		t.Fatalf("cfg = %+v", cfg)
	}
	for _, ok := range []string{"127.0.0.1:8783", "[fd00::5]:8783"} {
		if _, err := Load(write(t, "listen: \""+ok+"\"\n")); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
}

func TestLoadRejectsAnUnsafeListen(t *testing.T) {
	for name, c := range map[string]struct{ body, want string }{
		"missing":     {"socket: /tmp/x\n", "listen is required"},
		"empty host":  {"listen: \":8783\"\n", "IP address"},
		"all ipv4":    {"listen: 0.0.0.0:8783\n", "every interface"},
		"all ipv6":    {"listen: \"[::]:8783\"\n", "every interface"},
		"host name":   {"listen: usknas.local:8783\n", "IP address"},
		"no port":     {"listen: 192.168.0.82\n", "listen"},
		"unknown key": {"listen: 192.168.0.82:8783\ntoken: x\n", "token"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(write(t, c.body))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	if got := DefaultPath(); got != "/cfg/switchboard/web.yaml" {
		t.Fatalf("DefaultPath = %q", got)
	}
}
