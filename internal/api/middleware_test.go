package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
)

func newTestAdminAuth(adminToken string) http.Handler {
	cfg := &config.Config{AdminToken: adminToken}
	logger := zap.NewNop()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return requireAdminAuth(cfg, logger)(next)
}

func TestRequireAdminAuth_NoTokenConfigured(t *testing.T) {
	handler := newTestAdminAuth("")

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/queue", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json content-type, got %q", ct)
	}
}

func TestRequireAdminAuth_MissingBearerPrefix(t *testing.T) {
	handler := newTestAdminAuth("secret-token")

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/queue", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json content-type, got %q", ct)
	}
}

func TestRequireAdminAuth_WrongToken(t *testing.T) {
	handler := newTestAdminAuth("secret-token")

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/queue", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json content-type, got %q", ct)
	}
}

func TestRequireAdminAuth_ValidToken(t *testing.T) {
	handler := newTestAdminAuth("secret-token")

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/queue", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected request to reach the wrapped handler (200), got %d", rec.Code)
	}
}
