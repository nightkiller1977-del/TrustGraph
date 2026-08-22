package models

import (
	"time"

	"github.com/google/uuid"
)

// AssessmentRequest is the incoming request for a trust assessment
type AssessmentRequest struct {
	ContractVersion string            `json:"contractVersion"`
	AssessmentID    *string           `json:"assessmentId,omitempty"`
	IdempotencyKey  string            `json:"idempotencyKey"`
	Subject         SubjectData       `json:"subject"`
	Signals         SignalsData       `json:"signals,omitempty"`
	RequestContext  *RequestContext   `json:"requestContext,omitempty"`
}

// SubjectData identifies the person being assessed
type SubjectData struct {
	ConnectionSphereUserID string `json:"connectionSphereUserId"`
	Email                  string `json:"email,omitempty"`
	Phone                  string `json:"phone,omitempty"`
}

// SignalsData contains first-party signals available at registration
type SignalsData struct {
	EmailVerified       bool   `json:"emailVerified,omitempty"`
	PhoneVerified       bool   `json:"phoneVerified,omitempty"`
	DeviceToken         string `json:"deviceToken,omitempty"`
	ImageHash           string `json:"imageHash,omitempty"`
	DeviceFingerprint   string `json:"deviceFingerprint,omitempty"`
	IPAddress           string `json:"ipAddress,omitempty"`
}

// RequestContext provides tracing information
type RequestContext struct {
	UserAgent     string `json:"userAgent,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`
}

type AssessmentResponse struct {
	ContractVersion string           `json:"contractVersion"`
	AssessmentID    string           `json:"assessmentId"`
	Status          string           `json:"status"`
	TrustTier       string           `json:"trustTier"`
	RiskBand        string           `json:"riskBand"`
	RiskScore       int              `json:"riskScore"`
	Decision        string           `json:"decision"`
	ReasonCodes     []string         `json:"reasonCodes"`
	RequiredActions []string         `json:"requiredActions"`
	PolicyVersion   string           `json:"policyVersion"`
	CompletedAt     *time.Time       `json:"completedAt,omitempty"`
	Signals         SignalsProcessed `json:"signals,omitempty"`
	EnforcementMode string           `json:"enforcementMode,omitempty"`
}

// SignalsProcessed indicates which signals were evaluated
type SignalsProcessed struct {
	Processed []string `json:"processed"`
	Skipped   []string `json:"skipped"`
}

// Assessment is the internal database model
type Assessment struct {
	AssessmentID      uuid.UUID
	ContractVersion   string
	IdempotencyKey    string
	SubjectID         uuid.UUID
	AssessmentType    string
	TrustTier         string
	RiskBand          string
	RiskScore         int
	Decision          string
	ReasonCodes       []string
	RequiredActions   []string
	PolicyVersion     string
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CompletedAt       *time.Time
}

// TrustTier constants
const (
	TrustTierProvisional = "provisional"
	TrustTierStandard    = "standard"
	TrustTierElevated    = "elevated"
	TrustTierLimited     = "limited"
)

// RiskBand constants
const (
	RiskBandLow       = "low"
	RiskBandElevated  = "elevated"
	RiskBandHigh      = "high"
	RiskBandUnknown   = "unknown"
)

// AssessmentStatus constants
const (
	AssessmentStatusPending  = "pending"
	AssessmentStatusComplete = "complete"
	AssessmentStatusError    = "error"
	AssessmentStatusDeferred = "deferred"
)

// RequiredAction constants
const (
	RequiredActionVerifyEmail = "VERIFY_EMAIL"
	RequiredActionVerifyPhone = "VERIFY_PHONE"
	RequiredActionProvideID   = "PROVIDE_ID"
	RequiredActionReviewByHuman = "REVIEW_BY_HUMAN"
)

// ReasonCode constants
const (
	ReasonCodeEmailVerified              = "EMAIL_VERIFIED"
	ReasonCodeEmailNotVerified           = "EMAIL_NOT_VERIFIED"
	ReasonCodePhoneVerified              = "PHONE_VERIFIED"
	ReasonCodePhoneNotVerified           = "PHONE_NOT_VERIFIED"
	ReasonCodeDeviceFirstSeen            = "DEVICE_FIRST_SEEN"
	ReasonCodeDeviceSharedWithStandard   = "DEVICE_SHARED_WITH_STANDARD"
	ReasonCodeDeviceSharedWithEnforced   = "DEVICE_SHARED_WITH_ENFORCED"
	ReasonCodeImageHashNew               = "IMAGE_HASH_NEW"
	ReasonCodeImageHashReused            = "IMAGE_HASH_REUSED"
	ReasonCodeHighRegistrationVelocity   = "HIGH_REGISTRATION_VELOCITY"
	ReasonCodeDisposableEmail            = "DISPOSABLE_EMAIL"
	ReasonCodeVerificationPending        = "VERIFICATION_PENDING"
	ReasonCodeAssessmentDeferred         = "ASSESSMENT_DEFERRED"
	ReasonCodeAssessmentUnavailable      = "ASSESSMENT_UNAVAILABLE"

	// Plane B: Education verification reason codes
	ReasonCodeEducationVerified          = "EDUCATION_VERIFIED"
	ReasonCodeEducationSelfReported      = "EDUCATION_SELF_REPORTED"
	ReasonCodeEducationTimelinePlausible = "EDUCATION_TIMELINE_PLAUSIBLE"
	ReasonCodeEducationKnownUniversity   = "EDUCATION_KNOWN_UNIVERSITY"
	ReasonCodeEducationCareerAligned     = "EDUCATION_CAREER_ALIGNED"
	ReasonCodeEducationRecentGraduate    = "EDUCATION_RECENT_GRADUATE"
	ReasonCodeEducationGPADisclosed      = "EDUCATION_GPA_DISCLOSED"
)
