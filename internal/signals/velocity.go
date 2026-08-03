package signals

import (
	"context"
	"fmt"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// VelocityProvider detects high registration velocity from the same IP address.
type VelocityProvider struct{}

func (p *VelocityProvider) Name() string {
	return "velocity"
}

func (p *VelocityProvider) Evaluate(ctx context.Context, evalCtx *EvalContext, db *store.PostgresDB) SignalResult {
	result := SignalResult{
		Provider:   p.Name(),
		Confidence: 0.85,
	}

	if evalCtx.IPAddress == "" {
		// No IP to evaluate -- nothing to flag
		result.Score = 0
		result.Confidence = 0.3
		return result
	}

	// Count distinct subjects that registered from this IP in the last hour.
	// We use the observation table with type 'ip_address' and look at the
	// source_data->>'ip' field within the past 60 minutes.
	query := `
		SELECT COUNT(DISTINCT o.subject_id)
		FROM observation o
		WHERE o.observation_type = 'ip_address'
		  AND o.source_data->>'ip' = $1
		  AND o.created_at > now() - interval '1 hour'
	`

	var count int
	err := db.QueryRowContext(ctx, query, evalCtx.IPAddress).Scan(&count)
	if err != nil {
		result.Error = fmt.Errorf("velocity check: %w", err)
		result.Score = 0
		result.Confidence = 0.3
		return result
	}

	switch {
	case count > 5:
		result.ReasonCodes = []string{models.ReasonCodeHighRegistrationVelocity}
		result.Score = 35
	case count > 3:
		result.ReasonCodes = []string{models.ReasonCodeHighRegistrationVelocity}
		result.Score = 15
	default:
		result.Score = 0
	}

	return result
}
