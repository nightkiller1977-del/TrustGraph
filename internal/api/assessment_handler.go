package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// AssessmentHandler handles assessment-related HTTP requests
type AssessmentHandler struct {
	db     *store.PostgresDB
	logger *zap.Logger
	repo   *store.AssessmentRepository
}

// NewAssessmentHandler creates a new assessment handler
func NewAssessmentHandler(db *store.PostgresDB, logger *zap.Logger) *AssessmentHandler {
	return &AssessmentHandler{
		db:     db,
		logger: logger,
		repo:   store.NewAssessmentRepository(db),
	}
}

// CreateAssessment handles POST /v1/assessments
func (h *AssessmentHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var req models.AssessmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Failed to decode assessment request", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "bad_request",
			"message": "Invalid request body",
		})
		return
	}

	// Validate required fields
	if req.ContractVersion == "" || req.IdempotencyKey == "" || req.Subject.ConnectionSphereUserID == "" {
		h.logger.Warn("Missing required fields in assessment request",
			zap.String("contract_version", req.ContractVersion),
			zap.String("idempotency_key", req.IdempotencyKey),
		)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "bad_request",
			"message": "Missing required fields: contractVersion, idempotencyKey, subject.connectionSphereUserId",
		})
		return
	}

	// Check for existing assessment with same idempotency key
	existingAssessment, err := h.repo.GetAssessmentByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		h.logger.Error("Failed to check for existing assessment", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "internal_error",
			"message": "Failed to process request",
		})
		return
	}

	if existingAssessment != nil {
		// Return cached response
		h.logger.Debug("Returning cached assessment",
			zap.String("assessment_id", existingAssessment.AssessmentID.String()),
			zap.String("idempotency_key", req.IdempotencyKey),
		)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(h.modelToResponse(existingAssessment))
		return
	}

	// Create new subject if needed
	subjectID := uuid.New()

	// Create assessment
	assessment := &models.Assessment{
		AssessmentID:    uuid.New(),
		ContractVersion: req.ContractVersion,
		IdempotencyKey:  req.IdempotencyKey,
		SubjectID:       subjectID,
		AssessmentType:  "registration",
		TrustTier:       models.TrustTierProvisional, // Default to provisional
		RiskBand:        models.RiskBandUnknown,
		RiskScore:       0,
		Decision:        "accept",
		ReasonCodes:     []string{},
		PolicyVersion:   "registration-v1",
		Status:          models.AssessmentStatusComplete, // Phase 1: synchronous
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Evaluate signals (Phase 1: simple stub logic)
	assessment.ReasonCodes = h.evaluateSignals(req.Signals)
	assessment.TrustTier, assessment.RiskBand, assessment.RiskScore = h.decideOutcome(assessment.ReasonCodes)

	// Persist assessment
	if err := h.repo.CreateAssessment(ctx, assessment); err != nil {
		h.logger.Error("Failed to create assessment", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "internal_error",
			"message": "Failed to create assessment",
		})
		return
	}

	completedNow := time.Now()
	assessment.CompletedAt = &completedNow

	h.logger.Info("Assessment created",
		zap.String("assessment_id", assessment.AssessmentID.String()),
		zap.String("trust_tier", assessment.TrustTier),
		zap.String("risk_band", assessment.RiskBand),
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h.modelToResponse(assessment))
}

// GetAssessment handles GET /v1/assessments/{assessmentId}
func (h *AssessmentHandler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	assessmentIDStr := vars["assessmentId"]

	assessmentID, err := uuid.Parse(assessmentIDStr)
	if err != nil {
		h.logger.Warn("Invalid assessment ID format", zap.String("assessment_id", assessmentIDStr))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "bad_request",
			"message": "Invalid assessment ID format",
		})
		return
	}

	assessment, err := h.repo.GetAssessmentByID(ctx, assessmentID)
	if err != nil {
		h.logger.Error("Failed to get assessment", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "internal_error",
			"message": "Failed to retrieve assessment",
		})
		return
	}

	if assessment == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "not_found",
			"message": "Assessment not found",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h.modelToResponse(assessment))
}

// evaluateSignals evaluates available signals and returns reason codes
func (h *AssessmentHandler) evaluateSignals(signals models.SignalsData) []string {
	var reasons []string

	// Phase 1: Simple stub logic
	// In Phase 2, this calls real signal providers

	if signals.EmailVerified {
		reasons = append(reasons, models.ReasonCodeEmailVerified)
	} else {
		reasons = append(reasons, models.ReasonCodeEmailNotVerified)
	}

	if signals.PhoneVerified {
		reasons = append(reasons, models.ReasonCodePhoneVerified)
	} else {
		reasons = append(reasons, models.ReasonCodePhoneNotVerified)
	}

	if signals.DeviceToken == "" {
		reasons = append(reasons, models.ReasonCodeDeviceFirstSeen)
	}

	if signals.ImageHash == "" {
		reasons = append(reasons, models.ReasonCodeImageHashNew)
	}

	return reasons
}

// decideOutcome determines trust tier, risk band, and risk score from reason codes
func (h *AssessmentHandler) decideOutcome(reasonCodes []string) (string, string, int) {
	// Phase 1: Simple heuristic
	// In Phase 2+, this becomes a policy engine

	var score int
	var tier string
	var band string

	// Default
	score = 50
	tier = models.TrustTierProvisional
	band = models.RiskBandUnknown

	// Check for verification signals
	emailVerified := contains(reasonCodes, models.ReasonCodeEmailVerified)
	phoneVerified := contains(reasonCodes, models.ReasonCodePhoneVerified)

	if emailVerified && phoneVerified {
		score = 20
		tier = models.TrustTierStandard
		band = models.RiskBandLow
	} else if emailVerified {
		score = 40
		tier = models.TrustTierProvisional
		band = models.RiskBandLow
	}

	return tier, band, score
}

// modelToResponse converts an Assessment model to a response
func (h *AssessmentHandler) modelToResponse(assessment *models.Assessment) models.AssessmentResponse {
	resp := models.AssessmentResponse{
		ContractVersion: assessment.ContractVersion,
		AssessmentID:    assessment.AssessmentID.String(),
		Status:          assessment.Status,
		TrustTier:       assessment.TrustTier,
		RiskBand:        assessment.RiskBand,
		RiskScore:       assessment.RiskScore,
		ReasonCodes:     assessment.ReasonCodes,
		PolicyVersion:   assessment.PolicyVersion,
		CompletedAt:     assessment.CompletedAt,
	}

	// Determine required actions based on tier
	if assessment.TrustTier == models.TrustTierProvisional {
		resp.RequiredActions = append(resp.RequiredActions, models.RequiredActionVerifyEmail)
		resp.RequiredActions = append(resp.RequiredActions, models.RequiredActionVerifyPhone)
	}

	if assessment.TrustTier == models.TrustTierLimited {
		resp.RequiredActions = append(resp.RequiredActions, models.RequiredActionReviewByHuman)
	}

	resp.Signals = models.SignalsProcessed{
		Processed: []string{}, // Phase 1: stub
		Skipped:   []string{},
	}

	return resp
}

// contains checks if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
