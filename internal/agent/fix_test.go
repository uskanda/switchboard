package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/uskanda/switchboard/internal/workspace"
)

func TestSettingsNeverAllowRebaseOrPush(t *testing.T) {
	for name, s := range map[string][]string{
		"implement": settings("make verify", "").Deny,
		"fix ci":    settings("make verify", "ci").Deny,
		"fix push":  settings("make verify", "push").Deny,
		"plan":      planSettings().Deny,
	} {
		for _, d := range []string{"Bash(git rebase)", "Bash(git rebase *)", "Bash(git push)", "Bash(git push *)"} {
			if !slices.Contains(s, d) {
				t.Errorf("%s: deny lacks %s", name, d)
			}
		}
	}
}

func TestOnlyAPushFixMayMerge(t *testing.T) {
	if !slices.Contains(settings("", "push").Allow, "Bash(git merge *)") {
		t.Error("a push fix run must be allowed to merge")
	}
	for _, cause := range []string{"", "ci"} {
		if s := settings("", cause); slices.Contains(s.Allow, "Bash(git merge *)") || slices.Contains(s.Allow, "Bash(git merge)") {
			t.Errorf("settings(%q) allows git merge", cause)
		}
	}
}

type scriptedPush struct{ errs []error }

func (p *scriptedPush) Push(context.Context) error {
	err := p.errs[0]
	p.errs = p.errs[1:]
	return err
}

func TestPushTriesATransientFailureAgain(t *testing.T) {
	a := &Agent{Log: io.Discard, PushDelays: []time.Duration{time.Millisecond, time.Millisecond}}
	blip := &workspace.PushError{Kind: workspace.PushTransient, Output: "Could not resolve host: github.com"}
	p := &scriptedPush{errs: []error{blip, nil}}
	if err := a.push(context.Background(), p); err != nil || len(p.errs) != 0 {
		t.Fatalf("push = %v with %d tries left, want success on the second try", err, len(p.errs))
	}
	p = &scriptedPush{errs: []error{blip, blip, blip, nil}}
	var pe *workspace.PushError
	if err := a.push(context.Background(), p); !errors.As(err, &pe) || len(p.errs) != 1 {
		t.Fatalf("push = %v with %d tries left, want the third failure returned", err, len(p.errs))
	}
	rejected := &workspace.PushError{Kind: workspace.PushRejected, Output: "pre-receive hook declined"}
	p = &scriptedPush{errs: []error{rejected, nil}}
	if err := a.push(context.Background(), p); err != rejected {
		t.Fatalf("push = %v, want a refused push returned at once", err)
	}
}

func TestTransientPushFailureWaitsWithReasonError(t *testing.T) {
	o := (&outcome{msg: english}).fail(fmt.Sprintf(english.pushFailed, "x"))
	if o.State != "waiting" || o.Reason != "error" {
		t.Fatalf("outcome = %s", o.result())
	}
}

func TestPushRefusedNamesTheKind(t *testing.T) {
	auth := (&outcome{msg: japanese}).pushRefused(workspace.PushAuth, "remote: Permission to o/r.git denied", 0, 2)
	if auth.State != "waiting" || auth.Reason != "push" || !strings.Contains(auth.Body, "トークン") || !strings.Contains(auth.Body, "Permission to o/r.git denied") {
		t.Fatalf("auth = %s\n%s", auth.result(), auth.Body)
	}
	rule := (&outcome{msg: english}).pushRefused(workspace.PushRejected, "GH006: Protected branch update failed", 0, 2)
	if rule.Reason != "push" || !strings.Contains(rule.Body, "branch protection") || !strings.Contains(rule.Body, "reply with what to change") {
		t.Fatalf("rule = %s\n%s", rule.result(), rule.Body)
	}
	moved := (&outcome{msg: english}).pushRefused(workspace.PushNonFastForward, "[rejected] (fetch first)", 2, 2)
	if moved.Reason != "push" || !strings.Contains(moved.Body, "2 fix run(s)") {
		t.Fatalf("moved = %s\n%s", moved.result(), moved.Body)
	}
}
