package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// newTestHandler builds an AssessmentHandler wired to a sqlmock-backed
// database, mirroring exactly how NewAssessmentHandler wires up the real
// repositories/evaluator/policy engine/auditor in production.
func newTestHandler(t *testing.T) (*AssessmentHandler, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{IdempotencyTTLHours: 24}
	handler := NewAssessmentHandler(&store.PostgresDB{DB: db}, zaptest.NewLogger(t), cfg)
	return handler, mock
}

func doCreateAssessment(t *testing.T, handler *AssessmentHandler, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/assessments", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	handler.CreateAssessment(rec, req)
	return rec
}

// Query/exec patterns are intentionally short, unique substrings (rather than
// the full hand-written SQL) so the tests stay robust to incidental
// whitespace/formatting changes while still pinning down which statement ran
// at each step, in order.
const (
	pIdempotencyLookup = `idempotency_key = \$1`
	pSubjectUpsert     = `INSERT INTO subject \(`
	pSubjectSelect     = `SELECT subject_id FROM subject`
	pAuditLogInsert    = `INSERT INTO audit_log \(`
	pAdvisoryLock      = `pg_advisory_xact_lock`
	pCreateAssessment  = `INSERT INTO assessment \(`
	pRecordObservation = `INSERT INTO observation \(`
	pSubjectFlags      = `SELECT has_government_id, has_liveness, verified_at, age_blocked, age_blocked_at FROM subject`
)

// expectNoSubjectFlags queues the persisted age-block lookup with no row, i.e.
// the subject carries no restriction. It runs after the per-signal audit writes
// and before the atomic insert transaction.
func expectNoSubjectFlags(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(pSubjectFlags).WillReturnError(sql.ErrNoRows)
}

