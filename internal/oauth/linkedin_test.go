package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Configured(t *testing.T) {
	assert.False(t, NewConfig("", "", "").Configured())
	assert.False(t, NewConfig("id", "", "https://cb").Configured())
	assert.True(t, NewConfig("id", "secret", "https://cb").Configured())
}

func TestNewState_IsUniqueAndHex(t *testing.T) {
	a, err := NewState()
	require.NoError(t, err)
	b, err := NewState()
	require.NoError(t, err)

	assert.NotEqual(t, a, b)
	assert.Len(t, a, 64) // 32 random bytes hex-encoded
}

func TestConfig_AuthURL_IncludesStateAndScopes(t *testing.T) {
	cfg := NewConfig("client-1", "secret", "https://app.example/callback")
	authURL, err := cfg.AuthURL("state-abc")
	require.NoError(t, err)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	q := parsed.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "client-1", q.Get("client_id"))
	assert.Equal(t, "state-abc", q.Get("state"))
	assert.Contains(t, q.Get("scope"), "openid")
}

func TestConfig_AuthURL_RejectsEmptyState(t *testing.T) {
	cfg := NewConfig("client-1", "secret", "https://app.example/callback")
	_, err := cfg.AuthURL("")
	assert.Error(t, err)
}

func TestConfig_AuthURL_NotConfigured(t *testing.T) {
	cfg := NewConfig("", "", "")
	_, err := cfg.AuthURL("state")
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestConfig_ExchangeCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := url.ParseQuery(readBody(r))
		assert.Equal(t, "authorization_code", body.Get("grant_type"))
		assert.Equal(t, "the-code", body.Get("code"))
		assert.Equal(t, "secret", body.Get("client_secret"))

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "at-123",
			"refresh_token": "rt-456",
			"expires_in":    3600,
			"scope":         "openid profile",
		})
	}))
	defer server.Close()

	cfg := NewConfig("client-1", "secret", "https://app.example/callback")
	cfg.TokenURL = server.URL

	token, err := cfg.ExchangeCode(context.Background(), "the-code")
	require.NoError(t, err)
	assert.Equal(t, "at-123", token.AccessToken)
	assert.Equal(t, "rt-456", token.RefreshToken)
	require.NotNil(t, token.ExpiresAt())
}

func TestConfig_ExchangeCode_MissingTokenIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"expires_in": 100})
	}))
	defer server.Close()

	cfg := NewConfig("client-1", "secret", "https://cb")
	cfg.TokenURL = server.URL

	_, err := cfg.ExchangeCode(context.Background(), "code")
	assert.Error(t, err)
}

func TestConfig_ExchangeCode_VendorErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	cfg := NewConfig("client-1", "secret", "https://cb")
	cfg.TokenURL = server.URL

	_, err := cfg.ExchangeCode(context.Background(), "code")
	require.Error(t, err)
	// The error message must not echo the client secret back.
	assert.NotContains(t, err.Error(), "secret")
}

func TestConfig_FetchUserInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer at-123", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"sub":         "linkedin-42",
			"email":       "jane@example.com",
			"given_name":  "Jane",
			"family_name": "Doe",
			"headline":    "Engineer",
		})
	}))
	defer server.Close()

	cfg := NewConfig("client-1", "secret", "https://cb")
	cfg.UserInfoURL = server.URL

	info, err := cfg.FetchUserInfo(context.Background(), "at-123")
	require.NoError(t, err)
	assert.Equal(t, "linkedin-42", info.Sub)
	assert.Equal(t, "jane@example.com", info.Email)
	assert.Equal(t, "Jane", info.GivenName)
}

func TestConfig_FetchUserInfo_RejectsEmptyToken(t *testing.T) {
	cfg := NewConfig("client-1", "secret", "https://cb")
	_, err := cfg.FetchUserInfo(context.Background(), "")
	assert.Error(t, err)
}

func readBody(r *http.Request) string {
	var sb strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := r.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			return sb.String()
		}
	}
}
