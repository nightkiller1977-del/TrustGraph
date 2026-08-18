package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

// AssessmentRepository handles database operations for assessments
type AssessmentRepository struct {
	db *PostgresDB
}

// NewAssessmentRepository creates a new assessment repository
func NewAssessmentRepository(db *PostgresDB) *AssessmentRepository {
	return &AssessmentRepository{db: db}
}

// CreateAssessment inserts a new assessment into the database
func (r *AssessmentRepository) CreateAssessment(ctx context.Context, assessment *models.Assessment) error {
	query := `
		INSERT INTO assessment (
			assessment_id, contract_version, idempotency_key, subject_id,
			assessment_type, trust_tier, risk_band, risk_score, decision,
			reason_codes, required_actions, policy_version, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING assessment_id
	`

	err := r.db.QueryRowContext(ctx, query,
		assessment.AssessmentID,
		assessment.ContractVersion,
		assessment.IdempotencyKey,
		assessment.SubjectID,
		assessment.AssessmentType,
		assessment.TrustTier,
		assessment.RiskBand,
		assessment.RiskScore,
		assessment.Decision,
		pq.Array(assessment.ReasonCodes),
		pq.Array(assessment.RequiredActions),
		assessment.PolicyVersion,
		assessment.Status,
		assessment.CreatedAt,
		assessment.UpdatedAt,
	).Scan(&assessment.AssessmentID)

	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to create assessment: %w", err)
	}

	return nil
}

