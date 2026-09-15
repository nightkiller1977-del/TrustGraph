package models

import "time"

// Investigation case statuses.
const (
	CaseStatusOpen          = "open"
	CaseStatusInvestigating = "investigating"
	CaseStatusEscalated     = "escalated"
	CaseStatusResolved      = "resolved"
	CaseStatusClosed        = "closed"
)

// Investigation case priorities.
const (
	CasePriorityLow    = "low"
	CasePriorityNormal = "normal"
	CasePriorityHigh   = "high"
	CasePriorityUrgent = "urgent"
)

// InvestigationCase is a Plane C investigation record.
type InvestigationCase struct {
	CaseID      string     `json:"caseId"`
	CaseNumber  string     `json:"caseNumber"`
	SubjectID   *string    `json:"subjectId,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	TriggerType string     `json:"triggerType,omitempty"`
	TriggerRef  string     `json:"triggerRef,omitempty"`
	AssignedTo  string     `json:"assignedTo,omitempty"`
	CreatedBy   string     `json:"createdBy"`
	Findings    string     `json:"findings,omitempty"`
	Resolution  string     `json:"resolution,omitempty"`
	OpenedAt    time.Time  `json:"openedAt"`
	ClosedAt    *time.Time `json:"closedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`

	Notes []InvestigationNote `json:"notes,omitempty"`
}

// InvestigationNote is one entry in a case's running record.
type InvestigationNote struct {
	NoteID     string    `json:"noteId"`
	CaseID     string    `json:"caseId"`
	Author     string    `json:"author"`
	AuthorRole string    `json:"authorRole,omitempty"`
	NoteType   string    `json:"noteType"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

// InvestigationToolQuery records one OSINT tool invocation.
type InvestigationToolQuery struct {
	QueryID       string                 `json:"queryId"`
	CaseID        *string                `json:"caseId,omitempty"`
	SubjectID     *string                `json:"subjectId,omitempty"`
	ToolName      string                 `json:"toolName"`
	QueryParams   map[string]interface{} `json:"queryParams,omitempty"`
	ResultSummary map[string]interface{} `json:"resultSummary,omitempty"`
	ResultCount   int                    `json:"resultCount"`
	Status        string                 `json:"status"`
	ErrorMessage  string                 `json:"errorMessage,omitempty"`
	CostUSD       float64                `json:"costUsd"`
	DurationMS    int                    `json:"durationMs"`
	Actor         string                 `json:"actor"`
	ActorRole     string                 `json:"actorRole,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
}

// BreakGlassGrant is a time-boxed emergency elevation.
type BreakGlassGrant struct {
	GrantID       string     `json:"grantId"`
	Actor         string     `json:"actor"`
	ActorRole     string     `json:"actorRole"`
	SubjectID     *string    `json:"subjectId,omitempty"`
	CaseID        *string    `json:"caseId,omitempty"`
	Justification string     `json:"justification"`
	GrantedAt     time.Time  `json:"grantedAt"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	ReviewedBy    string     `json:"reviewedBy,omitempty"`
	ReviewedAt    *time.Time `json:"reviewedAt,omitempty"`
	ReviewOutcome string     `json:"reviewOutcome,omitempty"`
}

// CaseCreateRequest is the POST body for opening a case.
type CaseCreateRequest struct {
	SubjectID   string `json:"subjectId,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
	TriggerType string `json:"triggerType,omitempty"`
	TriggerRef  string `json:"triggerRef,omitempty"`
	AssignedTo  string `json:"assignedTo,omitempty"`
}

// CaseUpdateRequest is the PATCH body for a case.
type CaseUpdateRequest struct {
	Status     *string `json:"status,omitempty"`
	Priority   *string `json:"priority,omitempty"`
	AssignedTo *string `json:"assignedTo,omitempty"`
	Findings   *string `json:"findings,omitempty"`
	Resolution *string `json:"resolution,omitempty"`
}

// FindingValue values for a resolved case.
const (
	FindingConfirmedViolation = "confirmed_violation"
	FindingNoViolation        = "no_violation"
	FindingInconclusive       = "inconclusive"
)
