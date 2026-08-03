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
func NewEvaluator(logger *zap.Logger) *Evaluator {
	return &Evaluator{
		logger: logger,
		providers: []Provider{
			&EmailProvider{},
			&PhoneProvider{},
			&DeviceProvider{},
			&VelocityProvider{},
			&ImageProvider{},
		},
	}
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
