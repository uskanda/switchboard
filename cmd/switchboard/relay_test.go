package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/uskanda/switchboard/internal/cli"
)

func TestRelayServeNeedsAHost(t *testing.T) {
	var out, errb bytes.Buffer
	if code := cli.Run([]string{"relay", "serve", "--config", writeConfig(t)}, &out, &errb); code != 1 || !strings.Contains(errb.String(), "relay token add") {
		t.Fatalf("code %d, stderr %s", code, errb.String())
	}
}
