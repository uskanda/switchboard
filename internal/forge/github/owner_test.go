package github

import (
	"context"
	"net/http"
	"testing"
)

func TestTokenOwner(t *testing.T) {
	s, c := newServer(t)
	s.on("GET /user", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"login":"olivia","id":7}`)) })
	got, err := c.TokenOwner(context.Background())
	if err != nil || got != "olivia" {
		t.Fatalf("owner = %q, %v", got, err)
	}
}
