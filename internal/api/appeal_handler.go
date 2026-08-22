package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/audit"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// AppealHandler stubs the appeal workflow for Phase 2 enforcement.
// Phase 1.5 accepts and persists appeals but takes no automated action.
type AppealHandler struct {
	logger  *zap.Logger
	appeals *store.AppealRepository
	auditor *audit.AuditLogger
}

func NewAppealHandler(db *store.PostgresDB, logger *zap.Logger) *AppealHandler {
	return &AppealHandler{
		logger:  logger,
		appeals: store.NewAppealRepository(db),
		auditor: audit.NewAuditLogger(db, logger),
	}
}

type submitAppealRequest struct {
	UserMessage string `json:"userMessage"`
}

// SubmitAppeal records a user's appeal of a limited-tier outcome.
// In Phase 1.5 this only persists the record; reviewers will process it manually.
func (h *AppealHandler) SubmitAppeal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	assessmentID, err := uuid.Parse(mux.Vars(r)["assessmentId"])
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid assessment ID")
		return
	}

	var req submitAppealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	// Prevent duplicate appeals
	existing, err := h.appeals.GetByAssessmentID(ctx, assessmentID)
	if err != nil {
		h.logger.Error("appeal lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to check existing appeal")
		return
	}
	if existing != nil {
		writeJSONError(w, http.StatusConflict, "already_appealed", "An appeal already exists for this assessment")
		return
	}

	appeal, err := h.appeals.CreateAppeal(ctx, assessmentID, req.UserMessage)
	if err != nil {
		if isUniqueViolation(err) {
			writeJSONError(w, http.StatusConflict, "already_appealed", "An appeal already exists for this assessment")
			return
		}
		h.logger.Error("appeal creation failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to create appeal")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane:        audit.PlaneA,
		Action:       audit.ActionAppealSubmitted,
		Actor:        "user",
		ActorType:    audit.ActorTypeUser,
		ResourceType: "assessment",
		Details:      map[string]interface{}{"assessmentId": assessmentID.String()},
		Result:       "ok",
		RequestID:    r.Header.Get("X-Request-ID"),
	})

	h.logger.Info("appeal submitted", zap.String("assessment_id", assessmentID.String()))

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(appeal)
}
