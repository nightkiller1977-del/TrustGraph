package audit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// newMockLogger builds an AuditLogger backed by a sqlmock-driven database and
// an observed zap core so tests can assert both the SQL that was written and
// what (if anything) was logged about it.
func newMockLogger(t *testing.T) (*AuditLogger, sqlmock.Sqlmock, *observer.ObservedLogs) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)

	auditLogger := NewAuditLogger(&store.PostgresDB{DB: db}, logger)
	return auditLogger, mock, logs
}

// auditLogInsertPattern is the distinguishing fragment of the INSERT used by
// Log(). Using regexp.QuoteMeta-escaped literal fragments (rather than the
// full multi-line query) keeps the expectation robust to incidental
// whitespace/formatting changes while still requiring the right table/columns.
const auditLogInsertPattern = `INSERT INTO audit_log \(\s*plane, action, actor, actor_type,\s*resource_type, resource_id, subject_id,\s*details, result, error_message,\s*request_id, ip_address\s*\) VALUES \(\$1, \$2, \$3, \$4, \$5, \$6, \$7, \$8, \$9, \$10, \$11, \$12\)`

// sampleEventJSON mirrors the shape of testdata/sample_event.json. AuditEvent
// itself has no json tags (it is only ever constructed in Go, never
// deserialized), so this local type exists solely to load the fixture and
// prove it matches the real field shape.
type sampleEventJSON struct {
	Plane        string                 `json:"plane"`
	Action       string                 `json:"action"`
	Actor        string                 `json:"actor"`
	ActorType    string                 `json:"actorType"`
	ResourceType string                 `json:"resourceType"`
	ResourceID   string                 `json:"resourceId"`
	SubjectID    string                 `json:"subjectId"`
	Details      map[string]interface{} `json:"details"`
	Result       string                 `json:"result"`
	ErrorMessage string                 `json:"errorMessage"`
	RequestID    string                 `json:"requestId"`
	IPAddress    string                 `json:"ipAddress"`
}

func loadSampleEvent(t *testing.T) AuditEvent {
	t.Helper()

	raw, err := os.ReadFile("testdata/sample_event.json")
	require.NoError(t, err)

	var parsed sampleEventJSON
	require.NoError(t, json.Unmarshal(raw, &parsed))

	resourceID, err := uuid.Parse(parsed.ResourceID)
	require.NoError(t, err)
	subjectID, err := uuid.Parse(parsed.SubjectID)
	require.NoError(t, err)

	return AuditEvent{
		Plane:        parsed.Plane,
		Action:       parsed.Action,
		Actor:        parsed.Actor,
		ActorType:    parsed.ActorType,
		ResourceType: parsed.ResourceType,
		ResourceID:   &resourceID,
		SubjectID:    &subjectID,
		Details:      parsed.Details,
		Result:       parsed.Result,
		ErrorMessage: parsed.ErrorMessage,
		RequestID:    parsed.RequestID,
		IPAddress:    parsed.IPAddress,
	}
}

