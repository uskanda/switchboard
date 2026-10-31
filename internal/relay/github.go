package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// verifyGitHub checks X-Hub-Signature-256, "sha256=<hex>", against the HMAC-SHA256 of body under secret.
func verifyGitHub(secret []byte, header string, body []byte) error {
	hexSig, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return ErrSignature
	}
	got, err := hex.DecodeString(hexSig)
	if err != nil {
		return ErrSignature
	}
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	if !hmac.Equal(got, m.Sum(nil)) {
		return ErrSignature
	}
	return nil
}

// githubPayload holds the only fields the relay reads. Everything else, bodies and logins included, is never decoded.
type githubPayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number      int             `json:"number"`
		PullRequest json.RawMessage `json:"pull_request"`
	} `json:"issue"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

var githubActions = map[string]map[string]bool{
	"issues":        {"labeled": true, "unlabeled": true, "closed": true, "reopened": true, "edited": true},
	"issue_comment": {"created": true, "edited": true},
}

// extractGitHub turns a delivery of event (X-GitHub-Event) into an Event. ok is false for an event the agent does
// not follow; it is not an error.
func extractGitHub(event string, body []byte) (Event, bool, error) {
	actions, follow := githubActions[event]
	if !follow {
		return Event{}, false, nil
	}
	var p githubPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return Event{}, false, fmt.Errorf("decode %s: %w", event, err)
	}
	if !actions[p.Action] || len(p.Issue.PullRequest) > 0 {
		return Event{}, false, nil
	}
	if p.Issue.Number <= 0 || p.Repository.FullName == "" {
		return Event{}, false, fmt.Errorf("%s without an issue number or a repository", event)
	}
	return Event{Forge: "gh", Project: p.Repository.FullName, Issue: p.Issue.Number, Kind: event}, true, nil
}
