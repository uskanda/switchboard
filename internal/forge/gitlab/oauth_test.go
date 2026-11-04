package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOAuthClientSendsABearerToken(t *testing.T) {
	var auth, private string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, private = r.Header.Get("Authorization"), r.Header.Get("PRIVATE-TOKEN")
		w.Write([]byte(`{"version":"19.5.1"}`))
	}))
	defer ts.Close()
	if _, err := NewOAuth("oauth-tok", ts.URL).Version(context.Background()); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer oauth-tok" || private != "" {
		t.Fatalf("Authorization = %q, PRIVATE-TOKEN = %q; want the bearer only", auth, private)
	}
}
