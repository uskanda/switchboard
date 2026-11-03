package push

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

// The notification categories the app registers, each with its buttons: approve on a plan waiting for approval,
// merge on a done issue whose change request and commit are known. Other notifications have no buttons.
const (
	CategoryPlan  = "plan"
	CategoryMerge = "done-merge"
)

// Channel is the Android notification channel the app creates for these notifications.
const Channel = "switchboard"

// Notification is what one push says about an issue: its reference, state, reason, title and the time of the post
// it reports on. It never carries a question, a plan, progress or a comment (Q9 of add-android-app).
type Notification struct {
	Ref          string
	State        string // waiting or done
	Reason       string // the wait's reason; "" when done
	Title        string
	Since        time.Time
	ChangeNumber int
	ChangeSHA    string
}

// FromItem is the notification of a listed issue; ok is false for an issue that is told about by no push (running,
// queued, or without the time of its post).
func FromItem(it inboxapi.Item) (n Notification, ok bool) {
	n = Notification{Ref: it.Ref, State: it.State, Title: it.Title}
	switch it.State {
	case "waiting":
		if it.WaitingSince == nil {
			return n, false
		}
		n.Reason, n.Since = it.Reason, it.WaitingSince.UTC()
	case "done":
		if it.DoneSince == nil {
			return n, false
		}
		n.Since, n.ChangeNumber, n.ChangeSHA = it.DoneSince.UTC(), it.ChangeNumber, it.ChangeSHA
	default:
		return n, false
	}
	return n, true
}

// Category is the notification's category, which gives it its buttons, or "".
func (n Notification) Category() string {
	switch {
	case n.State == "waiting" && n.Reason == "plan":
		return CategoryPlan
	case n.State == "done" && n.ChangeNumber > 0 && n.ChangeSHA != "":
		return CategoryMerge
	}
	return ""
}

// Data is the FCM data of the notification, in the keys expo-notifications shows a data message with: title,
// message, body (a JSON object the app reads back when a notification or one of its buttons is pressed), channelId
// and categoryId.
func (n Notification) Data() map[string]string {
	short := strings.TrimPrefix(strings.TrimPrefix(n.Ref, "gh:"), "gl:")
	word := n.Reason
	if n.State == "done" {
		word = "done"
	}
	body := map[string]any{"ref": n.Ref, "state": n.State, "since": n.Since.Format(time.RFC3339Nano)}
	if n.Reason != "" {
		body["reason"] = n.Reason
	}
	if n.ChangeNumber > 0 {
		body["change_number"] = n.ChangeNumber
		body["change_sha"] = n.ChangeSHA
	}
	raw, _ := json.Marshal(body)
	d := map[string]string{
		"title":     short + " " + word,
		"message":   n.Title,
		"body":      string(raw),
		"channelId": Channel,
	}
	if c := n.Category(); c != "" {
		d["categoryId"] = c
	}
	return d
}
