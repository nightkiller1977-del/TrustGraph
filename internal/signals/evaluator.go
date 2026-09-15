package signals

import (
	"context"

	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// Evaluator runs all registered signal providers and collects results.
type Evaluator struct {
	providers []Provider
	logger    *zap.Logger
}

// NewEvaluator creates an Evaluator with the standard Plane A signal providers.
// The age gate runs first: it is the only provider that can produce a
// hard-blocking deny, and keeping it at the head of the list makes the
// ordering explicit for any future provider that wants to short-circuit.
func NewEvaluator(logger *zap.Logger) *Evaluator {
	return &Evaluator{
		logger: logger,
		providers: []Provider{
			NewAgeGateProvider(),
			&EmailProvider{},
			&PhoneProvider{},
			&DeviceProvider{},
			&VelocityProvider{},
			&ImageProvider{},
		},
	}
}

// NewEvaluatorWithProviders creates an Evaluator from an explicit provider list.
// Tests and callers that need Plane B providers (education, employment) use
// this instead of the fixed Plane A set.
func NewEvaluatorWithProviders(logger *zap.Logger, providers ...Provider) *Evaluator {
	return &Evaluator{logger: logger, providers: providers}
}

// NewPlaneBEvaluator builds the evaluator used for re-assessment once a subject
// has consented Plane B data. It is deliberately separate from the
// registration-time evaluator: at registration Plane B data cannot exist yet.
// Providers that find nothing to evaluate return an error, which EvaluateAll
// surfaces as a skipped signal rather than a fabricated score.
func NewPlaneBEvaluator(logger *zap.Logger, eduRepo *store.EducationRepository) *Evaluator {
	return NewEvaluatorWithProviders(logger,
		NewEducationProvider(eduRepo),
	)
}

// Providers returns the registered providers in evaluation order.
func (e *Evaluator) Providers() []Provider {
	return e.providers
}

// EvaluateAll runs every registered provider and returns all results.
// Failed providers are included in the results with the Error field set.
func (e *Evaluator) EvaluateAll(ctx context.Context, evalCtx *EvalContext, db *store.PostgresDB) []SignalResult {
	results := make([]SignalResult, 0, len(e.providers))

	for _, p := range e.providers {
		result := p.Evaluate(ctx, evalCtx, db)

		if result.Error != nil {
			e.logger.Warn("signal provider failed",
				zap.String("provider", p.Name()),
				zap.Error(result.Error),
			)
		} else {
			e.logger.Debug("signal provider evaluated",
				zap.String("provider", p.Name()),
				zap.Int("score", result.Score),
				zap.Float64("confidence", result.Confidence),
				zap.Strings("reason_codes", result.ReasonCodes),
			)
		}

		results = append(results, result)
	}

	return results
}
