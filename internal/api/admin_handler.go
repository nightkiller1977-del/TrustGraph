package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/audit"
	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// pqUniqueViolation is the PostgreSQL error code for a unique-constraint
// violation (23505). Handlers use it to turn a check-then-insert race into a
// clean 409 instead of a generic 500, since the DB constraint is the actual
// source of truth — the pre-insert existence check is just a fast path.
const pqUniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == pqUniqueViolation
}

type AdminHandler struct {
	logger  *zap.Logger
	cfg     *config.Config
	repo    *store.AssessmentRepository
	reviews *store.ReviewRepository
	auditor *audit.AuditLogger
}

func NewAdminHandler(db *store.PostgresDB, logger *zap.Logger, cfg *config.Config) *AdminHandler {
	return &AdminHandler{
		logger:  logger,
		cfg:     cfg,
		repo:    store.NewAssessmentRepository(db),
		reviews: store.NewReviewRepository(db),
		auditor: audit.NewAuditLogger(db, logger),
	}
}

// GetQueue returns assessments that have not yet been reviewed.
// Query params: risk_band (optional), page (default 1), per_page (default 20).
func (h *AdminHandler) GetQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	riskBand := r.URL.Query().Get("risk_band")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	items, err := h.repo.ListPendingReview(ctx, riskBand, page, perPage)
	if err != nil {
		h.logger.Error("admin queue query failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load queue")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane:        audit.PlaneA,
		Action:       audit.ActionAdminQueueViewed,
		Actor:        "admin",
		ActorType:    audit.ActorTypeInvestigator,
		ResourceType: "queue",
		Details: map[string]interface{}{
			"riskBandFilter": riskBand,
			"count":          len(items),
		},
		Result:    "ok",
		RequestID: r.Header.Get("X-Request-ID"),
	})

	if items == nil {
		items = []models.QueueItem{}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"items": items,
		"page":  page,
	})
}

type submitReviewRequest struct {
	ReviewerEmail string `json:"reviewerEmail"`
	Outcome       string `json:"outcome"`
	Notes         string `json:"notes"`
}

// SubmitReview records a human reviewer's outcome for an assessment.
func (h *AdminHandler) SubmitReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	assessmentID, err := uuid.Parse(mux.Vars(r)["assessmentId"])
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid assessment ID")
		return
	}

	var req submitReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	if req.ReviewerEmail == "" || req.Outcome == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "reviewerEmail and outcome are required")
		return
	}

	switch models.ReviewOutcome(req.Outcome) {
	case models.ReviewOutcomeConfirmedAbuse, models.ReviewOutcomeLegitimate,
		models.ReviewOutcomeInconclusive, models.ReviewOutcomeError:
		// valid
	default:
		writeJSONError(w, http.StatusBadRequest, "bad_request",
			"outcome must be one of: confirmed_abuse, legitimate, inconclusive, error")
		return
	}

	// Idempotency: reject duplicate reviews on the same assessment
	existing, err := h.reviews.GetByAssessmentID(ctx, assessmentID)
	if err != nil {
		h.logger.Error("review lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to check existing review")
		return
	}
	if existing != nil {
		writeJSONError(w, http.StatusConflict, "already_reviewed", "This assessment has already been reviewed")
		return
	}

	review, err := h.reviews.CreateReview(ctx, assessmentID, req.ReviewerEmail, models.ReviewOutcome(req.Outcome), req.Notes)
	if err != nil {
		if isUniqueViolation(err) {
			writeJSONError(w, http.StatusConflict, "already_reviewed", "This assessment has already been reviewed")
			return
		}
		h.logger.Error("review creation failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to create review")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane:        audit.PlaneA,
		Action:       audit.ActionAdminReviewSubmitted,
		Actor:        req.ReviewerEmail,
		ActorType:    audit.ActorTypeInvestigator,
		ResourceType: "assessment",
		Details: map[string]interface{}{
			"assessmentId": assessmentID.String(),
			"outcome":      req.Outcome,
		},
		Result:    "ok",
		RequestID: r.Header.Get("X-Request-ID"),
	})

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(review)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
