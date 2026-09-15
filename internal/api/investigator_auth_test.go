package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
)

func TestInvestigatorAuth_NotConfiguredReturns503(t *testing.T) {
	handler := investigatorAuth(&config.Config{}, zap.NewNop())(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/investigations/cases", nil)
	req.Header.Set("Authorization", "Bearer anything")
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestInvestigatorAuth_MissingHeaderReturns401(t *testing.T) {
	cfg := &config.Config{InvestigatorToken: "inv-token"}
	handler := investigatorAuth(cfg, zap.NewNop())(okHandler())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestInvestigatorAuth_WrongTokenReturns403(t *testing.T) {
	cfg := &config.Config{InvestigatorToken: "inv-token"}
	handler := investigatorAuth(cfg, zap.NewNop())(okHandler())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestInvestigatorAuth_InvestigatorTokenGrantsInvestigatorRole(t *testing.T) {
	cfg := &config.Config{InvestigatorToken: "inv-token"}
	var gotRole, gotActor string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRole = investigatorRole(r.Context())
		gotActor = investigatorActor(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := investigatorAuth(cfg, zap.NewNop())(next)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer inv-token")
	req.Header.Set("X-Investigator-Actor", "alice@example.com")
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, RoleInvestigatorInvestigator, gotRole)
	assert.Equal(t, "alice@example.com", gotActor)
}

func TestInvestigatorAuth_AdminTokenGrantsSupervisorRole(t *testing.T) {
	cfg := &config.Config{InvestigatorToken: "inv-token", AdminToken: "admin-token"}
	var gotRole string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRole = investigatorRole(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := investigatorAuth(cfg, zap.NewNop())(next)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, RoleInvestigatorSupervisor, gotRole)
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		minimum  string
		wantPass bool
	}{
		{name: "viewer cannot investigate", role: RoleInvestigatorViewer, minimum: RoleInvestigatorInvestigator, wantPass: false},
		{name: "investigator can investigate", role: RoleInvestigatorInvestigator, minimum: RoleInvestigatorInvestigator, wantPass: true},
		{name: "supervisor can investigate", role: RoleInvestigatorSupervisor, minimum: RoleInvestigatorInvestigator, wantPass: true},
		{name: "investigator cannot supervise", role: RoleInvestigatorInvestigator, minimum: RoleInvestigatorSupervisor, wantPass: false},
		{name: "supervisor can supervise", role: RoleInvestigatorSupervisor, minimum: RoleInvestigatorSupervisor, wantPass: true},
		{name: "unknown role is denied", role: "root", minimum: RoleInvestigatorViewer, wantPass: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), ctxKeyInvestigatorRole, tt.role)
			rec := httptest.NewRecorder()
			passed := requireRole(ctx, rec, tt.minimum)
			assert.Equal(t, tt.wantPass, passed)
			if !tt.wantPass {
				assert.Equal(t, http.StatusForbidden, rec.Code)
			}
		})
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
