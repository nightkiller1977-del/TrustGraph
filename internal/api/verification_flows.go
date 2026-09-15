package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/audit"
	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/oauth"
	"github.com/nightkiller1977-del/trustgraph/internal/policy"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
	"github.com/nightkiller1977-del/trustgraph/internal/verification"
)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// maxUploadPayloadBytes caps the JSON body of the upload handlers. Base64
// inflates a payload by ~4/3, so 8 MiB of body is about 6 MiB of decoded image:
// comfortably above any real document/selfie/frame and far below what could
// exhaust the process, since the encoded string, the decoded bytes, and the
// vendor request body would otherwise all be resident at once.
const maxUploadPayloadBytes = 8 << 20

// decodeUploadBody caps and decodes a JSON upload body. It returns false and
// writes a 413 if the body exceeds the limit, so the caller can return.
func decodeUploadBody(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadPayloadBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large",
				"The uploaded payload exceeds the size limit")
			return false
		}
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return false
	}
	return true
}

// verificationService bundles the vendor clients and the LinkedIn OAuth config
// so the handler has one collaborator instead of six.
type verificationService struct {
	linkedin  *oauth.Config
	idVendor  *verification.IDVerifierClient
	liveness  *verification.LivenessVerifierClient
	reverse   *verification.ReverseImageClient
	synthetic *verification.SyntheticImageClient
}

func newVerificationService(cfg *config.Config) *verificationService {
	livenessBase := cfg.LivenessVendorBaseURL
	if livenessBase == "" {
		livenessBase = cfg.IDVendorBaseURL
	}
	livenessKey := cfg.LivenessVendorAPIKey
	if livenessKey == "" {
		livenessKey = cfg.IDVendorAPIKey
	}

	return &verificationService{
		linkedin:  oauth.NewConfig(cfg.LinkedInClientID, cfg.LinkedInClientSecret, cfg.LinkedInRedirectURI),
		idVendor:  verification.NewIDVerifierClient(cfg.IDVendorBaseURL, cfg.IDVendorAPIKey, cfg.IDVendorName),
		liveness:  verification.NewLivenessVerifierClient(livenessBase, livenessKey),
		reverse:   verification.NewReverseImageClient(cfg.ReverseImageBaseURL, cfg.ReverseImageAPIKey),
		synthetic: verification.NewSyntheticImageClient(cfg.SyntheticBaseURL, cfg.SyntheticAPIKey),
	}
}

// LinkedInAuthorize starts the OAuth flow: it mints a single-use state, stores
// it, and returns the URL to redirect the user to. The state is bound to the
// subject so the callback cannot attach a LinkedIn identity to someone else.
func (h *VerificationHandler) LinkedInAuthorize(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if !h.cfg.LinkedInConfigured() {
		writeJSONError(w, http.StatusServiceUnavailable, "not_configured", "LinkedIn OAuth is not configured")
		return
	}

	csUserID := mux.Vars(r)["csUserId"]
	subjectID, ok := h.resolveSubject(ctx, csUserID)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown or missing subject")
		return
	}

	// Consent is required before we even begin collecting LinkedIn data.
	if err := h.consents.RequireConsent(ctx, subjectID, audit.PlaneB, models.ConsentTypeLinkedIn); err != nil {
		writeConsentError(w, err)
		return
	}

	state, err := oauth.NewState()
	if err != nil {
		h.logger.Error("generate oauth state failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to start OAuth flow")
		return
	}

	redirectURI := r.URL.Query().Get("redirectUri")
	if err := h.linkedin.CreateOAuthState(ctx, state, subjectID, "linkedin", redirectURI); err != nil {
		h.logger.Error("store oauth state failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to start OAuth flow")
		return
	}

	authURL, err := h.service.linkedin.AuthURL(state)
	if err != nil {
		h.logger.Error("build auth url failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to start OAuth flow")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"authorizationUrl": authURL,
		"state":            state,
	})
}

