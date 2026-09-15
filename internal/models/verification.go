package models

import "time"

// Plane B consent types. One consent row is held per (subject, plane, type),
// matching the subject_consent unique constraint.
const (
	ConsentTypeLinkedIn          = "linkedin_oauth"
	ConsentTypeGovernmentID      = "government_id"
	ConsentTypeLiveness          = "liveness"
	ConsentTypeImageVerification = "image_verification"
)

// ConsentStatus values.
const (
	ConsentStatusGranted   = "granted"
	ConsentStatusWithdrawn = "withdrawn"
	ConsentStatusExpired   = "expired"
)

// VerificationType values mirror the roadmap's verification surface.
const (
	VerificationTypeEducation    = "education"
	VerificationTypeEmployment   = "employment"
	VerificationTypeGovernmentID = "government_id"
	VerificationTypeLiveness     = "liveness"
	VerificationTypeImage        = "image"
)

// VerificationStatus values for a verification_token row.
const (
	VerificationStatusPending    = "pending"
	VerificationStatusProcessing = "processing"
	VerificationStatusVerified   = "verified"
	VerificationStatusFailed     = "failed"
	VerificationStatusRejected   = "rejected"
	VerificationStatusDeleted    = "deleted"
)

// Badge states shown on a profile.
const (
	BadgeStateVerified     = "verified"
	BadgeStateSelfReported = "self_reported"
	BadgeStatePending      = "pending"
	BadgeStateNone         = "none"
)

// SubjectConsent records a subject's opt-in for one Plane B data collection
// purpose. Withdrawal is a soft state change (status + withdrawn_at) rather
// than a delete, so the audit trail of what was once consented survives.
type SubjectConsent struct {
	ConsentID     string                 `json:"consentId"`
	SubjectID     string                 `json:"subjectId"`
	Plane         string                 `json:"plane"`
	ConsentType   string                 `json:"consentType"`
	ConsentStatus string                 `json:"consentStatus"`
	PolicyVersion string                 `json:"policyVersion,omitempty"`
	TermsAccepted map[string]interface{} `json:"termsAccepted,omitempty"`
	GrantedAt     *time.Time             `json:"grantedAt,omitempty"`
	WithdrawnAt   *time.Time             `json:"withdrawnAt,omitempty"`
	ExpiresAt     *time.Time             `json:"expiresAt,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
}

// VerificationRecord is one attempt to verify a subject's attribute with a
// vendor or internal validator.
type VerificationRecord struct {
	VerificationID string                 `json:"verificationId"`
	SubjectID      string                 `json:"subjectId"`
	ConsentID      *string                `json:"consentId,omitempty"`
	Type           string                 `json:"verificationType"`
	Vendor         string                 `json:"vendor,omitempty"`
	Status         string                 `json:"status"`
	Result         map[string]interface{} `json:"result,omitempty"`
	ErrorMessage   string                 `json:"errorMessage,omitempty"`
	CostUSD        float64                `json:"costUsd"`
	CreatedAt      time.Time              `json:"createdAt"`
	CompletedAt    *time.Time             `json:"completedAt,omitempty"`
}

// Badge is the user-visible outcome of a verification, aggregated per type.
type Badge struct {
	Type      string     `json:"type"`
	State     string     `json:"state"`
	Label     string     `json:"label"`
	Detail    string     `json:"detail,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

// ConsentRequest is the POST body for granting or withdrawing a consent.
type ConsentRequest struct {
	ConsentType   string                 `json:"consentType"`
	PolicyVersion string                 `json:"policyVersion,omitempty"`
	TermsAccepted map[string]interface{} `json:"termsAccepted,omitempty"`
}

// VerificationStatusResponse summarises every verification type for a subject.
type VerificationStatusResponse struct {
	SubjectID     string               `json:"subjectId"`
	Consents      []SubjectConsent     `json:"consents"`
	Verifications []VerificationRecord `json:"verifications"`
}
