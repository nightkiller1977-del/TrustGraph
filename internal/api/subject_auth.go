package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
)

// ConnectionSphereUserHeader carries the authenticated ConnectSphere user id on
// a Plane B request. It is set by ConnectSphere (the caller holding the subject
// token), not by the end user's browser.
const ConnectionSphereUserHeader = "X-ConnectionSphere-User-Id"

// requireSubjectAuth authenticates the caller of the Plane B verification
// routes and binds the request to a single subject.
//
// These routes act on a caller-supplied {csUserId} and expose consent, status,
// verification submission, and irreversible data deletion. Without this guard
// any client that can reach the API could read or destroy another subject's
// data (the consent check does not help — the same client can grant consent
// first). The subject token authenticates ConnectSphere; the identity header
// then has to match the {csUserId} in the path, so one subject cannot act as
// another.
//
// Like the admin and investigator routes, an unconfigured deployment returns
// 503 rather than falling open.
func requireSubjectAuth(cfg *config.Config, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.SubjectToken == "" {
				logger.Error("Plane B endpoint called but SUBJECT_TOKEN is not set")
				writeJSONError(w, http.StatusServiceUnavailable, "not_configured", "Plane B access is not configured")
				return
			}

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "Authorization: Bearer <token> required")
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.SubjectToken)) != 1 {
				logger.Warn("Plane B auth failed", zap.String("remote_addr", r.RemoteAddr))
				writeJSONError(w, http.StatusForbidden, "forbidden", "Invalid subject token")
				return
			}

			authenticated := r.Header.Get(ConnectionSphereUserHeader)
			if authenticated == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", ConnectionSphereUserHeader+" is required")
				return
			}

			// The path identity must match the authenticated identity. OAuth
			// callback paths carry no {csUserId} and are protected by the
			// single-use CSRF state instead.
			if pathSubject, ok := mux.Vars(r)["csUserId"]; ok && pathSubject != authenticated {
				logger.Warn("Plane B subject mismatch",
					zap.String("path_subject", pathSubject),
					zap.String("authenticated_subject", authenticated),
					zap.String("remote_addr", r.RemoteAddr),
				)
				writeJSONError(w, http.StatusForbidden, "forbidden", "Path subject does not match the authenticated subject")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
