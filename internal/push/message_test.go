package push

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

var at = time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)

func waiting(ref, reason string, since time.Time) inboxapi.Item {
	return inboxapi.Item{State: "waiting", Ref: ref, Title: "Add a greeting", Reason: reason, WaitingSince: &since,
		Question: "Which word should the greeting use? The full question stays on the forge."}
}

func done(ref string, since time.Time, cr int, sha string) inboxapi.Item {
	return inboxapi.Item{State: "done", Ref: ref, Title: "Add a greeting", DoneSince: &since, ChangeNumber: cr, ChangeSHA: sha}
}

func payload(t *testing.T, data map[string]string) map[string]any {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal([]byte(data["body"]), &p); err != nil {
		t.Fatalf("body %q is not JSON: %v", data["body"], err)
	}
	return p
}

func TestWaitingQuestionMessage(t *testing.T) {
	n, ok := FromItem(waiting("gh:o/r#12", "question", at))
	if !ok {
		t.Fatal("a waiting issue is not notified")
	}
	d := n.Data()
	if d["title"] != "o/r#12 question" || d["message"] != "Add a greeting" {
		t.Errorf("title %q message %q", d["title"], d["message"])
	}
	if _, ok := d["categoryId"]; ok {
		t.Errorf("a question carries category %q; it gets no buttons", d["categoryId"])
	}
	p := payload(t, d)
	if p["ref"] != "gh:o/r#12" || p["state"] != "waiting" || p["reason"] != "question" || p["since"] != "2026-11-02T09:30:00Z" {
		t.Errorf("payload = %v", p)
	}
	for k, v := range d {
		if strings.Contains(v, "Which word") {
			t.Errorf("data %s carries the question", k)
		}
	}
}

func TestPlanMessageHasTheApproveCategory(t *testing.T) {
	n, _ := FromItem(waiting("gl:g/p#3", "plan", at))
	if d := n.Data(); d["categoryId"] != CategoryPlan || d["title"] != "g/p#3 plan" {
		t.Fatalf("data = %v", d)
	}
}

func TestPlanQuestionsGetNoButtons(t *testing.T) {
	n, _ := FromItem(waiting("gh:o/r#12", "plan-questions", at))
	if _, ok := n.Data()["categoryId"]; ok {
		t.Fatal("plan-questions carries a category")
	}
}

func TestDoneMessageHasTheMergeCategory(t *testing.T) {
	n, ok := FromItem(done("gh:o/r#12", at, 73, "abc123"))
	if !ok {
		t.Fatal("a done issue is not notified")
	}
	d := n.Data()
	if d["categoryId"] != CategoryMerge || d["title"] != "o/r#12 done" {
		t.Errorf("data = %v", d)
	}
	p := payload(t, d)
	if p["change_number"] != float64(73) || p["change_sha"] != "abc123" || p["since"] != "2026-11-02T09:30:00Z" {
		t.Errorf("payload = %v", p)
	}
}

func TestOldDoneGetsNoMergeButton(t *testing.T) {
	n, _ := FromItem(done("gh:o/r#12", at, 0, ""))
	if _, ok := n.Data()["categoryId"]; ok {
		t.Fatal("a done post without its change request carries a category")
	}
}

func TestRunningAndQueuedAreNotNotified(t *testing.T) {
	for _, state := range []string{"running", "queued"} {
		if _, ok := FromItem(inboxapi.Item{State: state, Ref: "gh:o/r#1"}); ok {
			t.Errorf("%s is notified", state)
		}
	}
}