// TestLog_WritesAllRequiredColumns verifies Log() inserts exactly the 12
// columns defined by the audit_log schema (migrations/001_init_schema.sql),
// in the documented order, using a real (fixture-shaped) event.
func TestLog_WritesAllRequiredColumns(t *testing.T) {
	auditLogger, mock, _ := newMockLogger(t)
	event := loadSampleEvent(t)

	expectedDetails, err := json.Marshal(event.Details)
	require.NoError(t, err)

	mock.ExpectExec(auditLogInsertPattern).
		WithArgs(
			event.Plane,
			event.Action,
			event.Actor,
			event.ActorType,
			event.ResourceType,
			event.ResourceID,
			event.SubjectID,
			expectedDetails,
			event.Result,
			event.ErrorMessage,
			event.RequestID,
			event.IPAddress,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	auditLogger.Log(context.Background(), event)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLog_MarshalDetailsFailureFallsBackToNilDetailsNotDropped verifies that
// when event.Details cannot be JSON-marshaled (e.g. it contains a channel),
// Log() still writes the row -- with details set to nil -- instead of
// dropping the audit event entirely. Losing an audit event silently would be
// a compliance gap; falling back to nil details is the safer failure mode.
func TestLog_MarshalDetailsFailureFallsBackToNilDetailsNotDropped(t *testing.T) {
	auditLogger, mock, logs := newMockLogger(t)

	event := AuditEvent{
		Plane:  PlaneA,
		Action: ActionAssessmentCompleted,
		Actor:  "trustgraph-api",
		Details: map[string]interface{}{
			"unmarshalable": make(chan int), // channels cannot be JSON-marshaled
		},
		Result:    "ok",
		RequestID: "req-marshal-fail",
	}

	mock.ExpectExec(auditLogInsertPattern).
		WithArgs(
			event.Plane,
			event.Action,
			event.Actor,
			event.ActorType,
			event.ResourceType,
			event.ResourceID,
			event.SubjectID,
			[]byte(nil), // details falls back to a nil []byte, not dropped
			event.Result,
			event.ErrorMessage,
			event.RequestID,
			nil,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	auditLogger.Log(context.Background(), event)

	assert.NoError(t, mock.ExpectationsWereMet())

	// The marshal failure itself must be visible in structured logs even
	// though it doesn't block the write.
	marshalErrors := logs.FilterMessage("failed to marshal audit event details")
	assert.Equal(t, 1, marshalErrors.Len())
}

// TestLog_EmptyIPAddressBecomesNil verifies the empty-string-to-NULL
// conversion documented in Log(): an empty IPAddress must be sent as nil so
// Postgres's INET column receives NULL rather than rejecting an empty string.
func TestLog_EmptyIPAddressBecomesNil(t *testing.T) {
	auditLogger, mock, _ := newMockLogger(t)

	event := AuditEvent{
		Plane:     PlaneA,
		Action:    ActionSignalEvaluated,
		Actor:     "email",
		Result:    "ok",
		RequestID: "req-empty-ip",
		IPAddress: "", // must become nil, not ""
	}

	mock.ExpectExec(auditLogInsertPattern).
		WithArgs(
			event.Plane,
			event.Action,
			event.Actor,
			event.ActorType,
			event.ResourceType,
			event.ResourceID,
			event.SubjectID,
			[]byte(nil), // no Details set, so detailsJSON is never populated
			event.Result,
			event.ErrorMessage,
			event.RequestID,
			nil, // <- the assertion under test
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	auditLogger.Log(context.Background(), event)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLog_DBWriteFailureDoesNotPanicOrReturnError characterizes a deliberate
// design decision, not a bug: AuditLogger.Log has no error return value at
// all (see the doc comments on AuditLogger and Log in logger.go, both of
// which state audit writes are fire-and-forget so a logging failure never
// blocks the business flow that triggered the audit event). This test proves
// that guarantee holds even when the underlying DB write fails: the call
// must not panic, and the only observable effect of the failure is a
// structured zap error log.
func TestLog_DBWriteFailureDoesNotPanicOrReturnError(t *testing.T) {
	auditLogger, mock, logs := newMockLogger(t)

	event := AuditEvent{
		Plane:     PlaneA,
		Action:    ActionAssessmentFailed,
		Actor:     "trustgraph-api",
		Result:    "error",
		RequestID: "req-db-down",
	}

	mock.ExpectExec(auditLogInsertPattern).
		WillReturnError(errors.New("connection refused"))

	assert.NotPanics(t, func() {
		auditLogger.Log(context.Background(), event)
	})

	assert.NoError(t, mock.ExpectationsWereMet())

	writeErrors := logs.FilterMessage("failed to write audit log entry")
	assert.Equal(t, 1, writeErrors.Len())
}

// TestLogAssessment_SetsCorrectPlaneAndActor verifies the LogAssessment
// convenience wrapper populates the Plane-A / service-actor fields that
// every assessment audit event is expected to carry, and forwards the
// caller-supplied action/assessmentID/subjectID/details/requestID through
// unchanged.
func TestLogAssessment_SetsCorrectPlaneAndActor(t *testing.T) {
	auditLogger, mock, _ := newMockLogger(t)

	assessmentID := uuid.New()
	subjectID := uuid.New()
	details := map[string]interface{}{"trustTier": "standard"}
	expectedDetails, err := json.Marshal(details)
	require.NoError(t, err)

	mock.ExpectExec(auditLogInsertPattern).
		WithArgs(
			PlaneA,
			ActionAssessmentCompleted,
			"trustgraph-api",
			ActorTypeService,
			"assessment",
			&assessmentID,
			&subjectID,
			expectedDetails,
			"ok",
			"",
			"req-la-1",
			nil,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	auditLogger.LogAssessment(context.Background(), ActionAssessmentCompleted, &assessmentID, subjectID, details, "req-la-1")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLogSignal_ResultReflectsProviderError verifies LogSignal writes
// whatever result string the caller computed for a provider's outcome
// (assessment_handler.go derives this via statusFromError(sr.Error)) rather
// than deriving it independently -- i.e. a provider error must show up as
// result="error" in the audit trail, and success as result="ok".
func TestLogSignal_ResultReflectsProviderError(t *testing.T) {
	tests := []struct {
		name   string
		result string
	}{
		{name: "provider succeeded", result: "ok"},
		{name: "provider errored", result: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditLogger, mock, _ := newMockLogger(t)

			subjectID := uuid.New()
			details := map[string]interface{}{"score": 10}
			expectedDetails, err := json.Marshal(details)
			require.NoError(t, err)

			mock.ExpectExec(auditLogInsertPattern).
				WithArgs(
					PlaneA,
					ActionSignalEvaluated,
					"device",
					ActorTypeService,
					"signal",
					nil,
					&subjectID,
					expectedDetails,
					tt.result,
					"",
					"req-ls-1",
					nil,
				).
				WillReturnResult(sqlmock.NewResult(0, 1))

			auditLogger.LogSignal(context.Background(), "device", subjectID, tt.result, details, "req-ls-1")

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
