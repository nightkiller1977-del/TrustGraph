package models

import "time"

type ReviewOutcome string

const (
	ReviewOutcomeConfirmedAbuse ReviewOutcome = "confirmed_abuse"
	ReviewOutcomeLegitimate     ReviewOutcome = "legitimate"
	ReviewOutcomeInconclusive   ReviewOutcome = "inconclusive"
	ReviewOutcomeError          ReviewOutcome = "error"
)

type AppealOutcome string

const (
	AppealOutcomePending  AppealOutcome = "pending"
	AppealOutcomeApproved AppealOutcome = "approved"
	AppealOutcomeRejected AppealOutcome = "rejected"
)

// AssessmentReview is a human reviewer's ground-truth label on a past assessment.
type AssessmentReview struct {
	ReviewID      string
	AssessmentID  string
	ReviewerEmail string
	Outcome       ReviewOutcome
	Notes         string
	CreatedAt     time.Time
}

// AssessmentAppeal is a user-initiated challenge to a limited-tier outcome.
type AssessmentAppeal struct {
	AppealID      string
	AssessmentID  string
	UserMessage   string
	ReviewerEmail string
	Outcome       AppealOutcome
	CreatedAt     time.Time
	ReviewedAt    *time.Time
}

// QueueItem is a pending-review row returned by the admin queue endpoint.
type QueueItem struct {
	AssessmentID  string    `json:"assessmentId"`
	SubjectID     string    `json:"subjectId"`
	RiskScore     int       `json:"riskScore"`
	RiskBand      string    `json:"riskBand"`
	TrustTier     string    `json:"trustTier"`
	Decision      string    `json:"decision"`
	ReasonCodes   []string  `json:"reasonCodes"`
	CreatedAt     time.Time `json:"createdAt"`
}
