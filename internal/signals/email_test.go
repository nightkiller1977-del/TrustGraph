package signals

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

func TestEmailProvider_Verified(t *testing.T) {
	provider := &EmailProvider{}
	evalCtx := &EvalContext{
		Email:         "user@example.com",
		EmailVerified: true,
	}

	result := provider.Evaluate(context.Background(), evalCtx, nil)

	assert.Equal(t, 0, result.Score)
	assert.Nil(t, result.Error)
	assert.Equal(t, "email", result.Provider)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeEmailVerified)
	assert.NotContains(t, result.ReasonCodes, models.ReasonCodeEmailNotVerified)
}

func TestEmailProvider_NotVerified(t *testing.T) {
	provider := &EmailProvider{}
	evalCtx := &EvalContext{
		Email:         "user@example.com",
		EmailVerified: false,
	}

	result := provider.Evaluate(context.Background(), evalCtx, nil)

	assert.Equal(t, 15, result.Score)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeEmailNotVerified)
	assert.NotContains(t, result.ReasonCodes, models.ReasonCodeEmailVerified)
}

func TestEmailProvider_DisposableEmail(t *testing.T) {
	provider := &EmailProvider{}
	evalCtx := &EvalContext{
		Email:         "user@mailinator.com",
		EmailVerified: true, // verified so base score is 0; disposable adds 25
	}

	result := provider.Evaluate(context.Background(), evalCtx, nil)

	assert.Equal(t, 25, result.Score)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDisposableEmail)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeEmailVerified)
	// Disposable detection lowers confidence from 0.9 to 0.7.
	assert.InDelta(t, 0.7, result.Confidence, 0.001)
}

func TestEmailProvider_DisposableAndNotVerified(t *testing.T) {
	provider := &EmailProvider{}
	evalCtx := &EvalContext{
		Email:         "user@mailinator.com",
		EmailVerified: false,
	}

	result := provider.Evaluate(context.Background(), evalCtx, nil)

	// 15 (not verified) + 25 (disposable) = 40
	assert.Equal(t, 40, result.Score)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeEmailNotVerified)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDisposableEmail)
	assert.InDelta(t, 0.7, result.Confidence, 0.001)
}

func TestEmailProvider_NormalDomain(t *testing.T) {
	provider := &EmailProvider{}
	evalCtx := &EvalContext{
		Email:         "user@gmail.com",
		EmailVerified: true,
	}

	result := provider.Evaluate(context.Background(), evalCtx, nil)

	assert.Equal(t, 0, result.Score)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeEmailVerified)
	assert.NotContains(t, result.ReasonCodes, models.ReasonCodeDisposableEmail)
	assert.InDelta(t, 0.9, result.Confidence, 0.001)
}
