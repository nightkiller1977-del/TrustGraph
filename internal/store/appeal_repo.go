package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

type AppealRepository struct {
	db *PostgresDB
}

func NewAppealRepository(db *PostgresDB) *AppealRepository {
	return &AppealRepository{db: db}
}

func (r *AppealRepository) CreateAppeal(ctx context.Context, assessmentID uuid.UUID, userMessage string) (*models.AssessmentAppeal, error) {
	id := uuid.New()
	var createdAt time.Time
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO assessment_appeal (appeal_id, assessment_id, user_message)
		 VALUES ($1, $2, $3)
		 RETURNING created_at`,
		id, assessmentID, userMessage,
	).Scan(&createdAt)
	if err != nil {
		return nil, fmt.Errorf("create appeal: %w", err)
	}
	return &models.AssessmentAppeal{
		AppealID:     id.String(),
		AssessmentID: assessmentID.String(),
		UserMessage:  userMessage,
		Outcome:      models.AppealOutcomePending,
		CreatedAt:    createdAt,
	}, nil
}

func (r *AppealRepository) GetByAssessmentID(ctx context.Context, assessmentID uuid.UUID) (*models.AssessmentAppeal, error) {
	var ap models.AssessmentAppeal
	var outcome string
	var reviewerEmail sql.NullString
	var reviewedAt sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT appeal_id, assessment_id, user_message, reviewer_email, outcome, created_at, reviewed_at
		 FROM assessment_appeal WHERE assessment_id = $1`,
		assessmentID,
	).Scan(&ap.AppealID, &ap.AssessmentID, &ap.UserMessage, &reviewerEmail, &outcome, &ap.CreatedAt, &reviewedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get appeal: %w", err)
	}
	ap.Outcome = models.AppealOutcome(outcome)
	if reviewerEmail.Valid {
		ap.ReviewerEmail = reviewerEmail.String
	}
	if reviewedAt.Valid {
		ap.ReviewedAt = &reviewedAt.Time
	}
	return &ap, nil
}
