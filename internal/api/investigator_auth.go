package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
)

// Investigator roles, from least to most privileged.
//
//   - viewer:       read cases and their notes; cannot run tools or write.
//   - investigator: everything a viewer can do, plus open/update cases, add
//     notes, and run OSINT tools.
//   - supervisor:   everything an investigator can do, plus close cases and
//     review break-glass grants.
const (
	RoleInvestigatorViewer       = "viewer"
	RoleInvestigatorInvestigator = "investigator"
	RoleInvestigatorSupervisor   = "supervisor"
)

type investigatorContextKey string

const (
	ctxKeyInvestigatorActor investigatorContextKey = "investigator_actor"
	ctxKeyInvestigatorRole  investigatorContextKey = "investigator_role"
)

// investigatorAuth resolves the caller's actor name and role from a bearer
// token.
//
// The deployment is configured with a single INVESTIGATOR_TOKEN whose role is
// "investigator" (the common case: one shared service account for the
// investigation console). The token is compared in constant time. When the
// token is unset the endpoint is unavailable (503) rather than open, matching
// how the admin routes refuse to run unconfigured.
func investigatorAuth(cfg *config.Config, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.InvestigatorsConfigured() {
				logger.Error("investigator endpoint called but INVESTIGATOR_TOKEN is not set")
				writeJSONError(w, http.StatusServiceUnavailable, "not_configured", "Investigator access is not configured")
				return
			}

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "Authorization: Bearer <token> required")
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")

			role := RoleInvestigatorViewer
			switch {
			case subtle.ConstantTimeCompare([]byte(token), []byte(cfg.InvestigatorToken)) == 1:
				role = RoleInvestigatorInvestigator
			case cfg.AdminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AdminToken)) == 1:
				// An admin token is also accepted, at supervisor level.
				role = RoleInvestigatorSupervisor
			default:
				logger.Warn("investigator auth failed", zap.String("remote_addr", r.RemoteAddr))
				writeJSONError(w, http.StatusForbidden, "forbidden", "Invalid investigator token")
				return
			}

			actor := r.Header.Get("X-Investigator-Actor")
			if actor == "" {
				actor = "investigator"
			}

			ctx := context.WithValue(r.Context(), ctxKeyInvestigatorActor, actor)
			ctx = context.WithValue(ctx, ctxKeyInvestigatorRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// investigatorActor returns the authenticated actor name.
func investigatorActor(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyInvestigatorActor).(string); ok {
		return v
	}
	return "unknown"
}

// investigatorRole returns the authenticated role.
func investigatorRole(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyInvestigatorRole).(string); ok {
		return v
	}
	return RoleInvestigatorViewer
}

// requireRole enforces a minimum role on a handler. Handlers call it directly
// (rather than relying on a route-level middleware) so the check is visible at
// the point it protects.
func requireRole(ctx context.Context, w http.ResponseWriter, minimum string) bool {
	if roleRank(investigatorRole(ctx)) >= roleRank(minimum) {
		return true
	}
	writeJSONError(w, http.StatusForbidden, "forbidden", "Your investigator role does not permit this action")
	return false
}

func roleRank(role string) int {
	switch role {
	case RoleInvestigatorSupervisor:
		return 3
	case RoleInvestigatorInvestigator:
		return 2
	case RoleInvestigatorViewer:
		return 1
	default:
		return 0
	}
}
