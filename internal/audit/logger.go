package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// AuditLogger writes audit events to the database. It never returns errors
// to callers — audit failures are logged via zap but do not break business flow.
type AuditLogger struct {
	db     *store.PostgresDB
	logger *zap.Logger
}

// NewAuditLogger creates a new AuditLogger backed by the given database.
func NewAuditLogger(db *store.PostgresDB, zapLogger *zap.Logger) *AuditLogger {
	return &AuditLogger{
		db:     db,
		logger: zapLogger,
	}
}

// Log writes an audit event to the database. It NEVER returns an error to the
// caller — audit failures are logged via zap but don't break the business flow.
func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) {
	var detailsJSON []byte
	if event.Details != nil {
		var err error
		detailsJSON, err = json.Marshal(event.Details)
		if err != nil {
			a.logger.Error("failed to marshal audit event details",
				zap.String("action", event.Action),
				zap.Error(err),
			)
			// Fall through with nil details rather than dropping the event entirely.
			detailsJSON = nil
		}
	}

	// Use a 5-second timeout so audit writes don't block the caller indefinitely.
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO audit_log (
			plane, action, actor, actor_type,
			resource_type, resource_id, subject_id,
			details, result, error_message,
			request_id, ip_address
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	// Convert empty ip_address to nil so the INET column receives NULL
	// instead of an empty string that PostgreSQL would reject.
	var ipAddr interface{}
	if event.IPAddress != "" {
		ipAddr = event.IPAddress
	}

	_, err := a.db.ExecContext(writeCtx, query,
		event.Plane,
		event.Action,
		event.Actor,
		event.ActorType,
		event.ResourceType,
		event.ResourceID,
		event.SubjectID,
		detailsJSON,
		event.Result,
		event.ErrorMessage,
		event.RequestID,
		ipAddr,
	)
	if err != nil {
		a.logger.Error("failed to write audit log entry",
			zap.String("action", event.Action),
			zap.String("request_id", event.RequestID),
			zap.Error(err),
		)
	}
}

// LogAssessment is a convenience method for assessment-related audit events.
func (a *AuditLogger) LogAssessment(ctx context.Context, action string, assessmentID *uuid.UUID, subjectID uuid.UUID, details map[string]interface{}, requestID string) {
	a.Log(ctx, AuditEvent{
		Plane:        PlaneA,
		Action:       action,
		Actor:        "trustgraph-api",
		ActorType:    ActorTypeService,
		ResourceType: "assessment",
		ResourceID:   assessmentID,
		SubjectID:    &subjectID,
		Details:      details,
		Result:       "ok",
		RequestID:    requestID,
	})
}

// LogAssessmentError is a convenience method for failed assessment audit events.
func (a *AuditLogger) LogAssessmentError(ctx context.Context, action string, assessmentID *uuid.UUID, subjectID uuid.UUID, details map[string]interface{}, errorMsg string, requestID string) {
	a.Log(ctx, AuditEvent{
		Plane:        PlaneA,
		Action:       action,
		Actor:        "trustgraph-api",
		ActorType:    ActorTypeService,
		ResourceType: "assessment",
		ResourceID:   assessmentID,
		SubjectID:    &subjectID,
		Details:      details,
		Result:       "error",
		ErrorMessage: errorMsg,
		RequestID:    requestID,
	})
}

// LogSignal logs a signal evaluation event.
func (a *AuditLogger) LogSignal(ctx context.Context, providerName string, subjectID uuid.UUID, result string, details map[string]interface{}, requestID string) {
	a.Log(ctx, AuditEvent{
		Plane:        PlaneA,
		Action:       ActionSignalEvaluated,
		Actor:        providerName,
		ActorType:    ActorTypeService,
		ResourceType: "signal",
		SubjectID:    &subjectID,
		Details:      details,
		Result:       result,
		RequestID:    requestID,
	})
}
