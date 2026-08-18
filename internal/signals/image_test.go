package signals

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

const imageHashQueryPattern = `SELECT o\.subject_id\s*FROM observation o\s*WHERE o\.observation_type = 'registration'\s*AND o\.source_data->>'image_hash' = \$1\s*AND o\.subject_id != \$2::uuid\s*LIMIT 1`

func TestImageProvider_EmptyHash_ReturnsNewLowConfidence(t *testing.T) {
	provider := &ImageProvider{}
	evalCtx := &EvalContext{ImageHash: ""}

	// No image hash means no DB lookup at all, so a nil db must be safe.
	result := provider.Evaluate(context.Background(), evalCtx, nil)

	assert.Equal(t, 0, result.Score)
	assert.InDelta(t, 0.3, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeImageHashNew)
}

func TestImageProvider_HashNotFoundInDB_ReturnsNewHigherConfidence(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &ImageProvider{}
	evalCtx := loadEvalContext(t, "testdata/image_payload.json")

	mock.ExpectQuery(imageHashQueryPattern).
		WithArgs(evalCtx.ImageHash, evalCtx.SubjectID).
		WillReturnError(sql.ErrNoRows)

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 0, result.Score)
	assert.InDelta(t, 0.8, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeImageHashNew)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestImageProvider_HashReusedByAnotherSubject_ReturnsReusedScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &ImageProvider{}
	evalCtx := loadEvalContext(t, "testdata/image_payload.json")

	rows := sqlmock.NewRows([]string{"subject_id"}).AddRow("77777777-7777-7777-7777-777777777777")
	mock.ExpectQuery(imageHashQueryPattern).
		WithArgs(evalCtx.ImageHash, evalCtx.SubjectID).
		WillReturnRows(rows)

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 20, result.Score)
	assert.InDelta(t, 0.75, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeImageHashReused)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestImageProvider_QueryErrorFallsBackToDefaultScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &ImageProvider{}
	evalCtx := loadEvalContext(t, "testdata/image_payload.json")

	mock.ExpectQuery(imageHashQueryPattern).
		WithArgs(evalCtx.ImageHash, evalCtx.SubjectID).
		WillReturnError(errors.New("connection reset"))

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 0, result.Score)
	assert.InDelta(t, 0.3, result.Confidence, 0.001)
	require.Error(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeImageHashNew)
	assert.NoError(t, mock.ExpectationsWereMet())
}
