package inboxserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/uskanda/switchboard/inboxapi"
	"github.com/uskanda/switchboard/internal/forge"
	"github.com/uskanda/switchboard/internal/inbox"
)

// script is an Inbox whose listing fails while fail is set, or when its context ends.
type script struct {
	calls int
	fail  bool
}

func (s *script) Collect(ctx context.Context) ([]inbox.Item, []inbox.ForgeError) {
	s.calls++
	if err := ctx.Err(); err != nil {
		return nil, []inbox.ForgeError{{Forge: "github", Err: err}}
	}
	if s.fail {
		return nil, []inbox.ForgeError{{Forge: "github", Err: errors.New("connection refused")}}
	}
	return []inbox.Item{{N: 1, Ref: "gh:o/r#12"}}, nil
}
func (s *script) ParseRef(string) (forge.IssueRef, error)                       { return forge.IssueRef{}, nil }
func (s *script) Detail(context.Context, forge.IssueRef) (*inbox.Detail, error) { return nil, nil }
func (s *script) Reply(context.Context, forge.IssueRef, string, bool) error     { return nil }

func list(t *testing.T, s *Server, ctx context.Context) inboxapi.Inbox {
	t.Helper()
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/v1/inbox", nil).WithContext(ctx))
	var out inboxapi.Inbox
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	return out
}

// A browser that gives up on its request does not cut the forges' answers short, nor leave an error in the cache.
func TestACanceledRequestStillFetches(t *testing.T) {
	sc := &script{}
	now := time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)
	s := &Server{Inbox: sc, Now: func() time.Time { return now }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := list(t, s, ctx); len(got.Errors) != 0 || len(got.Items) != 1 {
		t.Fatalf("canceled request: %+v", got)
	}
	if got := list(t, s, context.Background()); len(got.Items) != 1 || sc.calls != 1 {
		t.Fatalf("next request: %+v after %d calls", got, sc.calls)
	}
}

// A listing with an unreachable forge is shown, not kept: the next request asks the forges again.
func TestAListingWithErrorsIsNotCached(t *testing.T) {
	sc := &script{fail: true}
	now := time.Date(2026, 10, 31, 13, 0, 0, 0, time.UTC)
	s := &Server{Inbox: sc, Now: func() time.Time { return now }}
	if got := list(t, s, context.Background()); len(got.Errors) != 1 {
		t.Fatalf("first: %+v", got)
	}
	sc.fail = false
	if got := list(t, s, context.Background()); len(got.Errors) != 0 || len(got.Items) != 1 || sc.calls != 2 {
		t.Fatalf("second: %+v after %d calls", got, sc.calls)
	}
}
