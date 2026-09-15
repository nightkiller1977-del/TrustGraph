package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// NewRouter creates and configures the HTTP router.
func NewRouter(db *store.PostgresDB, logger *zap.Logger, cfg *config.Config) *mux.Router {
	router := mux.NewRouter()

	router.HandleFunc("/health", healthCheck).Methods("GET")

	v1 := router.PathPrefix("/v1").Subrouter()

	// Assessment endpoints (public — authenticated via app service account at the network level)
	assessmentHandler := NewAssessmentHandler(db, logger, cfg)
	v1.HandleFunc("/assessments", assessmentHandler.CreateAssessment).Methods("POST")
	v1.HandleFunc("/assessments/{assessmentId}", assessmentHandler.GetAssessment).Methods("GET")

	// Appeal endpoint (user-facing, no admin auth)
	appealHandler := NewAppealHandler(db, logger)
	v1.HandleFunc("/appeals/{assessmentId}", appealHandler.SubmitAppeal).Methods("POST")

	// Plane B: consent and verification (user-facing, subject-scoped)
	verificationHandler := NewVerificationHandler(db, logger, cfg)
	verify := v1.PathPrefix("/verification").Subrouter()
	verify.HandleFunc("/subjects/{csUserId}/consents", verificationHandler.ListConsents).Methods("GET")
	verify.HandleFunc("/subjects/{csUserId}/consents", verificationHandler.GrantConsent).Methods("POST")
	verify.HandleFunc("/subjects/{csUserId}/consents/{consentType}", verificationHandler.WithdrawConsent).Methods("DELETE")
	verify.HandleFunc("/subjects/{csUserId}/status", verificationHandler.VerifiedStatus).Methods("GET")
	verify.HandleFunc("/subjects/{csUserId}/badges", verificationHandler.GetBadges).Methods("GET")
	verify.HandleFunc("/subjects/{csUserId}", verificationHandler.DeleteVerification).Methods("DELETE")

	// Plane B verification flows
	verify.HandleFunc("/subjects/{csUserId}/linkedin/authorize", verificationHandler.LinkedInAuthorize).Methods("POST")
	verify.HandleFunc("/linkedin/callback", verificationHandler.LinkedInCallback).Methods("GET")
	verify.HandleFunc("/subjects/{csUserId}/government-id", verificationHandler.GovernmentIDVerify).Methods("POST")
	verify.HandleFunc("/subjects/{csUserId}/liveness", verificationHandler.LivenessVerify).Methods("POST")
	verify.HandleFunc("/subjects/{csUserId}/image", verificationHandler.ImageVerify).Methods("POST")

	// Admin endpoints — require Bearer token
	adminAuth := requireAdminAuth(cfg, logger)
	admin := v1.PathPrefix("/admin").Subrouter()
	admin.Use(adminAuth)
	adminHandler := NewAdminHandler(db, logger, cfg)
	admin.HandleFunc("/queue", adminHandler.GetQueue).Methods("GET")
	admin.HandleFunc("/reviews/{assessmentId}", adminHandler.SubmitReview).Methods("POST")

	// Metrics endpoint — require Bearer token
	metrics := v1.PathPrefix("/metrics").Subrouter()
	metrics.Use(adminAuth)
	metricsHandler := NewMetricsHandler(db, logger, cfg)
	metrics.HandleFunc("/calibration", metricsHandler.GetCalibration).Methods("GET")

	// Plane C: investigations — require investigator Bearer token
	investigationHandler := NewInvestigationHandler(db, logger, cfg)
	investigations := v1.PathPrefix("/investigations").Subrouter()
	investigations.Use(investigatorAuth(cfg, logger))
	investigations.HandleFunc("/tools", investigationHandler.ListTools).Methods("GET")
	investigations.HandleFunc("/cases", investigationHandler.ListCases).Methods("GET")
	investigations.HandleFunc("/cases", investigationHandler.CreateCase).Methods("POST")
	investigations.HandleFunc("/cases/{caseId}", investigationHandler.GetCase).Methods("GET")
	investigations.HandleFunc("/cases/{caseId}", investigationHandler.UpdateCase).Methods("PATCH")
	investigations.HandleFunc("/cases/{caseId}/notes", investigationHandler.AddNote).Methods("POST")
	investigations.HandleFunc("/cases/{caseId}/access-log", investigationHandler.GetAccessLog).Methods("GET")
	investigations.HandleFunc("/tools/{tool}/run", investigationHandler.RunTool).Methods("POST")
	investigations.HandleFunc("/break-glass", investigationHandler.RequestBreakGlass).Methods("POST")

	router.Use(loggingMiddleware(logger))
	router.Use(jsonContentTypeMiddleware)

	return router
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Debug("HTTP request",
				zap.String("method", r.Method),
				zap.String("path", r.RequestURI),
				zap.String("remote_addr", r.RemoteAddr),
			)
			next.ServeHTTP(w, r)
		})
	}
}
