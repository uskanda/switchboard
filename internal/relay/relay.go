// Package relay receives forge webhooks and passes on only what identifies the issue (ADR-004): the forge, the
// project, the issue number, the kind of event and the delivery ID. It verifies every webhook before it reads it and
// keeps no body, title, comment or login.
package relay

import "errors"

// Event is what the relay sends to the agents.
type Event struct {
	Forge    string `json:"forge"` // "gh" or "gl"
	Project  string `json:"project"`
	Issue    int    `json:"issue"`
	Kind     string `json:"kind"`
	Delivery string `json:"delivery"`
}

// ErrSignature reports a webhook whose signature does not verify.
var ErrSignature = errors.New("signature does not verify")
