package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/models"
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
// investigation console), and optionally an ADMIN_TOKEN that maps to
// "supervisor". Both are compared in constant time. The endpoint is
// unavailable (503) only when neither is set, matching how the admin routes
// refuse to run unconfigured.
//
// Actor identity: with a shared token there is no per-user identity to derive
// an actor from, so X-Investigator-Actor is honoured ONLY when the deployment
// declares a trusted identity-aware proxy in front of the service
// (TrustedProxyActorHeader). Otherwise every action is attributed to the
// token's own identity, so one investigator cannot forge another's audit
// trail. The actor is never taken from an untrusted caller.
func investigatorAuth(cfg *config.Config, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.InvestigatorsConfigured() {
				logger.Error("investigator endpoint called but neither INVESTIGATOR_TOKEN nor ADMIN_TOKEN is set")
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
			actor := ""
			switch {
			case cfg.InvestigatorToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(cfg.InvestigatorToken)) == 1:
				role = RoleInvestigatorInvestigator
				actor = "investigator"
			case cfg.AdminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(cfg.AdminToken)) == 1:
				// An admin token is also accepted, at supervisor level.
				role = RoleInvestigatorSupervisor
				actor = "admin"
			default:
				logger.Warn("investigator auth failed", zap.String("remote_addr", r.RemoteAddr))
				writeJSONError(w, http.StatusForbidden, "forbidden", "Invalid investigator token")
				return
			}

			// A per-request actor is only trustworthy when an identity-aware
			// proxy sets it; otherwise it is caller-controlled and would let one
			// investigator impersonate another in the audit trail.
			if cfg.TrustedProxyActorHeader {
				if proxyActor := r.Header.Get("X-Investigator-Actor"); proxyActor != "" {
					actor = proxyActor
				}
			} else if r.Header.Get("X-Investigator-Actor") != "" {
				logger.Warn("ignoring X-Investigator-Actor: no trusted proxy is configured to set it",
					zap.String("remote_addr", r.RemoteAddr))
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

// breakGlassStore is the slice of the case store requireRole needs to honour an
// active emergency grant. It is an interface so the middleware can be tested
// without a database.
type breakGlassStore interface {
	ActiveBreakGlassGrant(ctx context.Context, actor string, subjectID, caseID *uuid.UUID) (*models.BreakGlassGrant, error)
}

// requireRoleOrBreakGlass enforces a minimum role, but also permits the action
// when the caller holds an unexpired, unrevoked break-glass grant covering the
// target. Without this the grant would be recorded and audited yet have no
// effect, leaving the emergency path non-functional.
//
// A grant confers supervisor-level access for its window: its whole purpose is
// elevation beyond the caller's standing role, so the role recorded at request
// time must not cap it. Every elevation is logged, because using a break-glass
// grant is exactly the event the audit trail exists to capture.
//
// The returned grant is non-nil only when the action was authorised by
// break-glass, so the caller can mark the access-log entry accordingly.
func requireRoleOrBreakGlass(ctx context.Context, w http.ResponseWriter, minimum string, subjectID, caseID *uuid.UUID, cases breakGlassStore, logger *zap.Logger) *models.BreakGlassGrant {
	if roleRank(investigatorRole(ctx)) >= roleRank(minimum) {
		return nil
	}

	if cases != nil {
		actor := investigatorActor(ctx)
		grant, err := cases.ActiveBreakGlassGrant(ctx, actor, subjectID, caseID)
		if err != nil {
			logger.Error("break-glass lookup failed", zap.Error(err))
		} else if grant != nil {
			logger.Warn("privileged action authorised by break-glass grant",
				zap.String("actor", actor),
				zap.String("required_role", minimum),
				zap.String("grant_id", grant.GrantID),
			)
			return grant
		}
	}

	writeJSONError(w, http.StatusForbidden, "forbidden", "Your investigator role does not permit this action")
	return nil
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