// LinkedInCallback completes the OAuth flow: it validates and burns the state,
// exchanges the code, fetches the profile, and stores it under the consent.
func (h *VerificationHandler) LinkedInCallback(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if !h.cfg.LinkedInConfigured() {
		writeJSONError(w, http.StatusServiceUnavailable, "not_configured", "LinkedIn OAuth is not configured")
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "state and code are required")
		return
	}

	subjectID, _, err := h.linkedin.ConsumeOAuthState(ctx, state, "linkedin")
	if err != nil {
		h.logger.Error("consume oauth state failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to complete OAuth flow")
		return
	}
	if subjectID == uuid.Nil {
		// Rejecting an unknown/reused state is the CSRF protection; it must not
		// be a soft failure.
		writeJSONError(w, http.StatusBadRequest, "invalid_state", "OAuth state is invalid, expired, or already used")
		return
	}

	if err := h.consents.RequireConsent(ctx, subjectID, audit.PlaneB, models.ConsentTypeLinkedIn); err != nil {
		writeConsentError(w, err)
		return
	}

	token, err := h.service.linkedin.ExchangeCode(ctx, code)
	if err != nil {
		h.logger.Warn("linkedin token exchange failed", zap.Error(err))
		writeJSONError(w, http.StatusBadGateway, "oauth_exchange_failed", "Failed to exchange authorization code")
		return
	}

	info, err := h.service.linkedin.FetchUserInfo(ctx, token.AccessToken)
	if err != nil {
		h.logger.Warn("linkedin userinfo failed", zap.Error(err))
		writeJSONError(w, http.StatusBadGateway, "oauth_profile_failed", "Failed to fetch LinkedIn profile")
		return
	}

	profile := &store.LinkedInProfile{
		SubjectID:        subjectID,
		LinkedInID:       info.Sub,
		Email:            info.Email,
		FirstName:        info.GivenName,
		LastName:         info.FamilyName,
		ProfileURL:       info.ProfileURL,
		Headline:         info.Headline,
		ConnectionsCount: info.Connections,
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		TokenExpiresAt:   token.ExpiresAt(),
		Scopes:           strings.Fields(token.Scope),
		RawProfile: map[string]interface{}{
			"locale":  info.Locale,
			"picture": info.Picture,
		},
	}

	if err := h.linkedin.UpsertProfile(ctx, profile, nil); err != nil {
		h.logger.Error("store linkedin profile failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to store LinkedIn profile")
		return
	}

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane:        audit.PlaneB,
		Action:       audit.ActionVerificationCompleted,
		Actor:        subjectID.String(),
		ActorType:    audit.ActorTypeUser,
		ResourceType: "linkedin_profile",
		SubjectID:    &subjectID,
		Details:      map[string]interface{}{"verificationType": models.VerificationTypeEmployment},
		Result:       "ok",
		RequestID:    r.Header.Get("X-Request-ID"),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"subjectId":  subjectID.String(),
		"linkedinId": profile.LinkedInID,
		"headline":   profile.Headline,
	})
}

