package relay

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

// A webhook sent to the receiver comes out of the SSE stream as one event, through the two separate listeners.
func TestServeCarriesAWebhookToSubscribers(t *testing.T) {
	whLn, _ := net.Listen("tcp", "127.0.0.1:0")
	sseLn, _ := net.Listen("tcp", "127.0.0.1:0")
	cfg := config.Relay{
		Hooks: []config.Hook{{Project: "gl:nas/switchboard-sandbox", Key: glKey, SecretRef: "r/gl", Forge: "gl", Owner: "nas", Repo: "switchboard-sandbox"}},
		Hosts: []config.RelayHost{{Name: "ubuntu-a", TokenHash: hashOf("tok-a")}},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, cfg, whLn, sseLn, func(string) (string, error) { return signingKey, nil }, func(string, ...any) {})
	}()

	sub, subCancel := context.WithCancel(context.Background())
	defer subCancel()
	resp, sc := subscribe(t, sub, "http://"+sseLn.Addr().String(), "tok-a", "")
	if resp.StatusCode != 200 {
		t.Fatalf("subscribe: %d", resp.StatusCode)
	}
	// The webhook listener does not serve the stream, and the stream listener does not take webhooks.
	if r, err := http.Get("http://" + whLn.Addr().String() + "/events"); err != nil || r.StatusCode != 404 {
		t.Fatalf("/events on the webhook listener: %v %v", r, err)
	}

	ts := time.Now()
	req, _ := http.NewRequest("POST", "http://"+whLn.Addr().String()+"/webhook/gitlab/"+glKey, strings.NewReader(notePayload))
	req.Header.Set("X-Gitlab-Event", "Note Hook")
	req.Header.Set("webhook-id", "id-1")
	req.Header.Set("webhook-timestamp", strconv.FormatInt(ts.Unix(), 10))
	req.Header.Set("webhook-signature", gitlabSign(signingKey, "id-1", ts, notePayload))
	r, err := http.DefaultClient.Do(req)
	if err != nil || r.StatusCode != 200 {
		t.Fatalf("webhook: %v %v", r, err)
	}
	if _, data := next(t, sc); !strings.Contains(data, `"issue":4`) || !strings.Contains(data, `"delivery":"id-1"`) {
		t.Fatalf("data %s", data)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop")
	}
}
