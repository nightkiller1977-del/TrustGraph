package signals

import (
	"context"
	"strings"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// disposableDomains is a hardcoded set of known disposable email providers.
// Phase 1 uses a small static list; future phases may query an external service.
var disposableDomains = map[string]struct{}{
	"mailinator.com":     {},
	"tempmail.com":       {},
	"guerrillamail.com":  {},
	"throwaway.email":    {},
	"yopmail.com":        {},
	"sharklasers.com":    {},
}

// EmailProvider evaluates email verification status and disposable domain usage.
type EmailProvider struct{}

func (p *EmailProvider) Name() string {
	return "email"
}

func (p *EmailProvider) Evaluate(_ context.Context, evalCtx *EvalContext, _ *store.PostgresDB) SignalResult {
	result := SignalResult{
		Provider: p.Name(),
	}

	// Check email verification status
	if evalCtx.EmailVerified {
		result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeEmailVerified)
		result.Score = 0
		result.Confidence = 0.9
	} else {
		result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeEmailNotVerified)
		result.Score = 15
		result.Confidence = 0.9
	}

	// Check for disposable email domain
	if evalCtx.Email != "" {
		domain := extractDomain(evalCtx.Email)
		if _, ok := disposableDomains[domain]; ok {
			result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeDisposableEmail)
			result.Score += 25
			result.Confidence = 0.7 // lower confidence for disposable detection
		}
	}

	return result
}

// extractDomain returns the lowercased domain portion of an email address.
func extractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(parts[1]))
}