// GovernmentIDVerify submits a document for identity verification. On success
// the derived date of birth is retained for the age gate and the subject is
// flagged as ID-verified.
func (h *VerificationHandler) GovernmentIDVerify(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if !h.cfg.IDVendorConfigured() {
		writeJSONError(w, http.StatusServiceUnavailable, "not_configured", "Government ID verification is not configured")
		return
	}

	subjectID, ok := h.resolveSubject(ctx, mux.Vars(r)["csUserId"])
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown or missing subject")
		return
	}
	if err := h.consents.RequireConsent(ctx, subjectID, audit.PlaneB, models.ConsentTypeGovernmentID); err != nil {
		writeConsentError(w, err)
		return
	}

	var req struct {
		DocumentImage string `json:"documentImage"`
		SelfieImage   string `json:"selfieImage"`
		DocumentType  string `json:"documentType"`
		CountryCode   string `json:"countryCode"`
	}
	if !decodeUploadBody(w, r, &req) {
		return
	}

	document, err := base64.StdEncoding.DecodeString(stripDataURI(req.DocumentImage))
	if err != nil || len(document) == 0 {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "documentImage must be base64-encoded")
		return
	}
	var selfie []byte
	if req.SelfieImage != "" {
		selfie, _ = base64.StdEncoding.DecodeString(stripDataURI(req.SelfieImage))
	}

	verificationID, err := h.verifs.CreateVerification(ctx, subjectID, nil, models.VerificationTypeGovernmentID, h.cfg.IDVendorName, models.VerificationStatusProcessing)
	if err != nil {
		h.logger.Error("create verification failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to start verification")
		return
	}

	result, err := h.service.idVendor.Verify(ctx, verification.IDVerificationRequest{
		DocumentImage: document,
		SelfieImage:   selfie,
		DocumentType:  req.DocumentType,
		CountryCode:   req.CountryCode,
	})
	if err != nil {
		h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, err.Error(), 0, nil)
		h.logger.Warn("government id verification failed", zap.Error(err))
		status := http.StatusBadGateway
		if errors.Is(err, verification.ErrVendorNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		writeJSONError(w, status, "verification_failed", "Government ID verification failed to run")
		return
	}

	details := map[string]interface{}{
		"vendorReference": result.VendorReference,
		"verifiedName":    result.VerifiedName,
		"countryCode":     result.CountryCode,
	}
	if result.DateOfBirth != nil {
		details["dateOfBirth"] = result.DateOfBirth.Format("2006-01-02")
	}

	if result.Status != models.VerificationStatusVerified {
		h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, result.FailureReason, result.CostUSD, details)
		h.auditor.Log(ctx, audit.AuditEvent{
			Plane: audit.PlaneB, Action: audit.ActionVerificationFailed, Actor: subjectID.String(),
			ActorType: audit.ActorTypeUser, ResourceType: "government_id", SubjectID: &subjectID,
			Details: details, Result: "failed", RequestID: r.Header.Get("X-Request-ID"),
		})
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":        models.VerificationStatusFailed,
			"failureReason": result.FailureReason,
		})
		return
	}

	// Persist the vendor row with the derived attributes. The document image
	// is not stored — only the reference and the attributes the product needs.
	if err := h.verifs.RecordGovernmentID(ctx, subjectID, result, h.cfg.IDVendorName); err != nil {
		h.logger.Error("record government id failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Verification succeeded but could not be saved")
		return
	}
	if err := h.verifs.MarkVerified(ctx, subjectID, models.VerificationTypeGovernmentID); err != nil {
		// The subject-level flag is what badges and capability gating read. A
		// verified government-ID row that is not reflected there would leave the
		// subject unverified despite this response, so surface the failure rather
		// than acknowledging a verification the rest of the system cannot see.
		h.logger.Error("mark id verified failed", zap.Error(err))
		h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, "could not record verified state", result.CostUSD, details)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Verification succeeded but could not be recorded")
		return
	}

	// A verified date of birth is authoritative in a way the self-reported one
	// is not, so the age gate is re-evaluated against it. A subject who passed
	// the gate on a self-reported DOB can still be found underage here; that
	// outcome is audited rather than returned as a verification failure, because
	// the identity check itself succeeded.
	if result.DateOfBirth != nil {
		age := policy.EvaluateAgeGate(result.DateOfBirth, time.Now())
		details["ageStatus"] = age.Status
		if age.Status == policy.AgeStatusUnderage {
			// Persist the restriction so it outlives this request and can be
			// enforced by capability gating; an audit row alone would not stop the
			// subject registering with a falsified or omitted DOB.
			if err := h.verifs.SetAgeBlock(ctx, subjectID, "government_id_verification"); err != nil {
				h.logger.Error("persist age block failed", zap.Error(err))
				h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, "could not record age restriction", result.CostUSD, details)
				writeJSONError(w, http.StatusInternalServerError, "internal_error", "Verification succeeded but the age restriction could not be recorded")
				return
			}
			h.auditor.Log(ctx, audit.AuditEvent{
				Plane: audit.PlaneB, Action: audit.ActionAgeGateBlocked, Actor: subjectID.String(),
				ActorType: audit.ActorTypeUser, ResourceType: "government_id", SubjectID: &subjectID,
				Details: map[string]interface{}{"source": "government_id_verification", "ageStatus": age.Status},
				Result:  "blocked", RequestID: r.Header.Get("X-Request-ID"),
			})
		}
	}

	h.completeVerification(ctx, verificationID, subjectID.String(), models.VerificationStatusVerified, "", result.CostUSD, details)

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane: audit.PlaneB, Action: audit.ActionVerificationCompleted, Actor: subjectID.String(),
		ActorType: audit.ActorTypeUser, ResourceType: "government_id", SubjectID: &subjectID,
		Details: details, Result: "ok", RequestID: r.Header.Get("X-Request-ID"),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         models.VerificationStatusVerified,
		"verifiedName":   result.VerifiedName,
		"hasDateOfBirth": result.DateOfBirth != nil,
		"ageStatus":      details["ageStatus"],
		"ageBlocked":     details["ageStatus"] == policy.AgeStatusUnderage,
	})
}

