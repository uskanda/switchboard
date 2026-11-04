package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/instructions"
	"github.com/uskanda/switchboard/internal/relayclient"
	"github.com/uskanda/switchboard/internal/state"
	"github.com/uskanda/switchboard/internal/webauth"
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

// syncBuffer is a bytes.Buffer safe to write from one goroutine while the test reads it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// With inbox.relay, the inbox service follows the relay with its own token and logs when it connects.
func TestInboxServeFollowsTheRelay(t *testing.T) {
	cfg := load(t, bothForges+"inbox:\n  relay: {url: \"http://127.0.0.1:8782/events\", token_ref: relay-inbox}\n")
	api, _ := inboxAPI(cfg, map[string]forge.Forge{}, instructions.Store{})
	if !api.Relay {
		t.Fatal("the server does not know it follows a relay")
	}
	var auth atomic.Value
	relaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "id: 1\ndata: {\"forge\":\"gl\",\"project\":\"nas/b\",\"issue\":3,\"kind\":\"issue\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer relaySrv.Close()
	stderr := &syncBuffer{}
	rc := &relayclient.Client{URL: relaySrv.URL, Token: "tok", Logf: func(string, ...any) {}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { followRelay(ctx, rc, api, stderr); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(stderr.String(), "relay: connected") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if !strings.Contains(stderr.String(), "switchboard: inbox: relay: connected") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if got, _ := auth.Load().(string); got != "Bearer tok" {
		t.Fatalf("authorization = %q", got)
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

func TestInboxPushFailsWithoutItsKey(t *testing.T) {
	cfg := load(t, bothForges+"inbox:\n  notify:\n    fcm: {project_id: p, credentials_ref: switchboard/fcm}\n")
	missing := func(ref string) (string, error) { return "", fmt.Errorf("no token for %s in the Secret Service", ref) }
	if _, err := inboxPush(cfg, missing, state.Dir{Path: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "switchboard/fcm") {
		t.Fatalf("err = %v, want the credentials_ref named", err)
	}
	broken := func(string) (string, error) { return "not-a-key-SECRET-VALUE", nil }
	_, err := inboxPush(cfg, broken, state.Dir{Path: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "switchboard/fcm") || strings.Contains(err.Error(), "SECRET-VALUE") {
		t.Fatalf("err = %v, want the ref named and the content left out", err)
	}
}

func TestInboxPushIsOffWithoutTheSetting(t *testing.T) {
	cfg := load(t, bothForges)
	n, err := inboxPush(cfg, func(string) (string, error) { t.Fatal("a key was read"); return "", nil }, state.Dir{Path: t.TempDir()})
	if err != nil || n != nil {
		t.Fatalf("notifier = %v, err = %v", n, err)
	}
}

const inboxAuthSetting = "inbox:\n  auth: {client_id: cid, client_secret_ref: switchboard/gitlab-oidc, member_groups: [team]}\n"

func TestInboxAuthReadsTheClientSecret(t *testing.T) {
	cfg := load(t, bothForges+inboxAuthSetting)
	var read []string
	a, err := inboxAuth(cfg, func(ref string) (string, error) { read = append(read, ref); return "the-secret", nil })
	if err != nil {
		t.Fatal(err)
	}
	g, ok := a.Provider.(*webauth.GitLab)
	if !ok || g.ClientID != "cid" || g.ClientSecret != "the-secret" || g.BaseURL != cfg.Forges.GitLab.BaseURL {
		t.Fatalf("provider = %+v", a.Provider)
	}
	if len(a.Allow.MemberGroups) != 1 || a.Allow.MemberGroups[0] != "team" {
		t.Errorf("allow = %+v", a.Allow)
	}
	if len(read) != 1 || read[0] != "switchboard/gitlab-oidc" {
		t.Errorf("read %v, want the client secret only", read)
	}
}

func TestInboxAuthFailsWithoutTheClientSecret(t *testing.T) {
	cfg := load(t, bothForges+inboxAuthSetting)
	missing := func(ref string) (string, error) { return "", fmt.Errorf("no token for %s in the Secret Service", ref) }
	if _, err := inboxAuth(cfg, missing); err == nil || !strings.Contains(err.Error(), "switchboard/gitlab-oidc") {
		t.Fatalf("err = %v, want the client_secret_ref named", err)
	}
}

func TestInboxAuthIsOffWithoutTheSetting(t *testing.T) {
	cfg := load(t, bothForges)
	a, err := inboxAuth(cfg, func(string) (string, error) { t.Fatal("a secret was read"); return "", nil })
	if err != nil || a != nil {
		t.Fatalf("auth = %v, err = %v", a, err)
	}
}
