package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// SubjectRepository handles database operations for subjects
type SubjectRepository struct {
	db *PostgresDB
}

// NewSubjectRepository creates a new subject repository
func NewSubjectRepository(db *PostgresDB) *SubjectRepository {
	return &SubjectRepository{db: db}
}

// FindOrCreateSubject looks up a subject by ConnectionSphere user ID.
// If not found, creates one and returns it. This is idempotent.
func (r *SubjectRepository) FindOrCreateSubject(ctx context.Context, connectionSphereUserID string) (uuid.UUID, error) {
	// Attempt insert; ON CONFLICT does nothing if the user already exists
	insertQuery := `
		INSERT INTO subject (connection_sphere_user_id)
		VALUES ($1)
		ON CONFLICT (connection_sphere_user_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, insertQuery, connectionSphereUserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert subject: %w", err)
	}

	// Always SELECT to get the actual ID (whether just inserted or already existed)
	selectQuery := `
		SELECT subject_id FROM subject
		WHERE connection_sphere_user_id = $1 AND deleted_at IS NULL
	`

	var subjectID uuid.UUID
	err = r.db.QueryRowContext(ctx, selectQuery, connectionSphereUserID).Scan(&subjectID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to retrieve subject after upsert: %w", err)
	}

	return subjectID, nil
}

// GetSubjectByCSUserID returns the subject_id for a ConnectionSphere user, or uuid.Nil if not found
func (r *SubjectRepository) GetSubjectByCSUserID(ctx context.Context, csUserID string) (uuid.UUID, error) {
	query := `
		SELECT subject_id FROM subject
		WHERE connection_sphere_user_id = $1 AND deleted_at IS NULL
	`

	var subjectID uuid.UUID
	err := r.db.QueryRowContext(ctx, query, csUserID).Scan(&subjectID)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, nil
		}
		return uuid.Nil, fmt.Errorf("failed to get subject by CS user ID: %w", err)
	}

	return subjectID, nil
}

// GetSubjectTrustTier returns the most recent trust tier for a subject from their latest assessment
func (r *SubjectRepository) GetSubjectTrustTier(ctx context.Context, subjectID uuid.UUID) (string, error) {
	query := `
		SELECT trust_tier FROM assessment
		WHERE subject_id = $1 AND status = 'complete'
		ORDER BY created_at DESC
		LIMIT 1
	`

	var trustTier string
	err := r.db.QueryRowContext(ctx, query, subjectID).Scan(&trustTier)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("failed to get subject trust tier: %w", err)
	}

	return trustTier, nil
}
