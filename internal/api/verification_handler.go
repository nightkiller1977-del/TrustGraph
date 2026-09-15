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
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// VerificationHandler serves the Plane B consent, verification, and badge
// endpoints. Plane B data is consented-only: every read and write first
// resolves the subject and (for writes) checks an active consent.
type VerificationHandler struct {
	db       *store.PostgresDB
	logger   *zap.Logger
	cfg      *config.Config
	subjects *store.SubjectRepository
	consents *store.ConsentRepository
	verifs   *store.VerificationRepository
	edu      *store.EducationRepository
	linkedin *store.LinkedInRepository
	auditor  *audit.AuditLogger

	service *verificationService
}

func NewVerificationHandler(db *store.PostgresDB, logger *zap.Logger, cfg *config.Config) *VerificationHandler {
	return &VerificationHandler{
		db:       db,
		logger:   logger,
		cfg:      cfg,
		subjects: store.NewSubjectRepository(db),
		consents: store.NewConsentRepository(db),
		verifs:   store.NewVerificationRepository(db),
		edu:      store.NewEducationRepository(db),
		linkedin: store.NewLinkedInRepository(db),
		auditor:  audit.NewAuditLogger(db, logger),
		service:  newVerificationService(cfg),
	}
}

// resolveSubject maps a ConnectionSphere user ID to a subject UUID, creating
// the subject row if needed so verification can start before/independently of
// an assessment.
func (h *VerificationHandler) resolveSubject(ctx context.Context, csUserID string) (uuid.UUID, bool) {
	if csUserID == "" {
		return uuid.Nil, false
	}
	id, err := h.subjects.FindOrCreateSubject(ctx, csUserID)
	if err != nil {
		h.logger.Error("resolve subject failed", zap.Error(err))
		return uuid.Nil, false
	}
	return id, true
}

// ListConsents returns every consent held for a subject.
func (h *VerificationHandler) ListConsents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	subjectID, ok := h.resolveSubject(ctx, mux.Vars(r)["csUserId"])
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown or missing subject")
		return
	}

	consents, err := h.consents.ListBySubject(ctx, subjectID)
	if err != nil {
		h.logger.Error("list consents failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load consents")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"subjectId": subjectID.String(),
		"consents":  consents,
	})
}

// GrantConsent records a subject's opt-in for one Plane B data purpose.
func (h *VerificationHandler) GrantConsent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	csUserID := mux.Vars(r)["csUserId"]
	subjectID, ok := h.resolveSubject(ctx, csUserID)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown or missing subject")
		return
	}

	var req models.ConsentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}
	if !isKnownConsentType(req.ConsentType) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "consentType must be one of: linkedin_oauth, government_id, liveness, image_verification")
		return
	}

	consent, err := h.consents.GrantConsent(ctx, subjectID, audit.PlaneB, req.ConsentType, req.PolicyVersion, req.TermsAccepted)
	if err != nil {
		h.logger.Error("grant consent failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to record consent")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane:        audit.PlaneB,
		Action:       audit.ActionConsentGranted,
		Actor:        csUserID,
		ActorType:    audit.ActorTypeUser,
		ResourceType: "consent",
		SubjectID:    &subjectID,
		Details:      map[string]interface{}{"consentType": req.ConsentType},
		Result:       "ok",
		RequestID:    r.Header.Get("X-Request-ID"),
	})

	writeJSON(w, http.StatusCreated, consent)
}

// WithdrawConsent revokes a subject's consent for one Plane B data purpose and
// erases the data collected under it.
func (h *VerificationHandler) WithdrawConsent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	csUserID := mux.Vars(r)["csUserId"]
	consentType := mux.Vars(r)["consentType"]
	if !isKnownConsentType(consentType) {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "consentType must be one of: linkedin_oauth, government_id, liveness, image_verification")
		return
	}
	subjectID, err := h.subjects.GetSubjectByCSUserID(ctx, csUserID)
	if err != nil {
		h.logger.Error("subject lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to resolve subject")
		return
	}
	if subjectID == uuid.Nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "Subject not found")
		return
	}

	consent, err := h.consents.WithdrawConsent(ctx, subjectID, audit.PlaneB, consentType)
	if err != nil {
		h.logger.Error("withdraw consent failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to withdraw consent")
		return
	}
	if consent == nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "No active consent found for that type")
		return
	}

	// A withdrawn consent must take its data with it, otherwise "withdraw" would
	// leave the subject's data in place while claiming it was revoked. The erase
	// is scoped to this consent type so withdrawing one purpose cannot destroy
	// data the subject still consented to.
	if err := h.consents.DeleteSubjectPlaneBDataForConsent(ctx, subjectID, consentType); err != nil {
		h.logger.Error("plane B data deletion failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Consent withdrawn but data deletion failed; retry required")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane:        audit.PlaneB,
		Action:       audit.ActionConsentWithdrawn,
		Actor:        csUserID,
		ActorType:    audit.ActorTypeUser,
		ResourceType: "consent",
		SubjectID:    &subjectID,
		Details:      map[string]interface{}{"consentType": consentType, "dataDeleted": true},
		Result:       "ok",
		RequestID:    r.Header.Get("X-Request-ID"),
	})

	writeJSON(w, http.StatusOK, consent)
}

