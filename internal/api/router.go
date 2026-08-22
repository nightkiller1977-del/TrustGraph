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
