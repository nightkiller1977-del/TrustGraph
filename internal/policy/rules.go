package policy

import "github.com/nightkiller1977-del/trustgraph/internal/models"

// Rule is a hard-block policy rule that overrides the score-based tier mapping
// when every code in RequiredCodes is present in the aggregated signal reasons.
type Rule struct {
	Name          string   // human-readable identifier
	RequiredCodes []string // all must be present for the rule to fire
	Decision      string   // decision to impose when matched
	TrustTier     string   // trust tier to impose when matched
	Priority      int      // lower value = higher priority; first match wins
}

// defaultRules returns the hard-block rules for the current policy version,
// pre-sorted by ascending priority (highest-priority first).
func defaultRules() []Rule {
	return []Rule{
		{
			Name:          "fraud-pattern-velocity-disposable",
			RequiredCodes: []string{models.ReasonCodeDisposableEmail, models.ReasonCodeHighRegistrationVelocity},
			Decision:      decisionDeny,
			TrustTier:     models.TrustTierLimited,
			Priority:      1,
		},
		{
			Name:          "device-shared-enforced",
			RequiredCodes: []string{models.ReasonCodeDeviceSharedWithEnforced},
			Decision:      decisionReview,
			TrustTier:     models.TrustTierLimited,
			Priority:      2,
		},
		{
			Name:          "image-reuse-velocity",
			RequiredCodes: []string{models.ReasonCodeImageHashReused, models.ReasonCodeHighRegistrationVelocity},
			Decision:      decisionReview,
			TrustTier:     models.TrustTierLimited,
			Priority:      3,
		},
	}
}

// matchRule returns true when every code in r.RequiredCodes exists in the
// provided reason-code set.
func matchRule(r Rule, codeSet map[string]struct{}) bool {
	for _, code := range r.RequiredCodes {
		if _, ok := codeSet[code]; !ok {
			return false
		}
	}
	return true
}
