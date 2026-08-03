package signals

import (
	"context"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// PhoneProvider evaluates phone verification status.
type PhoneProvider struct{}

func (p *PhoneProvider) Name() string {
	return "phone"
}

func (p *PhoneProvider) Evaluate(_ context.Context, evalCtx *EvalContext, _ *store.PostgresDB) SignalResult {
	result := SignalResult{
		Provider:   p.Name(),
		Confidence: 0.9,
	}

	if evalCtx.PhoneVerified {
		result.ReasonCodes = []string{models.ReasonCodePhoneVerified}
		result.Score = 0
	} else {
		result.ReasonCodes = []string{models.ReasonCodePhoneNotVerified}
		result.Score = 10
	}

	return result
}
