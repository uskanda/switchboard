package push

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testKey makes a service account key file and returns it with the public key.
func testKey(t *testing.T, tokenURI string) ([]byte, *rsa.PublicKey) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	data, _ := json.Marshal(map[string]string{
		"type": "service_account", "client_email": "push@p.iam.gserviceaccount.com",
		"private_key": string(pemKey), "token_uri": tokenURI,
	})
	return data, &k.PublicKey
}

// tokenServer checks the JWT bearer grant against pub and hands out access tokens access-1, access-2, ...
func tokenServer(t *testing.T, pub *rsa.PublicKey, calls *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		parts := strings.Split(r.Form.Get("assertion"), ".")
		if len(parts) != 3 {
			t.Fatalf("assertion has %d parts", len(parts))
		}
		sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
		sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
			t.Errorf("the assertion's signature does not verify: %v", err)
		}
		var claims map[string]any
		body, _ := base64.RawURLEncoding.DecodeString(parts[1])
		_ = json.Unmarshal(body, &claims)
		if claims["iss"] != "push@p.iam.gserviceaccount.com" || claims["scope"] != Scope {
			t.Errorf("claims = %v", claims)
		}
		n := calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-" + string(rune('0'+n)), "expires_in": 3600})
	}))
}

func TestSendSignsInAndPosts(t *testing.T) {
	var tokens atomic.Int32
	key, pub := testKey(t, "")
	ts := tokenServer(t, pub, &tokens)
	defer ts.Close()
	var got map[string]any
	var auth string
	fcm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects/proj/messages:send" {
			t.Errorf("path = %s", r.URL.Path)
		}
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = io.WriteString(w, `{"name":"projects/proj/messages/1"}`)
	}))
	defer fcm.Close()
	creds, err := ParseCredentials(key)
	if err != nil {
		t.Fatal(err)
	}
	creds.TokenURI = ts.URL
	c := &Client{ProjectID: "proj", Credentials: creds, BaseURL: fcm.URL}
	if err := c.Send(context.Background(), "device-1", map[string]string{"title": "t"}); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer access-1" {
		t.Errorf("Authorization = %q", auth)
	}
	msg, _ := got["message"].(map[string]any)
	if msg["token"] != "device-1" {
		t.Errorf("token = %v", msg["token"])
	}
	if d, _ := msg["data"].(map[string]any); d["title"] != "t" {
		t.Errorf("data = %v", msg["data"])
	}
	if a, _ := msg["android"].(map[string]any); a["priority"] != "high" {
		t.Errorf("android = %v", msg["android"])
	}
	if _, ok := msg["notification"]; ok {
		t.Error("the message has a notification part; it must carry data only")
	}
}

func TestAccessTokenIsReusedUntilFiveMinutesBeforeExpiry(t *testing.T) {
	var tokens atomic.Int32
	key, pub := testKey(t, "")
	ts := tokenServer(t, pub, &tokens)
	defer ts.Close()
	var auths []string
	fcm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
	}))
	defer fcm.Close()
	creds, _ := ParseCredentials(key)
	creds.TokenURI = ts.URL
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	c := &Client{ProjectID: "proj", Credentials: creds, BaseURL: fcm.URL, Now: func() time.Time { return now }}
	for _, step := range []time.Duration{0, 30 * time.Minute, 24 * time.Minute, time.Minute} {
		now = now.Add(step)
		if err := c.Send(context.Background(), "d", nil); err != nil {
			t.Fatal(err)
		}
	}
	// 0, 30m and 54m use the first token; 55m is five minutes before its expiry at 60m.
	want := []string{"Bearer access-1", "Bearer access-1", "Bearer access-1", "Bearer access-2"}
	if strings.Join(auths, ",") != strings.Join(want, ",") {
		t.Fatalf("authorizations = %v, want %v", auths, want)
	}
}

func TestUnregisteredToken(t *testing.T) {
	var tokens atomic.Int32
	key, pub := testKey(t, "")
	ts := tokenServer(t, pub, &tokens)
	defer ts.Close()
	fcm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":404,"status":"NOT_FOUND","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`)
	}))
	defer fcm.Close()
	creds, _ := ParseCredentials(key)
	creds.TokenURI = ts.URL
	c := &Client{ProjectID: "proj", Credentials: creds, BaseURL: fcm.URL}
	err := c.Send(context.Background(), "gone-device-token", nil)
	if !errors.Is(err, ErrUnregistered) {
		t.Fatalf("err = %v, want ErrUnregistered", err)
	}
}

func TestErrorsLeaveSecretsOut(t *testing.T) {
	var tokens atomic.Int32
	key, pub := testKey(t, "")
	ts := tokenServer(t, pub, &tokens)
	defer ts.Close()
	fcm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error":{"code":500,"status":"INTERNAL","message":"boom"}}`)
	}))
	defer fcm.Close()
	creds, _ := ParseCredentials(key)
	creds.TokenURI = ts.URL
	c := &Client{ProjectID: "proj", Credentials: creds, BaseURL: fcm.URL}
	err := c.Send(context.Background(), "secret-device-token", nil)
	if err == nil {
		t.Fatal("no error")
	}
	for _, leak := range []string{"secret-device-token", "access-1", "PRIVATE KEY"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error %q holds %q", err, leak)
		}
	}
}

func TestSendGivesUpAfterTheTimeout(t *testing.T) {
	var tokens atomic.Int32
	key, pub := testKey(t, "")
	ts := tokenServer(t, pub, &tokens)
	defer ts.Close()
	release := make(chan struct{})
	fcm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer fcm.Close()
	defer close(release)
	creds, _ := ParseCredentials(key)
	creds.TokenURI = ts.URL
	c := &Client{ProjectID: "proj", Credentials: creds, BaseURL: fcm.URL, Timeout: 50 * time.Millisecond}
	start := time.Now()
	if err := c.Send(context.Background(), "d", nil); err == nil {
		t.Fatal("no error")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("Send did not give up at its timeout")
	}
}

func TestParseCredentialsRejects(t *testing.T) {
	for name, body := range map[string]string{
		"not json":    "nope",
		"no email":    `{"private_key":"x"}`,
		"no key":      `{"client_email":"a@b"}`,
		"bad pem key": `{"client_email":"a@b","private_key":"-----BEGIN PRIVATE KEY-----\nzz\n-----END PRIVATE KEY-----\n"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseCredentials([]byte(body))
			if err == nil {
				t.Fatal("accepted")
			}
			if strings.Contains(err.Error(), "BEGIN PRIVATE KEY") {
				t.Errorf("error %q holds the key", err)
			}
		})
	}
}

func TestDefaultTokenURI(t *testing.T) {
	key, _ := testKey(t, "")
	creds, err := ParseCredentials(key)
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := url.Parse(creds.TokenURI); u.Host != "oauth2.googleapis.com" {
		t.Fatalf("token_uri = %q", creds.TokenURI)
	}
}
