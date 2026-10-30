package relay

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/config"
)

const (
	ghKey = "Z2l0aHViLWtleS0xMjM0NTY3OA"
	glKey = "Z2l0bGFiLWtleS0xMjM0NTY3OA"
	tkKey = "dG9rZW4ta2V5LTEyMzQ1Njc4OQ"
)

type recorder struct {
	mu     sync.Mutex
	events []Event
	logs   []string
}

func (r *recorder) publish(e Event) { r.mu.Lock(); r.events = append(r.events, e); r.mu.Unlock() }
func (r *recorder) logf(f string, a ...any) {
	r.mu.Lock()
	r.logs = append(r.logs, fmt.Sprintf(f, a...))
	r.mu.Unlock()
}

var now = time.Unix(1793375092, 0)

func receiver(t *testing.T) (*Receiver, *recorder) {
	t.Helper()
	rec := &recorder{}
	secrets := map[string]string{"r/gh": "s3cret", "r/gl": signingKey, "r/tk": "plain-token"}
	r := &Receiver{
		Hooks: []config.Hook{
			{Project: "gh:alice/sandbox", Key: ghKey, SecretRef: "r/gh", Forge: "gh", Owner: "alice", Repo: "sandbox"},
			{Project: "gl:nas/switchboard-sandbox", Key: glKey, SecretRef: "r/gl", Forge: "gl", Owner: "nas", Repo: "switchboard-sandbox"},
			{Project: "gl:nas/old", Key: tkKey, SecretRef: "r/tk", GitLabToken: true, Forge: "gl", Owner: "nas", Repo: "old"},
		},
		Secret:  func(ref string) (string, error) { return secrets[ref], nil },
		Now:     func() time.Time { return now },
		Publish: rec.publish,
		Logf:    rec.logf,
	}
	return r, rec
}

