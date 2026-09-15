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
	// ActionAgeGateBlocked records an underage registration attempt. It is a
	// separate action (rather than just a reason code on the assessment) so
	// compliance can alert and report on it without scanning assessments.
	ActionAgeGateBlocked = "age_gate.blocked"
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

// Plane B verification actions.
const (
	ActionVerificationStarted     = "verification.started"
	ActionVerificationCompleted   = "verification.completed"
	ActionVerificationFailed      = "verification.failed"
	ActionVerificationDataDeleted = "verification.data.deleted"
)

// Plane C investigation actions.
const (
	ActionInvestigationCaseCreated = "investigation.case.created"
	ActionInvestigationCaseViewed  = "investigation.case.viewed"
	ActionInvestigationCaseUpdated = "investigation.case.updated"
	ActionInvestigationCaseClosed  = "investigation.case.closed"
	ActionInvestigationToolQueried = "investigation.tool.queried"
	ActionInvestigationActionTaken = "investigation.action.taken"
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
