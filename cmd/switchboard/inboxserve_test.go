package main

import (
	"bytes"
	"strings"
	"testing"
)

const bothForges = `version: 1
host: {name: here}
forges:
  github: {token_ref: gh-agent, inbox_token_ref: gh-inbox, agent_identity: bot}
  gitlab: {base_url: "https://gl.test", token_ref: gl-agent, inbox_token_ref: gl-inbox, agent_identity: me}
trusted_authors: {github: [me], gitlab: [me]}
projects:
  - {ref: "gh:o/r", path: /src/r, host: here}
  - {ref: "gl:nas/b", path: /src/b, host: here, token_ref: gl-bot, agent_identity: project_7_bot}
`

func TestInboxServeReadsTheInboxTokensOnly(t *testing.T) {
	cfg := load(t, bothForges)
	var asked []string
	b, err := serveInbox(newConn(cfg, tokens(&asked)))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Forges) != 2 || b.Only != "" {
		t.Fatalf("inbox = %+v", b)
	}
	if got := strings.Join(asked, " "); got != "gh-inbox gl-inbox" {
		t.Fatalf("asked for %q", got)
	}
}

func TestInboxServeSocketPath(t *testing.T) {
	t.Setenv("RUNTIME_DIRECTORY", "/run/switchboard")
	if got := socketPath(""); got != "/run/switchboard/inbox.sock" {
		t.Fatalf("default = %q", got)
	}
	if got := socketPath("/tmp/x.sock"); got != "/tmp/x.sock" {
		t.Fatalf("--socket = %q", got)
	}
}

func TestInboxServeTakesNoArguments(t *testing.T) {
	var out, errOut bytes.Buffer
	p := writeConfig(t)
	if code := inboxServe([]string{"--config", p, "extra"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage") {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
}
