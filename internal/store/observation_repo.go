package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// ObservationRepository handles database operations for observations
type ObservationRepository struct {
	db *PostgresDB
}

// NewObservationRepository creates a new observation repository
func NewObservationRepository(db *PostgresDB) *ObservationRepository {
	return &ObservationRepository{db: db}
}

// RecordObservation inserts a new observation for a subject
func (r *ObservationRepository) RecordObservation(ctx context.Context, assessmentID, subjectID uuid.UUID, observationType, plane, source string, sourceData map[string]interface{}, confidence float64) error {
	sourceDataJSON, err := json.Marshal(sourceData)
	if err != nil {
		return fmt.Errorf("failed to marshal source_data: %w", err)
	}

	var assessmentParam interface{}
	if assessmentID != uuid.Nil {
		assessmentParam = assessmentID
	}

	query := `
		INSERT INTO observation (
			assessment_id, subject_id, observation_type, plane, source,
			source_data, confidence
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = r.db.ExecContext(ctx, query,
		assessmentParam,
		subjectID,
		observationType,
		plane,
		source,
		sourceDataJSON,
		confidence,
	)
	if err != nil {
		return fmt.Errorf("failed to record observation: %w", err)
	}

	return nil
}

// CountObservationsByIPInWindow counts registrations from the same IP in a time window (for velocity checks)
func (r *ObservationRepository) CountObservationsByIPInWindow(ctx context.Context, ipAddress string, windowMinutes int) (int, error) {
	query := `
		SELECT COUNT(*) FROM observation
		WHERE source_data->>'ip_address' = $1
		  AND created_at > now() - make_interval(mins := $2)
		  AND observation_type = 'registration'
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, ipAddress, windowMinutes).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count observations by IP in window: %w", err)
	}

	return count, nil
}

// FindObservationsByDeviceFingerprint finds subjects who share the same device fingerprint
func (r *ObservationRepository) FindObservationsByDeviceFingerprint(ctx context.Context, fingerprint string, excludeSubjectID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT DISTINCT subject_id FROM observation
		WHERE source_data->>'device_fingerprint' = $1
		  AND subject_id != $2
	`

	rows, err := r.db.QueryContext(ctx, query, fingerprint, excludeSubjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to find observations by device fingerprint: %w", err)
	}
	defer rows.Close()

	var subjectIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan subject ID: %w", err)
		}
		subjectIDs = append(subjectIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating observation rows: %w", err)
	}

	return subjectIDs, nil
}

// FindObservationsByImageHash finds subjects who share the same image hash
func (r *ObservationRepository) FindObservationsByImageHash(ctx context.Context, imageHash string, excludeSubjectID uuid.UUID) ([]uuid.UUID, error) {
	query := `
		SELECT DISTINCT subject_id FROM observation
		WHERE source_data->>'image_hash' = $1
		  AND subject_id != $2
	`

	rows, err := r.db.QueryContext(ctx, query, imageHash, excludeSubjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to find observations by image hash: %w", err)
	}
	defer rows.Close()

	var subjectIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan subject ID: %w", err)
		}
		subjectIDs = append(subjectIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating observation rows: %w", err)
	}

	return subjectIDs, nil
}
