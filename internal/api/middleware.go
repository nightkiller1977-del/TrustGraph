package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
)

// requireAdminAuth enforces Bearer token authentication for admin routes.
// Uses constant-time comparison to prevent timing attacks.
func requireAdminAuth(cfg *config.Config, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.AdminToken == "" {
				logger.Error("admin endpoint called but TRUSTGRAPH_ADMIN_TOKEN is not set")
				http.Error(w, `{"error":"forbidden","message":"Admin access not configured"}`, http.StatusForbidden)
				return
			}

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				http.Error(w, `{"error":"unauthorized","message":"Authorization: Bearer <token> required"}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AdminToken)) != 1 {
				logger.Warn("admin auth failed", zap.String("remote_addr", r.RemoteAddr))
				http.Error(w, `{"error":"forbidden","message":"Invalid admin token"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
