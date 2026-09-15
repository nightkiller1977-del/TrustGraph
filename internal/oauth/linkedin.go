// Package oauth implements the Plane B LinkedIn OAuth 2.0 authorization-code
// flow. It is deliberately small and dependency-free: the HTTP calls go through
// an injectable *http.Client so tests can stand up an httptest server instead of
// reaching LinkedIn.
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotConfigured is returned when the flow has no client credentials.
var ErrNotConfigured = errors.New("linkedin oauth is not configured")

// defaultScopes are the minimum needed for identity plus education/employment.
// LinkedIn's modern API exposes most profile fields through OpenID Connect
// scopes, so these are the ones requested from the consent screen.
var defaultScopes = []string{"openid", "profile", "email", "w_member_social"}

// Config holds LinkedIn OAuth client credentials and endpoints.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
	HTTPClient   *http.Client
}

// NewConfig returns a config pointing at LinkedIn's production endpoints. The
// URLs are overridable (per-field) so tests can redirect them.
func NewConfig(clientID, clientSecret, redirectURI string) *Config {
	return &Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		AuthorizeURL: "https://www.linkedin.com/oauth/v2/authorization",
		TokenURL:     "https://www.linkedin.com/oauth/v2/accessToken",
		UserInfoURL:  "https://api.linkedin.com/v2/userinfo",
		Scopes:       defaultScopes,
		HTTPClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// Configured reports whether the client has enough to start a flow.
func (c *Config) Configured() bool {
	return c != nil && c.ClientID != "" && c.ClientSecret != "" && c.RedirectURI != ""
}

// NewState generates a cryptographically random, URL-safe OAuth state value.
func NewState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// AuthURL builds the authorization URL the user is redirected to.
func (c *Config) AuthURL(state string) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	if state == "" {
		return "", errors.New("oauth state must not be empty")
	}

	u, err := url.Parse(c.AuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("parse authorize url: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.ClientID)
	q.Set("redirect_uri", c.RedirectURI)
	q.Set("state", state)
	q.Set("scope", strings.Join(c.Scopes, " "))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// TokenResponse is the subset of the token endpoint response we store.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

// ExpiresAt converts ExpiresIn into an absolute expiry.
func (t *TokenResponse) ExpiresAt() *time.Time {
	if t.ExpiresIn <= 0 {
		return nil
	}
	expiry := time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	return &expiry
}

// ExchangeCode trades an authorization code for tokens.
func (c *Config) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	if code == "" {
		return nil, errors.New("authorization code must not be empty")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.RedirectURI)
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, truncate(string(body), 512))
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if token.AccessToken == "" {
		return nil, errors.New("token endpoint returned no access token")
	}
	return &token, nil
}

// UserInfo is the OpenID Connect profile returned by LinkedIn's userinfo
// endpoint. Education/employment are not part of OIDC, so they arrive empty
// here and are populated by a separate (partner-tier) call in production;
// the validators handle an empty history gracefully.
type UserInfo struct {
	Sub         string `json:"sub"`
	Email       string `json:"email"`
	GivenName   string `json:"given_name"`
	FamilyName  string `json:"family_name"`
	Picture     string `json:"picture"`
	Locale      string `json:"locale"`
	ProfileURL  string `json:"profile"`
	Headline    string `json:"headline"`
	Connections int    `json:"connections_count"`
}

// FetchUserInfo calls the OIDC userinfo endpoint with the access token.
func (c *Config) FetchUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	if !c.Configured() {
		return nil, ErrNotConfigured
	}
	if accessToken == "" {
		return nil, errors.New("access token must not be empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.UserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read userinfo response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, truncate(string(body), 512))
	}

	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode userinfo response: %w", err)
	}
	return &info, nil
}

func (c *Config) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