// LivenessVerify runs a proof-of-person check, optionally tying the result to
// an existing government-ID verification.
func (h *VerificationHandler) LivenessVerify(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	if !h.cfg.LivenessVendorConfigured() {
		writeJSONError(w, http.StatusServiceUnavailable, "not_configured", "Liveness verification is not configured")
		return
	}

	subjectID, ok := h.resolveSubject(ctx, mux.Vars(r)["csUserId"])
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown or missing subject")
		return
	}
	if err := h.consents.RequireConsent(ctx, subjectID, audit.PlaneB, models.ConsentTypeLiveness); err != nil {
		writeConsentError(w, err)
		return
	}

	var req struct {
		Frame                 string `json:"frame"`
		GovernmentIDReference string `json:"governmentIdReference"`
	}
	if !decodeUploadBody(w, r, &req) {
		return
	}
	frame, err := base64.StdEncoding.DecodeString(stripDataURI(req.Frame))
	if err != nil || len(frame) == 0 {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "frame must be base64-encoded")
		return
	}

	verificationID, err := h.verifs.CreateVerification(ctx, subjectID, nil, models.VerificationTypeLiveness, "liveness", models.VerificationStatusProcessing)
	if err != nil {
		h.logger.Error("create liveness verification failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to start verification")
		return
	}

	result, err := h.service.liveness.Verify(ctx, verification.LivenessRequest{
		VideoFrame:            frame,
		GovernmentIDReference: req.GovernmentIDReference,
	})
	if err != nil {
		h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, err.Error(), 0, nil)
		h.logger.Warn("liveness verification failed", zap.Error(err))
		writeJSONError(w, http.StatusBadGateway, "verification_failed", "Liveness verification failed to run")
		return
	}

	details := map[string]interface{}{
		"livenessScore":   result.LivenessScore,
		"vendorReference": result.VendorReference,
	}
	if result.MatchedIdentity != nil {
		details["matchedIdentity"] = *result.MatchedIdentity
	}

	if result.Status != models.VerificationStatusVerified {
		h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, result.FailureReason, result.CostUSD, details)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":        models.VerificationStatusFailed,
			"failureReason": result.FailureReason,
			"livenessScore": result.LivenessScore,
		})
		return
	}

	// A liveness pass with a caller-supplied government-ID reference is only
	// meaningful if the vendor confirms the live person is the document holder.
	// A high score alone must not yield has_liveness when the vendor explicitly
	// reported matched_identity: false.
	if livenessIdentityMismatch(req.GovernmentIDReference, result.MatchedIdentity) {
		const reason = "liveness succeeded but the live person did not match the supplied identity document"
		h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, reason, result.CostUSD, details)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":        models.VerificationStatusFailed,
			"failureReason": reason,
			"livenessScore": result.LivenessScore,
		})
		return
	}

	if err := h.verifs.RecordLiveness(ctx, subjectID, result); err != nil {
		h.logger.Error("record liveness failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Verification succeeded but could not be saved")
		return
	}
	if err := h.verifs.MarkVerified(ctx, subjectID, models.VerificationTypeLiveness); err != nil {
		// Same reasoning as the ID path: if the subject flag cannot be set, the
		// rest of the system will not see the verification, so do not report
		// success.
		h.logger.Error("mark liveness verified failed", zap.Error(err))
		h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, "could not record verified state", result.CostUSD, details)
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Verification succeeded but could not be recorded")
		return
	}
	h.completeVerification(ctx, verificationID, subjectID.String(), models.VerificationStatusVerified, "", result.CostUSD, details)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        models.VerificationStatusVerified,
		"livenessScore": result.LivenessScore,
	})
}

