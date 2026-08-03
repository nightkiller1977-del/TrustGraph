package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// NewRouter creates and configures the HTTP router
func NewRouter(db *store.PostgresDB, logger *zap.Logger) *mux.Router {
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", healthCheck).Methods("GET")

	// v1 Assessment endpoints
	v1 := router.PathPrefix("/v1").Subrouter()

	assessmentHandler := NewAssessmentHandler(db, logger)

	v1.HandleFunc("/assessments", assessmentHandler.CreateAssessment).Methods("POST")
	v1.HandleFunc("/assessments/{assessmentId}", assessmentHandler.GetAssessment).Methods("GET")

	// Add middleware
	router.Use(loggingMiddleware(logger))
	router.Use(jsonContentTypeMiddleware)

	return router
}

// healthCheck is a simple health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// jsonContentTypeMiddleware sets Content-Type to application/json for all responses
func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
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
