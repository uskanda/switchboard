// Package push sends the inbox service's push notifications to the Android app through FCM HTTP v1: which issues
// to tell about, what a notification carries, and where the app registered. Sending is a side channel: a failure is
// recorded and never stops the listing it reports on.
package push

import (
	"bytes"
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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Scope is the OAuth scope that lets a service account send FCM messages.
const Scope = "https://www.googleapis.com/auth/firebase.messaging"

// DefaultTokenURI is where a service account trades its signed assertion for an access token.
const DefaultTokenURI = "https://oauth2.googleapis.com/token"

// ErrUnregistered reports that FCM no longer knows the device token: the app was removed or its token changed.
var ErrUnregistered = errors.New("fcm: the device token is not registered")

// Credentials are the parts of a Firebase service account key file that sign in.
type Credentials struct {
	ClientEmail string
	TokenURI    string
	key         *rsa.PrivateKey
}

// ParseCredentials reads a service account key file (JSON). Its errors never quote the key.
func ParseCredentials(data []byte) (*Credentials, error) {
	var f struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, errors.New("fcm: the service account key is not JSON")
	}
	if f.ClientEmail == "" || f.PrivateKey == "" {
		return nil, errors.New("fcm: the service account key needs client_email and private_key")
	}
	block, _ := pem.Decode([]byte(f.PrivateKey))
	if block == nil {
		return nil, errors.New("fcm: the service account's private_key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		parsed, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if err != nil || !ok {
		return nil, errors.New("fcm: the service account's private_key is not an RSA key")
	}
	if f.TokenURI == "" {
		f.TokenURI = DefaultTokenURI
	}
	return &Credentials{ClientEmail: f.ClientEmail, TokenURI: f.TokenURI, key: key}, nil
}

// Client sends data messages to devices through FCM HTTP v1.
type Client struct {
	ProjectID   string
	Credentials *Credentials
	BaseURL     string           // https://fcm.googleapis.com when empty
	HTTP        *http.Client     // http.DefaultClient when nil
	Timeout     time.Duration    // 10s when zero; bounds one Send, the sign-in included
	Now         func() time.Time // time.Now when nil

	mu      sync.Mutex
	access  string
	expires time.Time
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

// Send posts a data-only, high-priority message to one device. The data is what the app shows; FCM itself displays
// nothing, so the app can attach its buttons. Errors never hold the device token, the access token or the key.
func (c *Client) Send(ctx context.Context, device string, data map[string]string) error {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	access, err := c.token(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"message": map[string]any{
		"token":   device,
		"data":    data,
		"android": map[string]any{"priority": "high"},
	}})
	if err != nil {
		return err
	}
	base := c.BaseURL
	if base == "" {
		base = "https://fcm.googleapis.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/projects/"+url.PathEscape(c.ProjectID)+"/messages:send", bytes.NewReader(body))
	if err != nil {
		return errors.New("fcm: the request could not be built")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access)
	resp, err := c.http().Do(req)
	if err != nil {
		return transportError(err, timeout)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	var out struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	_ = json.Unmarshal(raw, &out)
	for _, d := range out.Error.Details {
		if d.ErrorCode == "UNREGISTERED" {
			return ErrUnregistered
		}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		c.mu.Lock()
		c.access = ""
		c.mu.Unlock()
	}
	return fmt.Errorf("fcm: %d %s %s", resp.StatusCode, out.Error.Status, strings.ReplaceAll(out.Error.Message, device, "<device>"))
}

func transportError(err error, timeout time.Duration) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("fcm: no answer within %v", timeout)
	case errors.Is(err, context.Canceled):
		return errors.New("fcm: canceled")
	}
	return errors.New("fcm: the request failed before an answer")
}

// token returns an access token, signing in again from five minutes before the last one expires.
func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.access != "" && now.Before(c.expires.Add(-5*time.Minute)) {
		return c.access, nil
	}
	assertion, err := c.assertion(now)
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Credentials.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("fcm: the sign-in request could not be built")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http().Do(req)
	if err != nil {
		return "", transportError(err, c.Timeout)
	}
	defer resp.Body.Close()
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Error       string `json:"error"`
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK || out.AccessToken == "" {
		return "", fmt.Errorf("fcm: sign-in refused: %d %s", resp.StatusCode, out.Error)
	}
	c.access = out.AccessToken
	c.expires = now.Add(time.Duration(out.ExpiresIn) * time.Second)
	return c.access, nil
}

// assertion is the signed JWT (RS256) that asks the token endpoint for an FCM access token.
func (c *Client) assertion(now time.Time) (string, error) {
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	unsigned := enc(map[string]string{"alg": "RS256", "typ": "JWT"}) + "." + enc(map[string]any{
		"iss": c.Credentials.ClientEmail, "scope": Scope, "aud": c.Credentials.TokenURI,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.Credentials.key, crypto.SHA256, sum[:])
	if err != nil {
		return "", errors.New("fcm: the sign-in assertion could not be signed")
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
