package webauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitLab is the Provider of a GitLab instance, at the fixed paths GitLab serves OAuth on: the discovery document is
// not read. ClientSecret is the confidential application's secret; PKCE is used as well.
type GitLab struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	HTTP         *http.Client // http.DefaultClient when nil
}

// Scopes are what a login asks for: who the person is, and api, to write to GitLab as them (issue #146).
const Scopes = "openid profile api"

func (g *GitLab) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return http.DefaultClient
}

func (g *GitLab) base() string { return strings.TrimRight(g.BaseURL, "/") }

func (g *GitLab) AuthURL(redirect, state, challenge string) string {
	q := url.Values{
		"client_id":             {g.ClientID},
		"redirect_uri":          {redirect},
		"response_type":         {"code"},
		"scope":                 {Scopes},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return g.base() + "/oauth/authorize?" + q.Encode()
}

func (g *GitLab) Exchange(ctx context.Context, code, verifier, redirect string) (Token, error) {
	return g.token(ctx, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {verifier},
		"redirect_uri": {redirect}})
}

func (g *GitLab) Refresh(ctx context.Context, refresh string) (Token, error) {
	return g.token(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}})
}

func (g *GitLab) token(ctx context.Context, form url.Values) (Token, error) {
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := g.post(ctx, "/oauth/token", form, &out); err != nil {
		return Token{}, err
	}
	if out.AccessToken == "" {
		return Token{}, fmt.Errorf("GitLab answered %s without an access token", form.Get("grant_type"))
	}
	return Token{Access: out.AccessToken, Refresh: out.RefreshToken, ExpiresIn: time.Duration(out.ExpiresIn) * time.Second}, nil
}

func (g *GitLab) UserInfo(ctx context.Context, access string) (User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.base()+"/oauth/userinfo", nil)
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	var out struct {
		Nickname string   `json:"nickname"`
		Groups   []string `json:"groups"`
	}
	if err := g.do(req, &out); err != nil {
		return User{}, err
	}
	if out.Nickname == "" {
		return User{}, fmt.Errorf("GitLab's userinfo has no nickname")
	}
	return User{Name: out.Nickname, Groups: out.Groups}, nil
}

// Revoke revokes token on GitLab. A failure is not reported: the session is gone either way, and the token expires
// within two hours.
func (g *GitLab) Revoke(ctx context.Context, token string) {
	if token == "" {
		return
	}
	g.post(ctx, "/oauth/revoke", url.Values{"client_id": {g.ClientID}, "client_secret": {g.ClientSecret}, "token": {token}}, nil)
}

func (g *GitLab) post(ctx context.Context, path string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.base()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return g.do(req, out)
}

func (g *GitLab) do(req *http.Request, out any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := g.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		json.Unmarshal(body, &e)
		if e.Error == "" {
			e.Error = resp.Status
		}
		return fmt.Errorf("GitLab %s: %s", req.URL.Path, e.Error)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}