// ImageVerify runs reverse-image search and synthetic-image detection on a
// submitted image. The two checks are independent: a partial configuration
// still returns whichever results are available.
func (h *VerificationHandler) ImageVerify(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if !h.cfg.ReverseImageConfigured() && !h.cfg.SyntheticConfigured() {
		writeJSONError(w, http.StatusServiceUnavailable, "not_configured", "Image verification is not configured")
		return
	}

	subjectID, ok := h.resolveSubject(ctx, mux.Vars(r)["csUserId"])
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "Unknown or missing subject")
		return
	}
	if err := h.consents.RequireConsent(ctx, subjectID, audit.PlaneB, models.ConsentTypeImageVerification); err != nil {
		writeConsentError(w, err)
		return
	}

	var req struct {
		Image string `json:"image"`
	}
	if !decodeUploadBody(w, r, &req) {
		return
	}
	image, err := base64.StdEncoding.DecodeString(stripDataURI(req.Image))
	if err != nil || len(image) == 0 {
		writeJSONError(w, http.StatusBadRequest, "bad_request", "image must be base64-encoded")
		return
	}

	imageHash := sha256Hex(image)

	var reverseResult *verification.ReverseImageResult
	var syntheticResult *verification.SyntheticImageResult
	var reverseFailed, syntheticFailed bool

	if h.cfg.ReverseImageConfigured() {
		reverseResult, err = h.service.reverse.Search(ctx, image)
		if err != nil {
			// A provider failure is not evidence of a clean image. Record that the
			// check did not complete so the caller never sees a false clean result.
			h.logger.Warn("reverse image search failed", zap.Error(err))
			reverseFailed = true
		}
	}
	if h.cfg.SyntheticConfigured() {
		syntheticResult, err = h.service.synthetic.Detect(ctx, image)
		if err != nil {
			h.logger.Warn("synthetic image detection failed", zap.Error(err))
			syntheticFailed = true
		}
	}

	record := &store.ImageVerificationRecord{
		SubjectID:         subjectID,
		ImageHash:         imageHash,
		ReverseProvider:   providerOrEmpty(reverseResult),
		ReverseMatchCount: matchCount(reverseResult),
		ReverseMatches:    matches(reverseResult),
		SyntheticProvider: syntheticProvider(syntheticResult),
		SyntheticScore:    syntheticScore(syntheticResult),
		IsSynthetic:       syntheticResult != nil && syntheticResult.IsSynthetic,
	}

	reasonCodes := []string{}
	if record.IsSynthetic {
		reasonCodes = append(reasonCodes, models.ReasonCodeImageSynthetic)
	}
	if record.ReverseMatchCount > 0 {
		reasonCodes = append(reasonCodes, models.ReasonCodeImageReverseMatch)
	}

	// A failure only counts as "incomplete" when it could change the verdict. A
	// clean image requires every configured check to have completed, so a failed
	// reverse-image search cannot be reported as IMAGE_VERIFIED_CLEAN.
	incomplete := (reverseFailed && record.ReverseMatchCount == 0) || (syntheticFailed && !record.IsSynthetic)
	switch {
	case len(reasonCodes) > 0:
		// A positive finding stands on its own even if the other check failed.
		record.Status = models.VerificationStatusRejected
	case incomplete:
		reasonCodes = append(reasonCodes, models.ReasonCodeImageCheckIncomplete)
		// The check did not complete, so the stored row must not read as a
		// successful verification even though no adverse finding was recorded.
		record.Status = models.VerificationStatusFailed
	default:
		reasonCodes = append(reasonCodes, models.ReasonCodeImageClean)
		record.Status = models.VerificationStatusVerified
	}

	// The image flow is tracked in verification_token like the ID and liveness
	// flows, so every attempt (successful, adverse, or incomplete) appears in the
	// status API's verifications array.
	verificationID, err := h.verifs.CreateVerification(ctx, subjectID, nil, models.VerificationTypeImage, "image", models.VerificationStatusProcessing)
	if err != nil {
		h.logger.Error("create image verification failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to start verification")
		return
	}

	if err := h.verifs.RecordImageVerification(ctx, record); err != nil {
		h.completeVerification(ctx, verificationID, "", models.VerificationStatusFailed, "could not record image verification", 0, nil)
		h.logger.Error("record image verification failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Image verification could not be saved")
		return
	}

	resultDetails := map[string]interface{}{
		"imageHash":       imageHash,
		"isSynthetic":     record.IsSynthetic,
		"matchCount":      record.ReverseMatchCount,
		"reverseFailed":   reverseFailed,
		"syntheticFailed": syntheticFailed,
		"reasonCodes":     reasonCodes,
	}
	// Only the failed status carries the incomplete-check message; a rejected
	// image has an adverse finding and needs no "did not complete" note.
	errMessage := ""
	if record.Status == models.VerificationStatusFailed {
		errMessage = "one or more image checks did not complete"
	}
	h.completeVerification(ctx, verificationID, subjectID.String(), record.Status, errMessage, 0, resultDetails)

	h.auditor.Log(ctx, audit.AuditEvent{
		Plane: audit.PlaneB, Action: audit.ActionVerificationCompleted, Actor: subjectID.String(),
		ActorType: audit.ActorTypeUser, ResourceType: "image", SubjectID: &subjectID,
		Details: map[string]interface{}{
			"isSynthetic":     record.IsSynthetic,
			"matchCount":      record.ReverseMatchCount,
			"reverseFailed":   reverseFailed,
			"syntheticFailed": syntheticFailed,
		},
		Result: "ok", RequestID: r.Header.Get("X-Request-ID"),
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imageHash":         imageHash,
		"isSynthetic":       record.IsSynthetic,
		"reverseMatches":    record.ReverseMatchCount,
		"reverseComplete":   !reverseFailed,
		"syntheticComplete": !syntheticFailed,
		"reasonCodes":       reasonCodes,
	})
}

