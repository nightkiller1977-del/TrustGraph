package signals

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// loadEvalContext reads one of the fixture payloads under testdata/ into an
// EvalContext. Shared by device_test.go, image_test.go, and velocity_test.go.
func loadEvalContext(t *testing.T, path string) *EvalContext {
	t.Helper()

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var evalCtx EvalContext
	require.NoError(t, json.Unmarshal(raw, &evalCtx))
	return &evalCtx
}

// newMockPostgres builds a *store.PostgresDB backed by sqlmock.
func newMockPostgres(t *testing.T) (*store.PostgresDB, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return &store.PostgresDB{DB: db}, mock
}

const deviceFingerprintQueryPattern = `SELECT a\.trust_tier\s*FROM observation o\s*JOIN assessment a ON a\.subject_id = o\.subject_id\s*WHERE o\.observation_type = 'registration'\s*AND o\.source_data->>'device_fingerprint' = \$1\s*AND o\.subject_id != \$2::uuid\s*ORDER BY a\.created_at DESC\s*LIMIT 1`

func TestDeviceProvider_EmptyFingerprint_ReturnsLowConfidenceFirstSeen(t *testing.T) {
	provider := &DeviceProvider{}
	evalCtx := &EvalContext{DeviceFingerprint: ""}

	// No DB call should be made at all when there's no fingerprint to look
	// up, so a nil db must be safe here.
	result := provider.Evaluate(context.Background(), evalCtx, nil)

	assert.Equal(t, 5, result.Score)
	assert.InDelta(t, 0.5, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDeviceFirstSeen)
}

func TestDeviceProvider_FingerprintNotFoundInDB_ReturnsFirstSeen(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &DeviceProvider{}
	evalCtx := loadEvalContext(t, "testdata/device_payload.json")

	mock.ExpectQuery(deviceFingerprintQueryPattern).
		WithArgs(evalCtx.DeviceFingerprint, evalCtx.SubjectID).
		WillReturnError(sql.ErrNoRows)

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 0, result.Score)
	assert.InDelta(t, 0.8, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDeviceFirstSeen)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeviceProvider_FingerprintSharedWithLimitedTier_ReturnsHighRiskScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &DeviceProvider{}
	evalCtx := loadEvalContext(t, "testdata/device_payload.json")

	rows := sqlmock.NewRows([]string{"trust_tier"}).AddRow(models.TrustTierLimited)
	mock.ExpectQuery(deviceFingerprintQueryPattern).
		WithArgs(evalCtx.DeviceFingerprint, evalCtx.SubjectID).
		WillReturnRows(rows)

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 30, result.Score)
	assert.InDelta(t, 0.8, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDeviceSharedWithEnforced)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeviceProvider_FingerprintSharedWithStandardTier_ReturnsMediumRiskScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &DeviceProvider{}
	evalCtx := loadEvalContext(t, "testdata/device_payload.json")

	rows := sqlmock.NewRows([]string{"trust_tier"}).AddRow(models.TrustTierStandard)
	mock.ExpectQuery(deviceFingerprintQueryPattern).
		WithArgs(evalCtx.DeviceFingerprint, evalCtx.SubjectID).
		WillReturnRows(rows)

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 10, result.Score)
	assert.InDelta(t, 0.8, result.Confidence, 0.001)
	assert.Nil(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDeviceSharedWithStandard)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeviceProvider_QueryErrorFallsBackToDefaultScore(t *testing.T) {
	db, mock := newMockPostgres(t)
	provider := &DeviceProvider{}
	evalCtx := loadEvalContext(t, "testdata/device_payload.json")

	mock.ExpectQuery(deviceFingerprintQueryPattern).
		WithArgs(evalCtx.DeviceFingerprint, evalCtx.SubjectID).
		WillReturnError(errors.New("connection reset"))

	result := provider.Evaluate(context.Background(), evalCtx, db)

	assert.Equal(t, 5, result.Score)
	assert.InDelta(t, 0.5, result.Confidence, 0.001)
	require.Error(t, result.Error)
	assert.Contains(t, result.ReasonCodes, models.ReasonCodeDeviceFirstSeen)
	assert.NoError(t, mock.ExpectationsWereMet())
}
