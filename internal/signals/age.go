package signals

import (
	"context"
	"time"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/policy"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// AgeGateProvider turns the minimum-age policy into an assessment signal.
//
// The policy engine can only act on reason codes, so the gate is expressed as
// a provider: an underage subject produces UNDERAGE_USER, which a priority-0
// hard-block rule converts into an unconditional deny. The provider holds no
// state, so it is safe to share across goroutines.
type AgeGateProvider struct {
	now func() time.Time
}

// NewAgeGateProvider returns an age-gate provider using the wall clock.
func NewAgeGateProvider() *AgeGateProvider {
	return &AgeGateProvider{now: time.Now}
}

func (p *AgeGateProvider) Name() string {
	return "age_gate"
}

func (p *AgeGateProvider) Evaluate(_ context.Context, evalCtx *EvalContext, _ *store.PostgresDB) SignalResult {
	result := SignalResult{Provider: p.Name()}

	switch policy.EvaluateAgeGate(evalCtx.DateOfBirth, p.now()).Status {
	case policy.AgeStatusUnderage:
		// Maximum risk weight: even without the hard-block rule this cannot be
		// outweighed by positive signals elsewhere.
		result.Score = 100
		result.Confidence = 1.0
		result.ReasonCodes = []string{models.ReasonCodeUnderageUser}
	case policy.AgeStatusInvalid:
		result.Score = 60
		result.Confidence = 1.0
		result.ReasonCodes = []string{models.ReasonCodeAgeInvalid}
	case policy.AgeStatusUnknown:
		// No date of birth is not evidence of anything. Score zero so this
		// signal cannot move the risk score either way; the reason code is
		// retained so reviewers can see age was never established.
		result.Score = 0
		result.Confidence = 0.0
		result.ReasonCodes = []string{models.ReasonCodeAgeUnknown}
	default:
		result.Score = 0
		result.Confidence = 0.5
		result.ReasonCodes = []string{models.ReasonCodeAgeVerified}
	}

	return result
}
