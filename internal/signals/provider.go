package signals

import (
	"context"
	"time"

	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// SignalResult holds the outcome of a single signal provider evaluation.
type SignalResult struct {
	Provider    string
	ReasonCodes []string
	Score       int     // contribution to risk score (0-100), higher = riskier
	Confidence  float64 // 0.0-1.0, how confident is this signal
	Error       error   // nil if evaluation succeeded
}

// EvalContext contains everything a signal provider needs to evaluate a subject.
type EvalContext struct {
	SubjectID              string
	ConnectionSphereUserID string
	Email                  string
	Phone                  string
	EmailVerified          bool
	PhoneVerified          bool
	DeviceFingerprint      string
	DeviceToken            string
	IPAddress              string
	ImageHash              string
	UserAgent              string
	// DateOfBirth is the subject's date of birth when supplied. Nil means age
	// was not established and the age gate reports it as unknown.
	DateOfBirth *time.Time
	// AccountAgeHours is how long the subject's account has existed. Plane B
	// validators use it for timeline-plausibility checks.
	AccountAgeHours int
	// CurrentJobTitle is the subject's current role, when known. Plane B
	// education/employment validators use it for career-alignment checks.
	CurrentJobTitle string
}

// Provider evaluates a single signal dimension and returns a result.
type Provider interface {
	Name() string
	Evaluate(ctx context.Context, evalCtx *EvalContext, db *store.PostgresDB) SignalResult
}