// GetAssessmentByID retrieves an assessment by ID
func (r *AssessmentRepository) GetAssessmentByID(ctx context.Context, assessmentID uuid.UUID) (*models.Assessment, error) {
	query := `
		SELECT assessment_id, contract_version, idempotency_key, subject_id,
			   assessment_type, trust_tier, risk_band, risk_score, decision,
			   reason_codes, required_actions, policy_version, status,
			   created_at, updated_at, completed_at
		FROM assessment
		WHERE assessment_id = $1
	`

	var assessment models.Assessment
	var completedAt sql.NullTime
	var reasonCodes pq.StringArray
	var requiredActions pq.StringArray

	err := r.db.QueryRowContext(ctx, query, assessmentID).Scan(
		&assessment.AssessmentID,
		&assessment.ContractVersion,
		&assessment.IdempotencyKey,
		&assessment.SubjectID,
		&assessment.AssessmentType,
		&assessment.TrustTier,
		&assessment.RiskBand,
		&assessment.RiskScore,
		&assessment.Decision,
		&reasonCodes,
		&requiredActions,
		&assessment.PolicyVersion,
		&assessment.Status,
		&assessment.CreatedAt,
		&assessment.UpdatedAt,
		&completedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	assessment.ReasonCodes = []string(reasonCodes)
	assessment.RequiredActions = []string(requiredActions)
	if completedAt.Valid {
		assessment.CompletedAt = &completedAt.Time
	}

	return &assessment, nil
}

// queryRowContexter is satisfied by both *sql.DB (via *PostgresDB) and
// *sql.Tx, so the idempotency-key lookup can run either standalone or as
// part of CreateAssessmentIfAbsent's transaction without duplicating the
// query.
type queryRowContexter interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

const assessmentByIdempotencyKeyQuery = `
	SELECT assessment_id, contract_version, idempotency_key, subject_id,
		   assessment_type, trust_tier, risk_band, risk_score, decision,
		   reason_codes, required_actions, policy_version, status,
		   created_at, updated_at, completed_at
	FROM assessment
	WHERE idempotency_key = $1
	  AND ($2 = 0 OR created_at > now() - make_interval(hours := $2))
	ORDER BY created_at DESC
	LIMIT 1
`

func scanAssessmentByIdempotencyKey(ctx context.Context, q queryRowContexter, idempotencyKey string, ttlHours int) (*models.Assessment, error) {
	var assessment models.Assessment
	var completedAt sql.NullTime
	var reasonCodes pq.StringArray
	var requiredActions pq.StringArray

	err := q.QueryRowContext(ctx, assessmentByIdempotencyKeyQuery, idempotencyKey, ttlHours).Scan(
		&assessment.AssessmentID,
		&assessment.ContractVersion,
		&assessment.IdempotencyKey,
		&assessment.SubjectID,
		&assessment.AssessmentType,
		&assessment.TrustTier,
		&assessment.RiskBand,
		&assessment.RiskScore,
		&assessment.Decision,
		&reasonCodes,
		&requiredActions,
		&assessment.PolicyVersion,
		&assessment.Status,
		&assessment.CreatedAt,
		&assessment.UpdatedAt,
		&completedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get assessment by idempotency key: %w", err)
	}

	assessment.ReasonCodes = []string(reasonCodes)
	assessment.RequiredActions = []string(requiredActions)
	if completedAt.Valid {
		assessment.CompletedAt = &completedAt.Time
	}

	return &assessment, nil
}

// GetAssessmentByIdempotencyKey retrieves an assessment by idempotency key within the TTL window.
// ttlHours=0 disables the time filter and matches any entry.
func (r *AssessmentRepository) GetAssessmentByIdempotencyKey(ctx context.Context, idempotencyKey string, ttlHours int) (*models.Assessment, error) {
	return scanAssessmentByIdempotencyKey(ctx, r.db, idempotencyKey, ttlHours)
}

// CreateAssessmentIfAbsent atomically re-checks the idempotency TTL window
// and inserts only if nothing matched, closing the race
// GetAssessmentByIdempotencyKey + CreateAssessment leaves open: those are
// two independent statements, so two concurrent requests carrying the same
// NEW idempotency key can both pass the "not found" check before either
// inserts, creating duplicate assessments despite the documented
// idempotency guarantee.
//
// Uses a transaction-scoped Postgres advisory lock keyed by a hash of the
// idempotency key (auto-released on commit/rollback) rather than a
// database constraint — a true rolling-TTL-window uniqueness constraint
// isn't expressible as a static index (see migrations/001's removed
// idx_assessment_idempotency_active, which tried and fails because
// index predicates must be IMMUTABLE and now() is only STABLE), and a
// permanent UNIQUE constraint would break the documented TTL-based key
// reuse this repo relies on. The lock only needs to be held for this
// narrow re-check-and-insert — callers should still do their own upfront
// GetAssessmentByIdempotencyKey check before expensive work (signal
// evaluation, policy engine), since that's the common non-racing case and
// shouldn't pay the extra round trip or contend for the lock.
//
// Returns (existing, true, nil) if a concurrent request already won the
// race (existing.AssessmentID is the OTHER request's row, not the
// caller-supplied assessment), or (assessment, false, nil) once the
// caller-supplied assessment has been newly inserted.
func (r *AssessmentRepository) CreateAssessmentIfAbsent(ctx context.Context, assessment *models.Assessment, ttlHours int) (result *models.Assessment, alreadyExisted bool, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("begin idempotent-create tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// pg_advisory_xact_lock takes a bigint key; hashtext gives a stable
	// hash of the idempotency key so only requests sharing the same key
	// serialize against each other.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", assessment.IdempotencyKey); err != nil {
		return nil, false, fmt.Errorf("acquire idempotency lock: %w", err)
	}

	existing, err := scanAssessmentByIdempotencyKey(ctx, tx, assessment.IdempotencyKey, ttlHours)
	if err != nil {
		return nil, false, fmt.Errorf("idempotency re-check: %w", err)
	}
	if existing != nil {
		if err = tx.Commit(); err != nil {
			return nil, false, fmt.Errorf("commit idempotency re-check: %w", err)
		}
		return existing, true, nil
	}

	insertQuery := `
		INSERT INTO assessment (
			assessment_id, contract_version, idempotency_key, subject_id,
			assessment_type, trust_tier, risk_band, risk_score, decision,
			reason_codes, required_actions, policy_version, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING assessment_id
	`
	if err = tx.QueryRowContext(ctx, insertQuery,
		assessment.AssessmentID,
		assessment.ContractVersion,
		assessment.IdempotencyKey,
		assessment.SubjectID,
		assessment.AssessmentType,
		assessment.TrustTier,
		assessment.RiskBand,
		assessment.RiskScore,
		assessment.Decision,
		pq.Array(assessment.ReasonCodes),
		pq.Array(assessment.RequiredActions),
		assessment.PolicyVersion,
		assessment.Status,
		assessment.CreatedAt,
		assessment.UpdatedAt,
	).Scan(&assessment.AssessmentID); err != nil {
		return nil, false, fmt.Errorf("failed to create assessment: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit assessment insert: %w", err)
	}

	return assessment, false, nil
}

// UpdateAssessmentStatus updates the status and completion time of an assessment
func (r *AssessmentRepository) UpdateAssessmentStatus(ctx context.Context, assessmentID uuid.UUID, status string, trustTier string, reasonCodes []string) error {
	query := `
		UPDATE assessment
		SET status = $1, trust_tier = $2, reason_codes = $3, completed_at = $4, updated_at = $5
		WHERE assessment_id = $6
	`

	_, err := r.db.ExecContext(ctx, query,
		status,
		trustTier,
		pq.Array(reasonCodes),
		time.Now(),
		time.Now(),
		assessmentID,
	)

	if err != nil {
		return fmt.Errorf("failed to update assessment status: %w", err)
	}

	return nil
}
