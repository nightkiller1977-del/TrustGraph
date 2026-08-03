package policy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(zaptest.NewLogger(t))
}

func TestEvaluate_AllVerified(t *testing.T) {
	engine := newTestEngine(t)

	signals := []SignalResult{
		{Provider: "email", ReasonCodes: []string{models.ReasonCodeEmailVerified}, Score: 0, Confidence: 0.9},
		{Provider: "phone", ReasonCodes: []string{models.ReasonCodePhoneVerified}, Score: 0, Confidence: 0.9},
	}

	result := engine.Evaluate(signals)

	assert.Equal(t, models.TrustTierStandard, result.TrustTier)
	assert.Equal(t, models.RiskBandLow, result.RiskBand)
	assert.Equal(t, 0, result.RiskScore)
	assert.Equal(t, "accept", result.Decision)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeEmailVerified)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodePhoneVerified)
	assert.Empty(t, result.RequiredActions)
	assert.Equal(t, CurrentPolicyVersion, result.PolicyVersion)
}

func TestEvaluate_NothingVerified(t *testing.T) {
	engine := newTestEngine(t)

	// Scores chosen so the weighted average lands in the 41-60 range
	// (provisional tier, elevated risk band).
	signals := []SignalResult{
		{Provider: "email", ReasonCodes: []string{models.ReasonCodeEmailNotVerified}, Score: 50, Confidence: 0.8},
		{Provider: "phone", ReasonCodes: []string{models.ReasonCodePhoneNotVerified}, Score: 50, Confidence: 0.8},
	}

	result := engine.Evaluate(signals)

	// Weighted: (50*0.8 + 50*0.8) / (0.8+0.8) = 50
	assert.Equal(t, 50, result.RiskScore)
	assert.Equal(t, models.TrustTierProvisional, result.TrustTier)
	assert.Equal(t, models.RiskBandElevated, result.RiskBand)
	assert.Equal(t, "verify", result.Decision)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeEmailNotVerified)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodePhoneNotVerified)
	// VERIFY_EMAIL and VERIFY_PHONE required when those codes are present.
	assert.Contains(t, result.RequiredActions, models.RequiredActionVerifyEmail)
	assert.Contains(t, result.RequiredActions, models.RequiredActionVerifyPhone)
}

func TestEvaluate_HardBlockFraudPattern(t *testing.T) {
	engine := newTestEngine(t)

	signals := []SignalResult{
		{Provider: "email", ReasonCodes: []string{models.ReasonCodeDisposableEmail}, Score: 25, Confidence: 0.7},
		{Provider: "velocity", ReasonCodes: []string{models.ReasonCodeHighRegistrationVelocity}, Score: 30, Confidence: 0.8},
	}

	result := engine.Evaluate(signals)

	assert.Equal(t, "deny", result.Decision)
	assert.Equal(t, models.TrustTierLimited, result.TrustTier)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDisposableEmail)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeHighRegistrationVelocity)
	// Limited tier always requires human review.
	assert.Contains(t, result.RequiredActions, models.RequiredActionReviewByHuman)
}

func TestEvaluate_HardBlockDeviceSharedEnforced(t *testing.T) {
	engine := newTestEngine(t)

	signals := []SignalResult{
		{Provider: "device", ReasonCodes: []string{models.ReasonCodeDeviceSharedWithEnforced}, Score: 40, Confidence: 0.9},
	}

	result := engine.Evaluate(signals)

	assert.Equal(t, "review", result.Decision)
	assert.Equal(t, models.TrustTierLimited, result.TrustTier)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDeviceSharedWithEnforced)
	assert.Contains(t, result.RequiredActions, models.RequiredActionReviewByHuman)
}

