package github

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/uskanda/switchboard/internal/forge"
)

var attachedPNG = []byte("\x89PNG\r\n\x1a\nattached")

// attachmentWorld is a github.com that redirects /user-attachments/assets/<id> to store, and the store, which
// answers with img. Both record the Authorization they got.
func attachmentWorld(t *testing.T, img []byte, to func(store string) string) (*Client, *[]string) {
	t.Helper()
	var auths []string
	store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, "store:"+r.Header.Get("Authorization"))
		_, _ = w.Write(img)
	}))
	t.Cleanup(store.Close)
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, "web:"+r.Header.Get("Authorization"))
		if r.URL.Path != "/user-attachments/assets/abc-1" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, to(store.URL), http.StatusFound)
	}))
	t.Cleanup(web.Close)
	u, _ := url.Parse(store.URL)
	c := New("tok", WithWebURL(web.URL))
	c.attachmentHost = func(host string) bool { return host == u.Host }
	return c, &auths
}

func TestReadAttachmentFollowsTheRedirectWithoutTheToken(t *testing.T) {
	c, auths := attachmentWorld(t, attachedPNG, func(store string) string { return store + "/x.png?sig=1" })
	got, err := c.ReadAttachment(ctx, "abc-1")
	if err != nil || !bytes.Equal(got, attachedPNG) {
		t.Fatalf("ReadAttachment = %q, %v", got, err)
	}
	if len(*auths) != 2 || (*auths)[0] != "web:Bearer tok" || (*auths)[1] != "store:" {
		t.Fatalf("authorization = %q, want the token for github.com only", *auths)
	}
}

func TestReadAttachmentRefusesARedirectElsewhere(t *testing.T) {
	c, auths := attachmentWorld(t, attachedPNG, func(string) string { return "http://127.0.0.1:1/elsewhere.png" })
	if _, err := c.ReadAttachment(ctx, "abc-1"); err == nil {
		t.Fatal("a redirect to another host was followed")
	}
	if len(*auths) != 1 {
		t.Fatalf("requests = %q", *auths)
	}
}

func TestReadAttachmentChecksTheImage(t *testing.T) {
	c, _ := attachmentWorld(t, []byte("<svg/>"), func(store string) string { return store + "/x.svg" })
	if _, err := c.ReadAttachment(ctx, "abc-1"); err == nil {
		t.Fatal("an SVG was returned")
	}
	if _, err := c.ReadAttachment(ctx, "other"); !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("missing: err = %v", err)
	}
}

func TestAttachmentHosts(t *testing.T) {
	for host, want := range map[string]bool{
		"github-production-user-asset-6210df.s3.amazonaws.com": true,
		"private-user-images.githubusercontent.com":            true,
		"evil.s3.amazonaws.com":                                false,
		"github-production-user-asset-x.s3.amazonaws.com.evil": false,
		"github.com": false,
	} {
		if got := isAttachmentHost(host); got != want {
			t.Errorf("isAttachmentHost(%q) = %v", host, got)
		}
	}
}