// VerifiedStatus summarises consents and verification attempts for a subject.
func (h *VerificationHandler) VerifiedStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	subjectID, err := h.subjects.GetSubjectByCSUserID(ctx, mux.Vars(r)["csUserId"])
	if err != nil {
		h.logger.Error("subject lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to resolve subject")
		return
	}
	if subjectID == uuid.Nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "Subject not found")
		return
	}

	consents, err := h.consents.ListBySubject(ctx, subjectID)
	if err != nil {
		h.logger.Error("list consents failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load consents")
		return
	}
	verifications, err := h.verifs.ListBySubject(ctx, subjectID)
	if err != nil {
		h.logger.Error("list verifications failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load verifications")
		return
	}

	writeJSON(w, http.StatusOK, models.VerificationStatusResponse{
		SubjectID:     subjectID.String(),
		Consents:      consents,
		Verifications: verifications,
	})
}

// GetBadges assembles the user-visible verification badges for a subject.
func (h *VerificationHandler) GetBadges(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	subjectID, err := h.subjects.GetSubjectByCSUserID(ctx, mux.Vars(r)["csUserId"])
	if err != nil {
		h.logger.Error("subject lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to resolve subject")
		return
	}
	if subjectID == uuid.Nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "Subject not found")
		return
	}

	flags, err := h.verifs.GetSubjectVerificationFlags(ctx, subjectID)
	if err != nil {
		h.logger.Error("verification flags lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load badges")
		return
	}

	badges := []models.Badge{}
	if flags != nil {
		if flags.HasGovernmentID {
			badges = append(badges, models.Badge{
				Type: models.VerificationTypeGovernmentID, State: models.BadgeStateVerified,
				Label: "Government ID verified", UpdatedAt: flags.VerifiedAt,
			})
		}
		if flags.HasLiveness {
			badges = append(badges, models.Badge{
				Type: models.VerificationTypeLiveness, State: models.BadgeStateVerified,
				Label: "Liveness check passed", UpdatedAt: flags.VerifiedAt,
			})
		}
	}

	edu, err := h.edu.GetEducationBySubject(ctx, subjectID)
	if err != nil {
		h.logger.Error("education lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to load badges")
		return
	}
	if edu != nil {
		state := models.BadgeStateSelfReported
		if edu.IsVerified {
			state = models.BadgeStateVerified
		}
		badges = append(badges, models.Badge{
			Type: models.VerificationTypeEducation, State: state,
			Label:  edu.SchoolName,
			Detail: edu.ValidationDetails,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"subjectId": subjectID.String(),
		"badges":    badges,
	})
}

// DeleteVerification erases everything Plane B holds for a subject. It backs
// the privacy right-to-erasure requirement independently of withdrawal.
func (h *VerificationHandler) DeleteVerification(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	csUserID := mux.Vars(r)["csUserId"]
	subjectID, err := h.subjects.GetSubjectByCSUserID(ctx, csUserID)
	if err != nil {
		h.logger.Error("subject lookup failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to resolve subject")
		return
	}
	if subjectID == uuid.Nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "Subject not found")
		return
	}

	if err := h.consents.DeleteSubjectPlaneBData(ctx, subjectID); err != nil {
		h.logger.Error("delete verification data failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to delete verification data")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane:        audit.PlaneB,
		Action:       audit.ActionVerificationDataDeleted,
		Actor:        csUserID,
		ActorType:    audit.ActorTypeUser,
		ResourceType: "subject",
		SubjectID:    &subjectID,
		Result:       "ok",
		RequestID:    r.Header.Get("X-Request-ID"),
	})

	w.WriteHeader(http.StatusNoContent)
}

func isKnownConsentType(t string) bool {
	switch t {
	case models.ConsentTypeLinkedIn, models.ConsentTypeGovernmentID,
		models.ConsentTypeLiveness, models.ConsentTypeImageVerification:
		return true
	default:
		return false
	}
}
