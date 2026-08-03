package policy

import (
	"math"
	"sort"

	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

// Decision constants used by both the engine and hard-block rules.
const (
	decisionAccept = "accept"
	decisionVerify = "verify"
	decisionReview = "review"
	decisionDeny   = "deny"
)

// SignalResult is a local mirror of the signal-provider output.  It is defined
// here (rather than imported from the signals package) to avoid a circular
// dependency between policy and signals.
type SignalResult struct {
	Provider    string
	ReasonCodes []string
	Score       int     // 0-100, higher = riskier
	Confidence  float64 // 0-1
	Error       error
}

// PolicyResult is the final trust decision produced by the engine.
type PolicyResult struct {
	TrustTier       string   // provisional, standard, elevated, limited
	RiskBand        string   // low, elevated, high, unknown
	RiskScore       int      // 0-100 (higher = riskier)
	Decision        string   // accept, verify, review, deny
	ReasonCodes     []string // all aggregated reason codes
	RequiredActions []string // what the user should do next
	PolicyVersion   string   // e.g., "registration-v1"
}

// Engine evaluates a set of signal results against the current policy and
// produces a PolicyResult.
type Engine struct {
	logger *zap.Logger
	rules  []Rule
}

// NewEngine creates a policy engine with the default rule set.
func NewEngine(logger *zap.Logger) *Engine {
	rules := defaultRules()
	// Guarantee priority ordering even if defaultRules drifts.
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	return &Engine{
		logger: logger,
		rules:  rules,
	}
}

// Evaluate processes signal results and returns the trust assessment.
func (e *Engine) Evaluate(signals []SignalResult) *PolicyResult {
	result := &PolicyResult{
		PolicyVersion: CurrentPolicyVersion,
	}

	// ---- 1. Filter usable signals and aggregate reason codes ----
	var usable []SignalResult
	codeSet := make(map[string]struct{})

	for _, s := range signals {
		// Collect reason codes from every signal (including errored ones).
		for _, rc := range s.ReasonCodes {
			codeSet[rc] = struct{}{}
		}
		if s.Error != nil {
			e.logger.Warn("skipping errored signal in score computation",
				zap.String("provider", s.Provider),
				zap.Error(s.Error),
			)
			continue
		}
		usable = append(usable, s)
	}

	// Flatten code set into a stable sorted slice.
	allCodes := make([]string, 0, len(codeSet))
	for c := range codeSet {
		allCodes = append(allCodes, c)
	}
	sort.Strings(allCodes)
	result.ReasonCodes = allCodes

	// ---- 2. Compute weighted risk score ----
	result.RiskScore = computeWeightedScore(usable)
	if len(usable) == 0 {
		result.RiskBand = models.RiskBandUnknown
	}

	// ---- 3. Evaluate hard-block rules (first match wins) ----
	if matched := e.evaluateRules(codeSet); matched != nil {
		e.logger.Info("hard-block rule matched",
			zap.String("rule", matched.Name),
			zap.String("decision", matched.Decision),
		)
		result.Decision = matched.Decision
		result.TrustTier = matched.TrustTier
		result.RiskBand = riskBandForScore(result.RiskScore)
		result.RequiredActions = determineActions(result.TrustTier, codeSet)
		return result
	}

	// ---- 4. Score-based tier / band / decision mapping ----
	result.TrustTier, result.RiskBand, result.Decision = mapScoreToTierBandDecision(result.RiskScore)

	// ---- 5. Required actions ----
	result.RequiredActions = determineActions(result.TrustTier, codeSet)

	e.logger.Info("policy evaluation complete",
		zap.Int("riskScore", result.RiskScore),
		zap.String("trustTier", result.TrustTier),
		zap.String("decision", result.Decision),
		zap.Int("reasonCodes", len(result.ReasonCodes)),
	)

	return result
}

// computeWeightedScore returns sum(score*confidence) / sum(confidence),
// rounded to the nearest integer, for the provided (non-errored) signals.
func computeWeightedScore(signals []SignalResult) int {
	if len(signals) == 0 {
		return 0
	}

	var weightedSum float64
	var confidenceSum float64
	for _, s := range signals {
		weightedSum += float64(s.Score) * s.Confidence
		confidenceSum += s.Confidence
	}
	if confidenceSum == 0 {
		return 0
	}

	score := weightedSum / confidenceSum
	// Clamp to [0, 100].
	score = math.Max(0, math.Min(100, score))
	return int(math.Round(score))
}

// evaluateRules checks hard-block rules in priority order and returns the
// first matching rule, or nil if none match.
func (e *Engine) evaluateRules(codeSet map[string]struct{}) *Rule {
	for i := range e.rules {
		if matchRule(e.rules[i], codeSet) {
			return &e.rules[i]
		}
	}
	return nil
}

// mapScoreToTierBandDecision converts a 0-100 risk score into the
// corresponding trust tier, risk band, and decision.
func mapScoreToTierBandDecision(score int) (tier, band, decision string) {
	switch {
	case score <= 20:
		return models.TrustTierStandard, models.RiskBandLow, decisionAccept
	case score <= 40:
		return models.TrustTierProvisional, models.RiskBandLow, decisionAccept
	case score <= 60:
		return models.TrustTierProvisional, models.RiskBandElevated, decisionVerify
	case score <= 80:
		return models.TrustTierProvisional, models.RiskBandHigh, decisionReview
	default:
		return models.TrustTierLimited, models.RiskBandHigh, decisionDeny
	}
}

// riskBandForScore maps a score to a risk band without setting tier/decision.
// Used when a hard-block rule overrides the tier but the band should still
// reflect the numeric score.
func riskBandForScore(score int) string {
	switch {
	case score <= 40:
		return models.RiskBandLow
	case score <= 60:
		return models.RiskBandElevated
	default:
		return models.RiskBandHigh
	}
}

// determineActions builds the list of required next-step actions based on the
// assigned trust tier and the presence of specific reason codes.
func determineActions(tier string, codeSet map[string]struct{}) []string {
	var actions []string

	if _, ok := codeSet[models.ReasonCodeEmailNotVerified]; ok {
		actions = append(actions, models.RequiredActionVerifyEmail)
	}
	if _, ok := codeSet[models.ReasonCodePhoneNotVerified]; ok {
		actions = append(actions, models.RequiredActionVerifyPhone)
	}
	if tier == models.TrustTierLimited {
		actions = append(actions, models.RequiredActionReviewByHuman)
	}

	return actions
}