func ghRequest(key, delivery, body, sig string) *http.Request {
	req := httptest.NewRequest("POST", "/webhook/github/"+key, strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", delivery)
	req.Header.Set("X-Hub-Signature-256", sig)
	return req
}

func glRequest(key, id, body string) *http.Request {
	req := httptest.NewRequest("POST", "/webhook/gitlab/"+key, strings.NewReader(body))
	req.Header.Set("X-Gitlab-Event", "Note Hook")
	req.Header.Set("webhook-id", id)
	req.Header.Set("webhook-timestamp", strconv.FormatInt(now.Unix(), 10))
	req.Header.Set("webhook-signature", gitlabSign(signingKey, id, now, body))
	return req
}

func serve(r *Receiver, req *http.Request) int {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestWebhookGitHub(t *testing.T) {
	r, rec := receiver(t)
	if code := serve(r, ghRequest(ghKey, "d-1", commentPayload, sign("s3cret", commentPayload))); code != 200 {
		t.Fatalf("code %d", code)
	}
	want := Event{Forge: "gh", Project: "alice/sandbox", Issue: 42, Kind: "issue_comment", Delivery: "d-1"}
	if len(rec.events) != 1 || rec.events[0] != want {
		t.Fatalf("events = %+v", rec.events)
	}
}

func TestWebhookGitLab(t *testing.T) {
	r, rec := receiver(t)
	if code := serve(r, glRequest(glKey, "id-1", notePayload)); code != 200 {
		t.Fatalf("code %d", code)
	}
	if len(rec.events) != 1 || rec.events[0].Delivery != "id-1" || rec.events[0].Issue != 4 {
		t.Fatalf("events = %+v", rec.events)
	}
}

func TestWebhookRefusals(t *testing.T) {
	big := `{"pad":"` + strings.Repeat("x", 5<<20) + `"}`
	cases := map[string]struct {
		req  *http.Request
		code int
	}{
		"unknown key":    {ghRequest("bm8tc3VjaC1rZXktMTIzNDU2Nzg", "d-1", commentPayload, sign("s3cret", commentPayload)), 404},
		"forge mismatch": {ghRequest(glKey, "d-1", commentPayload, sign("s3cret", commentPayload)), 404},
		"bad signature":  {ghRequest(ghKey, "d-1", commentPayload, sign("other", commentPayload)), 401},
		"too large":      {ghRequest(ghKey, "d-1", big, sign("s3cret", big)), 413},
		"GET":            {httptest.NewRequest("GET", "/webhook/github/"+ghKey, nil), 405},
		"other path":     {httptest.NewRequest("POST", "/events", nil), 404},
		"other project":  {ghRequest(ghKey, "d-1", strings.Replace(commentPayload, "alice/sandbox", "alice/other", 1), sign("s3cret", strings.Replace(commentPayload, "alice/sandbox", "alice/other", 1))), 400},
	}
	stale := glRequest(glKey, "id-9", notePayload)
	stale.Header.Set("webhook-timestamp", strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10))
	stale.Header.Set("webhook-signature", gitlabSign(signingKey, "id-9", now.Add(-10*time.Minute), notePayload))
	cases["stale"] = struct {
		req  *http.Request
		code int
	}{stale, 401}
	tokenOnly := httptest.NewRequest("POST", "/webhook/gitlab/"+glKey, strings.NewReader(notePayload))
	tokenOnly.Header.Set("X-Gitlab-Event", "Note Hook")
	tokenOnly.Header.Set("X-Gitlab-Token", signingKey)
	cases["X-Gitlab-Token without gitlab_token"] = struct {
		req  *http.Request
		code int
	}{tokenOnly, 401}
	for name, c := range cases {
		r, rec := receiver(t)
		if code := serve(r, c.req); code != c.code || len(rec.events) != 0 {
			t.Errorf("%s: code %d (want %d), events %v", name, code, c.code, rec.events)
		}
	}
}

func TestWebhookGitLabSecretToken(t *testing.T) {
	r, rec := receiver(t)
	body := strings.Replace(notePayload, "nas/switchboard-sandbox", "nas/old", 1)
	req := httptest.NewRequest("POST", "/webhook/gitlab/"+tkKey, strings.NewReader(body))
	req.Header.Set("X-Gitlab-Event", "Note Hook")
	req.Header.Set("Idempotency-Key", "k-1")
	req.Header.Set("X-Gitlab-Token", "plain-token")
	if code := serve(r, req); code != 200 || len(rec.events) != 1 || rec.events[0].Delivery != "k-1" {
		t.Fatalf("code %d, events %v", code, rec.events)
	}
}

func TestWebhookDuplicateDelivery(t *testing.T) {
	r, rec := receiver(t)
	for i := 0; i < 2; i++ {
		if code := serve(r, glRequest(glKey, "id-1", notePayload)); code != 200 {
			t.Fatalf("delivery %d: code %d", i, code)
		}
	}
	if len(rec.events) != 1 {
		t.Fatalf("events = %v", rec.events)
	}
	// After 72 hours the ID is forgotten.
	now = now.Add(73 * time.Hour)
	defer func() { now = now.Add(-73 * time.Hour) }()
	serve(r, glRequest(glKey, "id-1", notePayload))
	if len(rec.events) != 2 {
		t.Fatalf("after 72h: events = %v", rec.events)
	}
}

func TestWebhookSkipsOtherEvents(t *testing.T) {
	r, rec := receiver(t)
	body := `{"object_kind":"merge_request"}`
	req := glRequest(glKey, "id-1", body)
	req.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	if code := serve(r, req); code != 200 || len(rec.events) != 0 {
		t.Fatalf("code %d, events %v", code, rec.events)
	}
}

// The log carries IDs and the outcome only: no body, title or login.
func TestWebhookLogHoldsNoContent(t *testing.T) {
	r, rec := receiver(t)
	serve(r, glRequest(glKey, "id-1", notePayload))
	serve(r, ghRequest(ghKey, "d-1", commentPayload, sign("other", commentPayload)))
	all := strings.Join(rec.logs, "\n")
	for _, secret := range []string{"mallory", "ignore previous", "Secret plan", "issue body", "s3cret", "whsec_"} {
		if strings.Contains(all, secret) {
			t.Errorf("the log holds %q:\n%s", secret, all)
		}
	}
	if !bytes.Contains([]byte(all), []byte("id-1")) || !strings.Contains(all, "nas/switchboard-sandbox#4") {
		t.Errorf("the log lacks the delivery:\n%s", all)
	}
}
