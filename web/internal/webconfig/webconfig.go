// Package webconfig reads web.yaml, the settings of the web inbox. They are kept apart from the core's
// config.yaml, which knows nothing of the web inbox.
package webconfig

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is web.yaml.
type Config struct {
	// Listen is the address to serve on: an IP address of this host and a port. There is no default, and every
	// interface (0.0.0.0, ::) is refused: which network sees the inbox is for the user to decide.
	Listen string `yaml:"listen"`
	// Socket is the inbox service's socket; inboxapi.DefaultSocket when empty.
	Socket string `yaml:"socket"`
}

// DefaultPath is $XDG_CONFIG_HOME/switchboard/web.yaml, with ~/.config when XDG_CONFIG_HOME is unset.
func DefaultPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "switchboard", "web.yaml")
}

// Load reads and checks web.yaml at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read the web configuration: %w", err)
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) check() error {
	if c.Listen == "" {
		return errors.New("listen is required: write the LAN address of this host and a port, such as 192.168.0.82:8783")
	}
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil || port == "" {
		return fmt.Errorf("listen must be an IP address and a port, got %q", c.Listen)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("listen must name an IP address, not %q: the Host check compares against this one value", host)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("listen %q would serve on every interface; write the LAN address of this host instead", c.Listen)
	}
	return nil
}
