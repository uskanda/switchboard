// Package event holds what the relay sends to its subscribers. The relay writes it, and the agent and the inbox
// service read it through relayclient, so that neither depends on the relay itself.
package event

// Event is one change to an issue that the relay passes on.
type Event struct {
	Forge    string `json:"forge"` // "gh" or "gl"
	Project  string `json:"project"`
	Issue    int    `json:"issue"`
	Kind     string `json:"kind"`
	Delivery string `json:"delivery"`
}