func TestEvaluate_HardBlockImageReuseVelocity(t *testing.T) {
	engine := newTestEngine(t)

	signals := []SignalResult{
		{Provider: "image", ReasonCodes: []string{models.ReasonCodeImageHashReused}, Score: 30, Confidence: 0.8},
		{Provider: "velocity", ReasonCodes: []string{models.ReasonCodeHighRegistrationVelocity}, Score: 35, Confidence: 0.8},
	}

	result := engine.Evaluate(signals)

	assert.Equal(t, "review", result.Decision)
	assert.Equal(t, models.TrustTierLimited, result.TrustTier)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeImageHashReused)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeHighRegistrationVelocity)
	assert.Contains(t, result.RequiredActions, models.RequiredActionReviewByHuman)
}

func TestEvaluate_WeightedScoreComputation(t *testing.T) {
	engine := newTestEngine(t)

	signals := []SignalResult{
		{Provider: "a", ReasonCodes: []string{"SIG_A"}, Score: 20, Confidence: 0.8},
		{Provider: "b", ReasonCodes: []string{"SIG_B"}, Score: 60, Confidence: 0.4},
	}

	result := engine.Evaluate(signals)

	// Weighted average: (20*0.8 + 60*0.4) / (0.8+0.4) = (16+24)/1.2 = 33.33 → 33
	assert.Equal(t, 33, result.RiskScore)
	// Score 33 ≤ 40 → provisional / low / accept
	assert.Equal(t, models.TrustTierProvisional, result.TrustTier)
	assert.Equal(t, models.RiskBandLow, result.RiskBand)
	assert.Equal(t, "accept", result.Decision)
}

func TestEvaluate_ErroredSignalsExcludedFromScore(t *testing.T) {
	engine := newTestEngine(t)

	signals := []SignalResult{
		{Provider: "email", ReasonCodes: []string{models.ReasonCodeEmailVerified}, Score: 0, Confidence: 0.9, Error: nil},
		{Provider: "device", ReasonCodes: []string{models.ReasonCodeDeviceFirstSeen}, Score: 50, Confidence: 0.8, Error: fmt.Errorf("timeout")},
	}

	result := engine.Evaluate(signals)

	// Only the non-errored signal contributes to the score: (0*0.9)/0.9 = 0.
	assert.Equal(t, 0, result.RiskScore)
	// Reason codes from both signals are still aggregated.
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeEmailVerified)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDeviceFirstSeen)
	assert.Equal(t, models.TrustTierStandard, result.TrustTier)
	assert.Equal(t, "accept", result.Decision)
}

func TestEvaluate_RequiredActions(t *testing.T) {
	engine := newTestEngine(t)

	// Score >80 maps to limited tier via the score-based path (no hard-block).
	signals := []SignalResult{
		{Provider: "email", ReasonCodes: []string{models.ReasonCodeEmailNotVerified}, Score: 85, Confidence: 1.0},
	}

	result := engine.Evaluate(signals)

	assert.Equal(t, models.TrustTierLimited, result.TrustTier)
	assert.Equal(t, "deny", result.Decision)
	assert.Contains(t, result.RequiredActions, models.RequiredActionVerifyEmail, "EMAIL_NOT_VERIFIED should trigger VERIFY_EMAIL")
	assert.Contains(t, result.RequiredActions, models.RequiredActionReviewByHuman, "limited tier should trigger REVIEW_BY_HUMAN")
}

func TestEvaluate_EmptySignals(t *testing.T) {
	engine := newTestEngine(t)

	result := engine.Evaluate(nil)

	assert.Equal(t, 0, result.RiskScore)
	assert.Equal(t, CurrentPolicyVersion, result.PolicyVersion)
	// With no usable signals the score is 0; mapScoreToTierBandDecision(0)
	// maps to standard/low/accept (the early "unknown" assignment on line 98
	// is overwritten by step 4).
	assert.Equal(t, models.TrustTierStandard, result.TrustTier)
	assert.Equal(t, models.RiskBandLow, result.RiskBand)
	assert.Equal(t, "accept", result.Decision)
	assert.Empty(t, result.RequiredActions)
}
