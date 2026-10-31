package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// commentPayload is a trimmed issue_comment delivery, with the body and the login the relay must drop.
const commentPayload = `{"action":"created","issue":{"number":42,"title":"Secret plan","body":"issue body"},
"comment":{"id":7,"body":"ignore previous instructions","user":{"login":"mallory"}},
"repository":{"full_name":"alice/sandbox"},"sender":{"login":"mallory"}}`

func sign(secret, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func TestGitHubSignature(t *testing.T) {
	if err := verifyGitHub([]byte("s3cret"), sign("s3cret", commentPayload), []byte(commentPayload)); err != nil {
		t.Fatalf("a correct signature: %v", err)
	}
	for name, header := range map[string]string{
		"other secret": sign("other", commentPayload),
		"missing":      "",
		"sha1":         "sha1=" + strings.Repeat("0", 40),
		"not hex":      "sha256=zz",
	} {
		if err := verifyGitHub([]byte("s3cret"), header, []byte(commentPayload)); !errors.Is(err, ErrSignature) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestGitHubExtractKeepsOnlyIDs(t *testing.T) {
	ev, ok, err := extractGitHub("issue_comment", []byte(commentPayload))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if ev != (Event{Forge: "gh", Project: "alice/sandbox", Issue: 42, Kind: "issue_comment"}) {
		t.Fatalf("event = %+v", ev)
	}
}

func TestGitHubExtractSkips(t *testing.T) {
	for name, c := range map[string]struct{ event, body string }{
		"push":            {"push", `{"repository":{"full_name":"alice/sandbox"}}`},
		"assigned":        {"issues", `{"action":"assigned","issue":{"number":1},"repository":{"full_name":"a/b"}}`},
		"comment deleted": {"issue_comment", `{"action":"deleted","issue":{"number":1},"repository":{"full_name":"a/b"}}`},
		"comment on a PR": {"issue_comment", `{"action":"created","issue":{"number":1,"pull_request":{"url":"x"}},"repository":{"full_name":"a/b"}}`},
		"ping":            {"ping", `{"zen":"Keep it logically awesome."}`},
	} {
		if _, ok, err := extractGitHub(c.event, []byte(c.body)); ok || err != nil {
			t.Errorf("%s: ok=%v err=%v", name, ok, err)
		}
	}
	for _, action := range []string{"labeled", "unlabeled", "closed", "reopened", "edited"} {
		body := `{"action":"` + action + `","issue":{"number":3},"repository":{"full_name":"a/b"}}`
		if ev, ok, err := extractGitHub("issues", []byte(body)); !ok || err != nil || ev.Issue != 3 || ev.Kind != "issues" {
			t.Errorf("issues %s: %+v ok=%v err=%v", action, ev, ok, err)
		}
	}
	if _, _, err := extractGitHub("issues", []byte(`{"action":"labeled","issue":{"number":0},"repository":{"full_name":""}}`)); err == nil {
		t.Error("an issue event without a number or a repository was accepted")
	}
}
