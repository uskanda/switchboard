package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// maxSkew is how far webhook-timestamp may be from this host's clock (design.md 3).
const maxSkew = 5 * time.Minute

// ErrStale reports a signed webhook whose timestamp is too far from now, as a replay would be.
var ErrStale = errors.New("timestamp is out of range")

// verifyGitLab checks a GitLab signing token signature (V16): HMAC-SHA256, under the base64-decoded part of key after
// whsec_, of "<webhook-id>.<webhook-timestamp>.<body>", against any "v1,<base64>" in the space-separated header.
func verifyGitLab(key, id, timestamp, header string, body []byte, now time.Time) error {
	encoded, ok := strings.CutPrefix(key, "whsec_")
	if !ok {
		return errors.New("the signing token does not start with whsec_")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("the signing token is not base64: %w", err)
	}
	sec, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || id == "" {
		return ErrSignature
	}
	m := hmac.New(sha256.New, raw)
	m.Write([]byte(id + "." + timestamp + "."))
	m.Write(body)
	want := m.Sum(nil)
	matched := false
	for _, s := range strings.Fields(header) {
		b64, ok := strings.CutPrefix(s, "v1,")
		if !ok {
			continue
		}
		if got, err := base64.StdEncoding.DecodeString(b64); err == nil && hmac.Equal(got, want) {
			matched = true
		}
	}
	if !matched {
		return ErrSignature
	}
	if d := now.Sub(time.Unix(sec, 0)); d > maxSkew || d < -maxSkew {
		return ErrStale
	}
	return nil
}

// verifyGitLabToken compares X-Gitlab-Token with the secret in constant time. Only a project with gitlab_token uses it.
func verifyGitLabToken(secret, header string) error {
	if header == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(header)) != 1 {
		return ErrSignature
	}
	return nil
}

// gitlabPayload holds the only fields the relay reads.
type gitlabPayload struct {
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	ObjectAttributes struct {
		IID          int    `json:"iid"`
		NoteableType string `json:"noteable_type"`
	} `json:"object_attributes"`
	Issue struct {
		IID int `json:"iid"`
	} `json:"issue"`
}

// extractGitLab turns a delivery of event (X-Gitlab-Event) into an Event. Issue hooks and notes on issues, confidential
// or not, are followed; ok is false for the rest.
func extractGitLab(event string, body []byte) (Event, bool, error) {
	kind := ""
	switch event {
	case "Issue Hook", "Confidential Issue Hook":
		kind = "issue"
	case "Note Hook", "Confidential Note Hook":
		kind = "note"
	default:
		return Event{}, false, nil
	}
	var p gitlabPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return Event{}, false, fmt.Errorf("decode %s: %w", event, err)
	}
	iid := p.ObjectAttributes.IID
	if kind == "note" {
		if p.ObjectAttributes.NoteableType != "Issue" {
			return Event{}, false, nil
		}
		iid = p.Issue.IID
	}
	if iid <= 0 || p.Project.PathWithNamespace == "" {
		return Event{}, false, fmt.Errorf("%s without an issue number or a project", event)
	}
	return Event{Forge: "gl", Project: p.Project.PathWithNamespace, Issue: iid, Kind: kind}, true, nil
}
