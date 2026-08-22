package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

type ReviewRepository struct {
	db *PostgresDB
}

func NewReviewRepository(db *PostgresDB) *ReviewRepository {
	return &ReviewRepository{db: db}
}

func (r *ReviewRepository) CreateReview(ctx context.Context, assessmentID uuid.UUID, reviewerEmail string, outcome models.ReviewOutcome, notes string) (*models.AssessmentReview, error) {
	id := uuid.New()
	var createdAt time.Time
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO assessment_review (review_id, assessment_id, reviewer_email, outcome, notes)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING created_at`,
		id, assessmentID, reviewerEmail, string(outcome), notes,
	).Scan(&createdAt)
	if err != nil {
		return nil, fmt.Errorf("create review: %w", err)
	}
	return &models.AssessmentReview{
		ReviewID:      id.String(),
		AssessmentID:  assessmentID.String(),
		ReviewerEmail: reviewerEmail,
		Outcome:       outcome,
		Notes:         notes,
		CreatedAt:     createdAt,
	}, nil
}

func (r *ReviewRepository) GetByAssessmentID(ctx context.Context, assessmentID uuid.UUID) (*models.AssessmentReview, error) {
	var rev models.AssessmentReview
	var outcome string
	err := r.db.QueryRowContext(ctx,
		`SELECT review_id, assessment_id, reviewer_email, outcome, notes, created_at
		 FROM assessment_review WHERE assessment_id = $1`,
		assessmentID,
	).Scan(&rev.ReviewID, &rev.AssessmentID, &rev.ReviewerEmail, &outcome, &rev.Notes, &rev.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get review: %w", err)
	}
	rev.Outcome = models.ReviewOutcome(outcome)
	return &rev, nil
}
