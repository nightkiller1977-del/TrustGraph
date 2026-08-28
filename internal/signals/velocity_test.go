package signals

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

const velocityCountQueryPattern = `SELECT COUNT\(DISTINCT o\.subject_id\)\s*FROM observation o\s*WHERE o\.observation_type = 'registration'\s*AND o\.source_data->>'ip_address' = \$1\s*AND o\.created_at > now\(\) - interval '1 hour'`

func TestVelocityProvider_EmptyIP_ReturnsZeroScoreLowConfidence(t *testing.T) {
	provider := &VelocityProvider{}
	evalCtx := &EvalContext{IPAddress: ""}

	// No IP means no DB lookup at all, so a nil db must be safe.
	result := provider.Evaluate(context.Background(), evalCtx, nil)

	assert.Equal(t, 0, result.Score)
	assert.InDelta(t, 0.3, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Empty(t, result.ReasonCodes)
}

func TestVelocityProvider_LowCount_ReturnsZeroScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &VelocityProvider{}
	evalCtx := loadEvalContext(t, "testdata/velocity_payload.json")

	rows := sqlmock.NewRows([]string{"count"}).AddRow(2)
	mock.ExpectQuery(velocityCountQueryPattern).
		WithArgs(evalCtx.IPAddress).
		WillReturnRows(rows)

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 0, result.Score)
	assert.InDelta(t, 0.85, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Empty(t, result.ReasonCodes)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVelocityProvider_ModerateCount_ReturnsMediumScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &VelocityProvider{}
	evalCtx := loadEvalContext(t, "testdata/velocity_payload.json")

	rows := sqlmock.NewRows([]string{"count"}).AddRow(4)
	mock.ExpectQuery(velocityCountQueryPattern).
		WithArgs(evalCtx.IPAddress).
		WillReturnRows(rows)

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 15, result.Score)
	assert.InDelta(t, 0.85, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeHighRegistrationVelocity)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVelocityProvider_HighCount_ReturnsHighScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &VelocityProvider{}
	evalCtx := loadEvalContext(t, "testdata/velocity_payload.json")

	rows := sqlmock.NewRows([]string{"count"}).AddRow(6)
	mock.ExpectQuery(velocityCountQueryPattern).
		WithArgs(evalCtx.IPAddress).
		WillReturnRows(rows)

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 35, result.Score)
	assert.InDelta(t, 0.85, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeHighRegistrationVelocity)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVelocityProvider_QueryErrorFallsBackToDefaultScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &VelocityProvider{}
	evalCtx := loadEvalContext(t, "testdata/velocity_payload.json")

	mock.ExpectQuery(velocityCountQueryPattern).
		WithArgs(evalCtx.IPAddress).
		WillReturnError(errors.New("connection reset"))

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 0, result.Score)
	assert.InDelta(t, 0.3, result.Confidence, 0.001)
	require.Error(t, result.Error)
	assert.Empty(t, result.ReasonCodes)
	assert.NoError(t, mock.ExpectationsWereMet())
}
