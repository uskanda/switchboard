package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
)

func (f *fakeInbox) RegisterDevice(_ context.Context, token string) error {
	if f.err != nil {
		return f.err
	}
	f.devices = append(f.devices, token)
	return nil
}

func (f *fakeInbox) Approve(_ context.Context, ref string, since time.Time) error {
	if f.err != nil {
		return f.err
	}
	f.approved = append(f.approved, inboxapi.ApproveRequest{Ref: ref, Since: since})
	return nil
}

func TestDeviceRegistrationIsPassedOn(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/devices", `{"token":"fcm-1"}`, good)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	if len(f.devices) != 1 || f.devices[0] != "fcm-1" {
		t.Fatalf("devices = %v", f.devices)
	}
}

func TestApprovalIsPassedOn(t *testing.T) {
	f := &fakeInbox{}
	w := do(newServer(f), "POST", "/api/approve", `{"ref":"gh:o/r#20","since":"2026-11-02T09:30:00Z"}`, good)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
	want := time.Date(2026, 11, 2, 9, 30, 0, 0, time.UTC)
	if len(f.approved) != 1 || f.approved[0].Ref != "gh:o/r#20" || !f.approved[0].Since.Equal(want) {
		t.Fatalf("approved = %+v", f.approved)
	}
}

func TestStaleApprovalKeepsItsCode(t *testing.T) {
	f := &fakeInbox{err: &inboxapi.Error{Code: inboxapi.CodeStale, Message: "a newer plan"}}
	w := do(newServer(f), "POST", "/api/approve", `{"ref":"gh:o/r#20","since":"2026-11-02T09:30:00Z"}`, good)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body)
	}
}

func TestDevicesAndApprovalsFromElsewhereAreRefused(t *testing.T) {
	for _, path := range []string{"/api/devices", "/api/approve"} {
		for name, tc := range map[string]struct {
			h    map[string]string
			want int
		}{
			"other origin":  {map[string]string{"Content-Type": "application/json", "Origin": "https://evil.example"}, http.StatusForbidden},
			"no origin":     {map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
			"rebound host":  {map[string]string{"Content-Type": "application/json", "Origin": "http://" + listen, "Host": "evil.example:8783"}, http.StatusForbidden},
			"form encoding": {map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Origin": "http://" + listen}, http.StatusUnsupportedMediaType},
		} {
			t.Run(path+" "+name, func(t *testing.T) {
				f := &fakeInbox{}
				w := do(newServer(f), "POST", path, `{"token":"t","ref":"gh:o/r#20","since":"2026-11-02T09:30:00Z"}`, tc.h)
				if w.Code != tc.want {
					t.Fatalf("status = %d, want %d", w.Code, tc.want)
				}
				if len(f.devices)+len(f.approved) != 0 {
					t.Fatal("the request was passed on")
				}
			})
		}
	}
}
