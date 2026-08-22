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

// GetAssessmentByIdempotencyKey retrieves an assessment by idempotency key within the TTL window.
// ttlHours=0 disables the time filter and matches any entry.
func (r *AssessmentRepository) GetAssessmentByIdempotencyKey(ctx context.Context, idempotencyKey string, ttlHours int) (*models.Assessment, error) {
	query := `
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

	var assessment models.Assessment
	var completedAt sql.NullTime
	var reasonCodes pq.StringArray
	var requiredActions pq.StringArray

	err := r.db.QueryRowContext(ctx, query, idempotencyKey, ttlHours).Scan(
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

// ListPendingReview returns assessments that have not yet been reviewed, optionally
// filtered by risk_band.  Results are ordered newest-first and capped at perPage.
func (r *AssessmentRepository) ListPendingReview(ctx context.Context, riskBand string, page, perPage int) ([]models.QueueItem, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	query := `
		SELECT a.assessment_id, a.subject_id, a.risk_score, a.risk_band, a.trust_tier, a.decision, a.reason_codes, a.created_at
		FROM assessment a
		LEFT JOIN assessment_review r ON a.assessment_id = r.assessment_id
		WHERE r.review_id IS NULL
		  AND ($1 = '' OR a.risk_band = $1)
		ORDER BY a.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, riskBand, perPage, offset)
	if err != nil {
		return nil, fmt.Errorf("list pending review: %w", err)
	}
	defer rows.Close()

	var items []models.QueueItem
	for rows.Next() {
		var item models.QueueItem
		var codes pq.StringArray
		if err := rows.Scan(&item.AssessmentID, &item.SubjectID, &item.RiskScore, &item.RiskBand, &item.TrustTier, &item.Decision, &codes, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan queue item: %w", err)
		}
		item.ReasonCodes = []string(codes)
		items = append(items, item)
	}
	return items, nil
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