func TestCreateAssessment_MissingRequiredFields_Returns400(t *testing.T) {
	handler, mock := newTestHandler(t)

	// Missing idempotencyKey -- validation must reject before touching the DB.
	rec := doCreateAssessment(t, handler, map[string]interface{}{
		"contractVersion": "v1",
		"subject": map[string]interface{}{
			"connectionSphereUserId": "cs-missing-idem",
		},
	})

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "bad_request", body["error"])

	// No DB interaction should have happened at all for a validation failure.
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAssessment_IdempotentReplay_ReturnsCachedResultWithoutReEvaluating(t *testing.T) {
	handler, mock := newTestHandler(t)

	cachedAssessmentID := uuid.New()
	cachedSubjectID := uuid.New()
	now := time.Now()

	cols := []string{
		"assessment_id", "contract_version", "idempotency_key", "subject_id",
		"assessment_type", "trust_tier", "risk_band", "risk_score", "decision",
		"reason_codes", "required_actions", "policy_version", "status",
		"created_at", "updated_at", "completed_at",
	}
	cachedRow := sqlmock.NewRows(cols).AddRow(
		cachedAssessmentID.String(), "v1", "idem-replay-1", cachedSubjectID.String(),
		"registration", models.TrustTierStandard, models.RiskBandLow, 5, "accept",
		"{EMAIL_VERIFIED}", "{}", "registration-v1", models.AssessmentStatusComplete,
		now, now, now,
	)

	mock.ExpectQuery(pIdempotencyLookup).WillReturnRows(cachedRow)
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))

	// Signals fields below (device/IP/image) would each trigger their own DB
	// lookup if the evaluator actually ran. No expectations are queued for
	// those queries, so if the idempotent-replay early return were ever
	// removed, this test would fail (either on an unmet/unexpected-query
	// error surfacing as a 500, or on the response no longer matching the
	// cached assessment) instead of silently passing.
	rec := doCreateAssessment(t, handler, map[string]interface{}{
		"contractVersion": "v1",
		"idempotencyKey":  "idem-replay-1",
		"subject": map[string]interface{}{
			"connectionSphereUserId": "cs-replay-1",
			"email":                  "user@example.com",
		},
		"signals": map[string]interface{}{
			"deviceFingerprint": "fp-should-not-be-queried",
			"ipAddress":         "203.0.113.99",
			"imageHash":         "hash-should-not-be-queried",
		},
	})

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp models.AssessmentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, cachedAssessmentID.String(), resp.AssessmentID)
	assert.Equal(t, models.TrustTierStandard, resp.TrustTier)
	assert.Equal(t, "accept", resp.Decision)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAssessment_HappyPath_ReturnsPolicyDecision(t *testing.T) {
	handler, mock := newTestHandler(t)

	newSubjectID := uuid.New()
	newAssessmentID := uuid.New()

	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(pSubjectUpsert).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(pSubjectSelect).
		WillReturnRows(sqlmock.NewRows([]string{"subject_id"}).AddRow(newSubjectID.String()))
	// assessment.requested
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	// one signal.evaluated audit write per provider (age_gate, email, phone,
	// device, velocity, image) -- see internal/signals/evaluator.go's fixed
	// provider list and internal/api/assessment_handler.go's per-result audit
	// loop.
	for i := 0; i < 6; i++ {
		mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	expectNoSubjectFlags(mock)
	// CreateAssessmentIfAbsent: begin tx, take the advisory lock, re-check
	// the idempotency window under the lock (still nothing there), insert,
	// commit.
	mock.ExpectBegin()
	mock.ExpectExec(pAdvisoryLock).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(pCreateAssessment).
		WillReturnRows(sqlmock.NewRows([]string{"assessment_id"}).AddRow(newAssessmentID.String()))
	mock.ExpectCommit()
	mock.ExpectExec(pRecordObservation).WillReturnResult(sqlmock.NewResult(0, 1))
	// assessment.completed
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))

	// No device fingerprint / IP / image hash, so device/velocity/image
	// providers take their "no input" early-return branch and never touch
	// the DB themselves (only the handler's own audit-log write per
	// provider result happens, which is accounted for above).
	rec := doCreateAssessment(t, handler, map[string]interface{}{
		"contractVersion": "v1",
		"idempotencyKey":  "idem-happy-1",
		"subject": map[string]interface{}{
			"connectionSphereUserId": "cs-happy-1",
			"email":                  "user@example.com",
		},
		"signals": map[string]interface{}{
			"emailVerified": true,
			"phoneVerified": true,
		},
	})

	require.Equal(t, http.StatusOK, rec.Code)

	var resp models.AssessmentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, newAssessmentID.String(), resp.AssessmentID)
	// Weighted score with everything verified and no device/IP/image signals
	// lands well under the accept threshold (see policy/engine.go
	// mapScoreToTierBandDecision: score <= 20 -> standard/low/accept).
	assert.Equal(t, models.TrustTierStandard, resp.TrustTier)
	assert.Equal(t, models.RiskBandLow, resp.RiskBand)
	assert.Equal(t, "accept", resp.Decision)
	assert.ElementsMatch(t, []string{"age_gate", "email", "phone", "device", "velocity", "image"}, resp.Signals.Processed)
	assert.Empty(t, resp.Signals.Skipped)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateAssessment_UnderageSubject_IsDeniedAndAudited drives the age gate
