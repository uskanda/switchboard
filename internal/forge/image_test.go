package forge

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 32)...)
	gifBytes  = append([]byte("GIF89a"), make([]byte, 32)...)
	webpBytes = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
	svgBytes  = []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
)

func TestImageTypeKnowsTheFourKinds(t *testing.T) {
	for want, data := range map[string][]byte{"image/png": pngBytes, "image/jpeg": jpegBytes, "image/gif": gifBytes, "image/webp": webpBytes} {
		if got, err := ImageType(data); err != nil || got != want {
			t.Errorf("ImageType = %q, %v; want %s", got, err, want)
		}
	}
}

func TestImageTypeRefusesSVGAndOthers(t *testing.T) {
	for name, data := range map[string][]byte{"svg": svgBytes, "html": []byte("<html></html>"), "empty": nil,
		"too large": append(append([]byte{}, pngBytes...), make([]byte, MaxImageBytes)...)} {
		if _, err := ImageType(data); !errors.Is(err, ErrBadImage) {
			t.Errorf("%s: err = %v, want ErrBadImage", name, err)
		}
	}
}

// imageServer serves body at /a.png and records whether a request carried the credential.
func imageServer(t *testing.T, body []byte) (*httptest.Server, *[]string) {
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path+" "+r.Header.Get("PRIVATE-TOKEN"))
		switch r.URL.Path {
		case "/a.png":
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, &seen
}

var auth = http.Header{"Private-Token": {"tok"}}

func TestGetImageSendsTheCredentialOnlyToItsHost(t *testing.T) {
	ts, seen := imageServer(t, pngBytes)
	u, _ := url.Parse(ts.URL)
	data, err := GetImage(context.Background(), http.DefaultClient, ts.URL+"/a.png", u.Host, auth)
	if err != nil || !bytes.Equal(data, pngBytes) {
		t.Fatalf("GetImage = %d bytes, %v", len(data), err)
	}
	if _, err := GetImage(context.Background(), http.DefaultClient, ts.URL+"/a.png", "api.example", auth); err != nil {
		t.Fatal(err)
	}
	if (*seen)[0] != "/a.png tok" || (*seen)[1] != "/a.png " {
		t.Fatalf("requests = %q", *seen)
	}
}

func TestGetImageDropsTheCredentialOnARedirectElsewhere(t *testing.T) {
	other, seen := imageServer(t, pngBytes)
	home := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/a.png", http.StatusFound)
	}))
	t.Cleanup(home.Close)
	u, _ := url.Parse(home.URL)
	if _, err := GetImage(context.Background(), http.DefaultClient, home.URL+"/x", u.Host, auth); err != nil {
		t.Fatal(err)
	}
	if (*seen)[0] != "/a.png " {
		t.Fatalf("the other host got %q", *seen)
	}
}

func TestGetImageRefusals(t *testing.T) {
	ts, _ := imageServer(t, svgBytes)
	if _, err := GetImage(context.Background(), http.DefaultClient, ts.URL+"/a.png", "", nil); !errors.Is(err, ErrBadImage) {
		t.Fatalf("svg: err = %v", err)
	}
	if _, err := GetImage(context.Background(), http.DefaultClient, ts.URL+"/none", "", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("404: err = %v", err)
	}
	big, _ := imageServer(t, append(append([]byte{}, pngBytes...), make([]byte, MaxImageBytes)...))
	if _, err := GetImage(context.Background(), http.DefaultClient, big.URL+"/a.png", "", nil); !errors.Is(err, ErrBadImage) {
		t.Fatalf("too large: err = %v", err)
	}
}
