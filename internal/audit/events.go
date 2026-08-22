package audit

import "github.com/google/uuid"

// Action constants for audit events.
const (
	ActionAssessmentRequested = "assessment.requested"
	ActionAssessmentCompleted = "assessment.completed"
	ActionAssessmentFailed    = "assessment.failed"
	ActionAssessmentCached    = "assessment.cached"
	ActionSignalEvaluated     = "signal.evaluated"
	ActionSignalFailed        = "signal.failed"
	ActionSubjectCreated      = "subject.created"
	ActionSubjectUpdated      = "subject.updated"
	ActionConsentGranted      = "consent.granted"
	ActionConsentWithdrawn    = "consent.withdrawn"
)

// ActorType constants identify who performed an action.
const (
	ActorTypeService      = "service"
	ActorTypeUser         = "user"
	ActorTypeInvestigator = "investigator"
	ActorTypeSystem       = "system"
)

// EnforcementMode constants mark whether an assessment was acted upon.
const (
	EnforcementModeShadow   = "shadow"
	EnforcementModeEnforced = "enforced"
)

// Admin and appeal action constants.
const (
	ActionAdminQueueViewed     = "admin.queue.viewed"
	ActionAdminReviewSubmitted = "admin.review.submitted"
	ActionAppealSubmitted      = "appeal.submitted"
	ActionAppealReviewed       = "appeal.reviewed"
)

// Plane constants for the three-plane architecture.
const (
	PlaneA = "A"
	PlaneB = "B"
	PlaneC = "C"
)

// AuditEvent represents a single auditable action in the system.
type AuditEvent struct {
	Plane        string
	Action       string
	Actor        string
	ActorType    string
	ResourceType string
	ResourceID   *uuid.UUID
	SubjectID    *uuid.UUID
	Details      map[string]interface{}
	Result       string
	ErrorMessage string
	RequestID    string
	IPAddress    string
}