// completeVerification finalises the verification_token row, logging but not
// failing the request if the bookkeeping write fails (the vendor call already
// happened and its result is being returned).
func (h *VerificationHandler) completeVerification(
	ctx context.Context,
	verificationID uuid.UUID,
	subjectID string,
	status, errorMessage string,
	cost float64,
	result map[string]interface{},
) {
	if err := h.verifs.CompleteVerification(ctx, verificationID, status, errorMessage, cost, result); err != nil {
		h.logger.Error("complete verification bookkeeping failed", zap.Error(err))
	}
}

// stripDataURI removes a leading "data:image/...;base64," prefix so callers can
// post either a bare base64 string or a browser data URI.
func stripDataURI(s string) string {
	if idx := strings.Index(s, ","); strings.HasPrefix(s, "data:") && idx >= 0 {
		return s[idx+1:]
	}
	return s
}

func writeConsentError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, store.ErrConsentRequired) {
		status = http.StatusForbidden
	}
	writeJSONError(w, status, "consent_required", "An active consent is required for this verification")
}

// livenessIdentityMismatch reports whether a successful liveness check must be
// rejected because the vendor explicitly said the live person does not match the
// government-ID reference the caller supplied. An absent reference or an
// unreported match result is not a mismatch.
func livenessIdentityMismatch(governmentIDReference string, matchedIdentity *bool) bool {
	return governmentIDReference != "" && matchedIdentity != nil && !*matchedIdentity
}

func providerOrEmpty(r *verification.ReverseImageResult) string {
	if r == nil {
		return ""
	}
	return r.Provider
}

func matchCount(r *verification.ReverseImageResult) int {
	if r == nil {
		return 0
	}
	return len(r.Matches)
}

func matches(r *verification.ReverseImageResult) interface{} {
	if r == nil {
		return nil
	}
	return r.Matches
}

func syntheticProvider(r *verification.SyntheticImageResult) string {
	if r == nil {
		return ""
	}
	return r.Provider
}

func syntheticScore(r *verification.SyntheticImageResult) *float64 {
	if r == nil {
		return nil
	}
	return &r.Score
}
