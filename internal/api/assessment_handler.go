package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/audit"
	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/policy"
	"github.com/nightkiller1977-del/trustgraph/internal/signals"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

type AssessmentHandler struct {
	db           *store.PostgresDB
	logger       *zap.Logger
	cfg          *config.Config
	repo         *store.AssessmentRepository
	subjects     *store.SubjectRepository
	observations *store.ObservationRepository
	evaluator    *signals.Evaluator
	policyEngine *policy.Engine
	auditor      *audit.AuditLogger
}

func NewAssessmentHandler(db *store.PostgresDB, logger *zap.Logger, cfg *config.Config) *AssessmentHandler {
	return &AssessmentHandler{
		db:           db,
		logger:       logger,
		cfg:          cfg,
		repo:         store.NewAssessmentRepository(db),
		subjects:     store.NewSubjectRepository(db),
		observations: store.NewObservationRepository(db),
		evaluator:    signals.NewEvaluator(logger),
		policyEngine: policy.NewEngine(logger),
		auditor:      audit.NewAuditLogger(db, logger),
	}
}

func (h *AssessmentHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	enforcementMode := audit.EnforcementModeShadow
	if h.cfg.EnforcementEnabled {
		enforcementMode = audit.EnforcementModeEnforced
	}

	var req models.AssessmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	if req.ContractVersion == "" || req.IdempotencyKey == "" || req.Subject.ConnectionSphereUserID == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Missing required fields: contractVersion, idempotencyKey, subject.connectionSphereUserId")
		return
	}

	// Idempotency check (respects configured TTL)
	existing, err := h.repo.GetAssessmentByIdempotencyKey(ctx, req.IdempotencyKey, h.cfg.IdempotencyTTLHours)
	if err != nil {
		h.logger.Error("idempotency check failed", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process request")
		return
	}
	if existing != nil {
		h.auditor.LogAssessment(ctx, audit.ActionAssessmentCached, &existing.AssessmentID, existing.SubjectID, map[string]interface{}{
			"idempotencyKey": req.IdempotencyKey,
		}, requestID)
		cachedResp := h.modelToResponse(existing)
		cachedResp.EnforcementMode = enforcementMode
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(cachedResp)
		return
	}

	// Find or create subject (revives soft-deleted subjects)
	subjectID, err := h.subjects.FindOrCreateSubject(ctx, req.Subject.ConnectionSphereUserID)
	if err != nil {
		h.logger.Error("subject upsert failed", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create subject")
		return
	}

	h.auditor.LogAssessment(ctx, audit.ActionAssessmentRequested, nil, subjectID, map[string]interface{}{
		"contractVersion":        req.ContractVersion,
		"connectionSphereUserId": req.Subject.ConnectionSphereUserID,
	}, requestID)

	// Build evaluation context from request
	evalCtx := &signals.EvalContext{
		SubjectID:              subjectID.String(),
		ConnectionSphereUserID: req.Subject.ConnectionSphereUserID,
		Email:                  req.Subject.Email,
		Phone:                  req.Subject.Phone,
		EmailVerified:          req.Signals.EmailVerified,
		PhoneVerified:          req.Signals.PhoneVerified,
		DeviceFingerprint:      req.Signals.DeviceFingerprint,
		DeviceToken:            req.Signals.DeviceToken,
		IPAddress:              req.Signals.IPAddress,
		ImageHash:              req.Signals.ImageHash,
	}
	if req.RequestContext != nil {
		evalCtx.UserAgent = req.RequestContext.UserAgent
	}

	// Run signal providers
	signalResults := h.evaluator.EvaluateAll(ctx, evalCtx, h.db)

	// Convert signal results to policy input
	policySignals := make([]policy.SignalResult, len(signalResults))
	var processed []string
	var skipped []string
	for i, sr := range signalResults {
		policySignals[i] = policy.SignalResult{
			Provider:    sr.Provider,
			ReasonCodes: sr.ReasonCodes,
			Score:       sr.Score,
			Confidence:  sr.Confidence,
			Error:       sr.Error,
		}
		if sr.Error != nil {
			skipped = append(skipped, sr.Provider)
		} else {
			processed = append(processed, sr.Provider)
		}

		h.auditor.LogSignal(ctx, sr.Provider, subjectID, statusFromError(sr.Error), map[string]interface{}{
			"score":       sr.Score,
			"confidence":  sr.Confidence,
			"reasonCodes": sr.ReasonCodes,
		}, requestID)
	}

	// Run policy engine
	policyResult := h.policyEngine.Evaluate(policySignals)

	now := time.Now()
	assessment := &models.Assessment{
		AssessmentID:    uuid.New(),
		ContractVersion: req.ContractVersion,
		IdempotencyKey:  req.IdempotencyKey,
		SubjectID:       subjectID,
		AssessmentType:  "registration",
		TrustTier:       policyResult.TrustTier,
		RiskBand:        policyResult.RiskBand,
		RiskScore:       policyResult.RiskScore,
		Decision:        policyResult.Decision,
		ReasonCodes:     policyResult.ReasonCodes,
		RequiredActions: policyResult.RequiredActions,
		PolicyVersion:   policyResult.PolicyVersion,
		Status:          models.AssessmentStatusComplete,
		CreatedAt:       now,
		UpdatedAt:       now,
		CompletedAt:     &now,
	}

	if err := h.repo.CreateAssessment(ctx, assessment); err != nil {
		h.logger.Error("assessment persist failed", zap.Error(err))
		h.auditor.LogAssessmentError(ctx, audit.ActionAssessmentFailed, &assessment.AssessmentID, subjectID, map[string]interface{}{
			"error": err.Error(),
		}, err.Error(), requestID)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create assessment")
		return
	}

	// Record observation AFTER assessment exists so the FK is satisfied
	if err := h.observations.RecordObservation(ctx, assessment.AssessmentID, subjectID, "registration", "A", "trustgraph-api", map[string]interface{}{
		"ip_address":         req.Signals.IPAddress,
		"device_fingerprint": req.Signals.DeviceFingerprint,
		"image_hash":         req.Signals.ImageHash,
		"email":              req.Subject.Email,
	}, 1.0); err != nil {
		h.logger.Warn("failed to record observation (non-fatal)", zap.Error(err))
	}

	h.auditor.LogAssessment(ctx, audit.ActionAssessmentCompleted, &assessment.AssessmentID, subjectID, map[string]interface{}{
		"trustTier":       assessment.TrustTier,
		"riskBand":        assessment.RiskBand,
		"riskScore":       assessment.RiskScore,
		"decision":        assessment.Decision,
		"enforcementMode": enforcementMode,
	}, requestID)

	h.logger.Info("assessment created",
		zap.String("assessment_id", assessment.AssessmentID.String()),
		zap.String("trust_tier", assessment.TrustTier),
		zap.String("decision", assessment.Decision),
		zap.Int("risk_score", assessment.RiskScore),
	)

	resp := h.modelToResponse(assessment)
	resp.Signals = models.SignalsProcessed{Processed: processed, Skipped: skipped}
	resp.EnforcementMode = enforcementMode

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *AssessmentHandler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	assessmentID, err := uuid.Parse(mux.Vars(r)["assessmentId"])
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid assessment ID format")
		return
	}

	assessment, err := h.repo.GetAssessmentByID(ctx, assessmentID)
	if err != nil {
		h.logger.Error("get assessment failed", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve assessment")
		return
	}

	if assessment == nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Assessment not found")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(h.modelToResponse(assessment))
}

func (h *AssessmentHandler) modelToResponse(a *models.Assessment) models.AssessmentResponse {
	return models.AssessmentResponse{
		ContractVersion: a.ContractVersion,
		AssessmentID:    a.AssessmentID.String(),
		Status:          a.Status,
		TrustTier:       a.TrustTier,
		RiskBand:        a.RiskBand,
		RiskScore:       a.RiskScore,
		Decision:        a.Decision,
		ReasonCodes:     a.ReasonCodes,
		RequiredActions: a.RequiredActions,
		PolicyVersion:   a.PolicyVersion,
		CompletedAt:     a.CompletedAt,
	}
}

func (h *AssessmentHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSONError(w, status, code, message)
}

func statusFromError(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
