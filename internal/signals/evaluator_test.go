package signals

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// fakeProvider is a test double implementing the Provider interface so
// evaluator behavior (ordering, error isolation) can be verified without
// depending on the real providers' scoring logic or a database.
type fakeProvider struct {
	name   string
	result SignalResult
}

func (p *fakeProvider) Name() string { return p.name }

func (p *fakeProvider) Evaluate(_ context.Context, _ *EvalContext, _ *store.PostgresDB) SignalResult {
	result := p.result
	result.Provider = p.name
	return result
}

func newTestEvaluator(t *testing.T, providers ...Provider) *Evaluator {
	t.Helper()
	return &Evaluator{
		logger:    zaptest.NewLogger(t),
		providers: providers,
	}
}

// TestEvaluateAll_ReturnsResultForEveryProvider verifies the real,
// production-registered provider set (NewEvaluator's fixed list of five
// providers) all produce a result. A nil db is safe here because none of the
// fields set below trigger a DB lookup in device/velocity/image (see their
// respective "empty input" early-return branches).
func TestEvaluateAll_ReturnsResultForEveryProvider(t *testing.T) {
	evaluator := NewEvaluator(zaptest.NewLogger(t))
	evalCtx := &EvalContext{
		SubjectID:     "11111111-1111-1111-1111-111111111111",
		Email:         "user@example.com",
		EmailVerified: true,
		PhoneVerified: true,
	}

	results := evaluator.EvaluateAll(context.Background(), evalCtx, nil)

	assert.Len(t, results, 5)
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Provider
		assert.Nil(t, r.Error, "provider %s should not error", r.Provider)
	}
	assert.Equal(t, []string{"email", "phone", "device", "velocity", "image"}, names)
}

// TestEvaluateAll_OneProviderErrorDoesNotAbortOthers injects a fake provider
// that fails between two that succeed, and asserts EvaluateAll still returns
// a result for every provider (including the failing one, with Error set)
// rather than stopping the fan-out early.
func TestEvaluateAll_OneProviderErrorDoesNotAbortOthers(t *testing.T) {
	boom := errors.New("boom")
	evaluator := newTestEvaluator(t,
		&fakeProvider{name: "p1", result: SignalResult{Score: 10, Confidence: 0.9}},
		&fakeProvider{name: "p2", result: SignalResult{Error: boom}},
		&fakeProvider{name: "p3", result: SignalResult{Score: 20, Confidence: 0.5}},
	)

	results := evaluator.EvaluateAll(context.Background(), &EvalContext{}, nil)

	assert.Len(t, results, 3)

	assert.Equal(t, "p1", results[0].Provider)
	assert.Nil(t, results[0].Error)
	assert.Equal(t, 10, results[0].Score)

	assert.Equal(t, "p2", results[1].Provider)
	assert.Equal(t, boom, results[1].Error)

	// The provider after the failing one must still have run and populated
	// its own result -- proof the loop didn't abort on p2's error.
	assert.Equal(t, "p3", results[2].Provider)
	assert.Nil(t, results[2].Error)
	assert.Equal(t, 20, results[2].Score)
}

// TestEvaluateAll_PreservesProviderOrder verifies EvaluateAll returns results
// in the same order the providers were registered. evaluator.go performs a
// simple sequential range over e.providers with no sorting or reordering, so
// registration order is the only ordering guarantee that exists -- this test
// pins that guarantee down explicitly.
func TestEvaluateAll_PreservesProviderOrder(t *testing.T) {
	evaluator := newTestEvaluator(t,
		&fakeProvider{name: "zzz"},
		&fakeProvider{name: "aaa"},
		&fakeProvider{name: "mmm"},
	)

	results := evaluator.EvaluateAll(context.Background(), &EvalContext{}, nil)

	assert.Equal(t, []string{"zzz", "aaa", "mmm"}, []string{
		results[0].Provider, results[1].Provider, results[2].Provider,
	})
}