// through the real handler, evaluator, and policy engine (only the database is
// mocked) and asserts the end-to-end outcome: a hard deny, the BLOCK_ACCOUNT
// action, and a dedicated compliance audit event.
func TestCreateAssessment_UnderageSubject_IsDeniedAndAudited(t *testing.T) {
	handler, mock := newTestHandler(t)

	newSubjectID := uuid.New()
	newAssessmentID := uuid.New()

	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(pSubjectUpsert).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(pSubjectSelect).
		WillReturnRows(sqlmock.NewRows([]string{"subject_id"}).AddRow(newSubjectID.String()))
	// assessment.requested
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	// one signal.evaluated audit write per provider (six providers)
	for i := 0; i < 6; i++ {
		mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	expectNoSubjectFlags(mock)
	// age_gate.blocked compliance event, written after the policy engine runs
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	// CreateAssessmentIfAbsent transaction
	mock.ExpectBegin()
	mock.ExpectExec(pAdvisoryLock).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(pCreateAssessment).
		WillReturnRows(sqlmock.NewRows([]string{"assessment_id"}).AddRow(newAssessmentID.String()))
	mock.ExpectCommit()
	mock.ExpectExec(pRecordObservation).WillReturnResult(sqlmock.NewResult(0, 1))
	// assessment.completed
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := doCreateAssessment(t, handler, map[string]interface{}{
		"contractVersion": "v1",
		"idempotencyKey":  "idem-underage-1",
		"subject": map[string]interface{}{
			"connectionSphereUserId": "cs-underage-1",
			"email":                  "user@example.com",
		},
		"signals": map[string]interface{}{
			"emailVerified": true,
			"phoneVerified": true,
			// A date comfortably inside the minor range for any plausible
			// runtime year, so the test does not age out.
			"dateOfBirth": time.Now().AddDate(-13, 0, 0).Format("2006-01-02"),
		},
	})

	require.Equal(t, http.StatusOK, rec.Code)

	var resp models.AssessmentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "deny", resp.Decision)
	assert.Equal(t, models.TrustTierLimited, resp.TrustTier)
	assert.Contains(t, resp.ReasonCodes, models.ReasonCodeUnderageUser)
	assert.Contains(t, resp.RequiredActions, models.RequiredActionBlockAccount)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateAssessment_SubjectDateOfBirthIsHonoured pins the OpenAPI contract
// shape: dateOfBirth is documented on the subject, so an underage value there
// must reach the age gate. Reading only signals.dateOfBirth would let a
// documented request pass as AGE_UNKNOWN.
func TestCreateAssessment_SubjectDateOfBirthIsHonoured(t *testing.T) {
	handler, mock := newTestHandler(t)

	newSubjectID := uuid.New()
	newAssessmentID := uuid.New()

	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(pSubjectUpsert).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(pSubjectSelect).
		WillReturnRows(sqlmock.NewRows([]string{"subject_id"}).AddRow(newSubjectID.String()))
	// assessment.requested
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	for i := 0; i < 6; i++ {
		mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	expectNoSubjectFlags(mock)
	mock.ExpectBegin()
	mock.ExpectExec(pAdvisoryLock).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(pCreateAssessment).
		WillReturnRows(sqlmock.NewRows([]string{"assessment_id"}).AddRow(newAssessmentID.String()))
	mock.ExpectCommit()
	mock.ExpectExec(pRecordObservation).WillReturnResult(sqlmock.NewResult(0, 1))
	// assessment.completed
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := doCreateAssessment(t, handler, map[string]interface{}{
		"contractVersion": "v1",
		"idempotencyKey":  "idem-subject-dob",
		"subject": map[string]interface{}{
			"connectionSphereUserId": "cs-subject-dob",
			"email":                  "user@example.com",
			"dateOfBirth":            time.Now().AddDate(-13, 0, 0).Format("2006-01-02"),
		},
		"signals": map[string]interface{}{
			"emailVerified": true,
			"phoneVerified": true,
		},
	})

	require.Equal(t, http.StatusOK, rec.Code)

	var resp models.AssessmentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "deny", resp.Decision, "a documented subject.dateOfBirth must reach the age gate")
	assert.Contains(t, resp.ReasonCodes, models.ReasonCodeUnderageUser)
	assert.Contains(t, resp.RequiredActions, models.RequiredActionBlockAccount)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateAssessment_PersistedAgeBlock_ForcesDenyWithoutDOB guards the fix
// that a subject already flagged as underage cannot escape the restriction by
// submitting a later assessment with no dateOfBirth. Without the persisted flag
// lookup the request would evaluate as AGE_UNKNOWN and be accepted.
func TestCreateAssessment_PersistedAgeBlock_ForcesDenyWithoutDOB(t *testing.T) {
	handler, mock := newTestHandler(t)

	newSubjectID := uuid.New()
	newAssessmentID := uuid.New()

	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(pSubjectUpsert).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(pSubjectSelect).
		WillReturnRows(sqlmock.NewRows([]string{"subject_id"}).AddRow(newSubjectID.String()))
	// assessment.requested
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	for i := 0; i < 6; i++ {
		mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	// The persisted restriction: age_blocked is set even though this request
	// carries no dateOfBirth.
	mock.ExpectQuery(pSubjectFlags).WillReturnRows(
		sqlmock.NewRows([]string{"has_government_id", "has_liveness", "verified_at", "age_blocked", "age_blocked_at"}).
			AddRow(true, true, time.Now(), true, time.Now()))
	// age_gate.blocked compliance event
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectExec(pAdvisoryLock).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(pCreateAssessment).
		WillReturnRows(sqlmock.NewRows([]string{"assessment_id"}).AddRow(newAssessmentID.String()))
	mock.ExpectCommit()
	mock.ExpectExec(pRecordObservation).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := doCreateAssessment(t, handler, map[string]interface{}{
		"contractVersion": "v1",
		"idempotencyKey":  "idem-persisted-age-block",
		"subject": map[string]interface{}{
			"connectionSphereUserId": "cs-persisted-age-block",
			"email":                  "user@example.com",
		},
	})

	require.Equal(t, http.StatusOK, rec.Code)

	var resp models.AssessmentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "deny", resp.Decision, "a persisted age block must deny even without a dateOfBirth")
	assert.Equal(t, models.TrustTierLimited, resp.TrustTier)
	assert.Contains(t, resp.ReasonCodes, models.ReasonCodeUnderageUser)
	assert.Contains(t, resp.RequiredActions, models.RequiredActionBlockAccount)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateAssessment_MalformedDateOfBirth_Returns400 pins the decision to
// reject unparseable dates rather than silently dropping them, which would let
// a caller bypass the age gate with a bad payload.
func TestCreateAssessment_MalformedDateOfBirth_Returns400(t *testing.T) {
	handler, mock := newTestHandler(t)

	rec := doCreateAssessment(t, handler, map[string]interface{}{
		"contractVersion": "v1",
		"idempotencyKey":  "idem-bad-dob",
		"subject": map[string]interface{}{
			"connectionSphereUserId": "cs-bad-dob",
		},
		"signals": map[string]interface{}{
			"dateOfBirth": "17/05/1990-ish",
		},
	})

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Rejected before any database work, same as other validation failures.
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateAssessment_PersistenceFailure_LogsAssessmentFailedAudit(t *testing.T) {
	handler, mock := newTestHandler(t)

	newSubjectID := uuid.New()

	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(pSubjectUpsert).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(pSubjectSelect).
		WillReturnRows(sqlmock.NewRows([]string{"subject_id"}).AddRow(newSubjectID.String()))
	// assessment.requested
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	for i := 0; i < 6; i++ {
		mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	expectNoSubjectFlags(mock)
	// Persistence fails inside CreateAssessmentIfAbsent's transaction...
	mock.ExpectBegin()
	mock.ExpectExec(pAdvisoryLock).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(pIdempotencyLookup).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(pCreateAssessment).WillReturnError(sql.ErrConnDone)
	// ...the deferred rollback fires since err != nil...
	mock.ExpectRollback()
	// ...and the handler must still write an assessment.failed audit event
	// (this is the ExecContext queued below; if the handler ever stopped
	// calling LogAssessmentError on this path, mock.ExpectationsWereMet()
	// would fail because this expectation would go unmet).
	mock.ExpectExec(pAuditLogInsert).WillReturnResult(sqlmock.NewResult(0, 1))

	rec := doCreateAssessment(t, handler, map[string]interface{}{
		"contractVersion": "v1",
		"idempotencyKey":  "idem-fail-1",
		"subject": map[string]interface{}{
			"connectionSphereUserId": "cs-fail-1",
			"email":                  "user@example.com",
		},
		"signals": map[string]interface{}{
			"emailVerified": true,
			"phoneVerified": true,
		},
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}
