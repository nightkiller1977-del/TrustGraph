package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// SubjectEducation represents education data stored for a subject
type SubjectEducation struct {
	EducationID         uuid.UUID
	SubjectID           uuid.UUID
	SchoolName          string
	FieldOfStudy        string
	StartDate           time.Time
	EndDate             time.Time
	Grade               string
	ConfidenceScore     int
	IsVerified          bool
	ValidationSignals   []string
	ValidationDetails   string
	ValidationRiskScore int
	Source              string
	SourceData          map[string]interface{}
	ValidatedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ExpiresAt           *time.Time
}

// EducationRepository handles database operations for education verification
type EducationRepository struct {
	db *PostgresDB
}

// NewEducationRepository creates a new education repository
func NewEducationRepository(db *PostgresDB) *EducationRepository {
	return &EducationRepository{db: db}
}

// SaveEducation inserts or updates education data for a subject
func (r *EducationRepository) SaveEducation(ctx context.Context, edu *SubjectEducation) error {
	sourceDataJSON, err := json.Marshal(edu.SourceData)
	if err != nil {
		return fmt.Errorf("failed to marshal source_data: %w", err)
	}

	query := `
		INSERT INTO subject_education (
			subject_id, school_name, field_of_study, start_date, end_date, grade,
			confidence_score, is_verified, validation_signals, validation_details,
			validation_risk_score, source, source_data, validated_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (subject_id) DO UPDATE SET
			school_name = EXCLUDED.school_name,
			field_of_study = EXCLUDED.field_of_study,
			start_date = EXCLUDED.start_date,
			end_date = EXCLUDED.end_date,
			grade = EXCLUDED.grade,
			confidence_score = EXCLUDED.confidence_score,
			is_verified = EXCLUDED.is_verified,
			validation_signals = EXCLUDED.validation_signals,
			validation_details = EXCLUDED.validation_details,
			validation_risk_score = EXCLUDED.validation_risk_score,
			source = EXCLUDED.source,
			source_data = EXCLUDED.source_data,
			validated_at = EXCLUDED.validated_at,
			expires_at = EXCLUDED.expires_at,
			updated_at = now()
		RETURNING education_id
	`

	now := time.Now()
	if edu.ValidatedAt == nil {
		edu.ValidatedAt = &now
	}

	var educationID uuid.UUID
	err = r.db.QueryRowContext(
		ctx, query,
		edu.SubjectID,
		edu.SchoolName,
		edu.FieldOfStudy,
		edu.StartDate,
		edu.EndDate,
		edu.Grade,
		edu.ConfidenceScore,
		edu.IsVerified,
		pq.Array(edu.ValidationSignals),
		edu.ValidationDetails,
		edu.ValidationRiskScore,
		edu.Source,
		sourceDataJSON,
		edu.ValidatedAt,
		edu.ExpiresAt,
	).Scan(&educationID)

	if err != nil {
		return fmt.Errorf("failed to save education: %w", err)
	}

	edu.EducationID = educationID
	return nil
}

// GetEducationBySubject retrieves education data for a subject
func (r *EducationRepository) GetEducationBySubject(ctx context.Context, subjectID uuid.UUID) (*SubjectEducation, error) {
	query := `
		SELECT
			education_id, subject_id, school_name, field_of_study, start_date, end_date, grade,
			confidence_score, is_verified, validation_signals, validation_details,
			validation_risk_score, source, source_data, validated_at, created_at, updated_at, expires_at
		FROM subject_education
		WHERE subject_id = $1
		AND (expires_at IS NULL OR expires_at > now())
	`

	var edu SubjectEducation
	var sourceDataJSON []byte
	var validationSignals pq.StringArray

	err := r.db.QueryRowContext(ctx, query, subjectID).Scan(
		&edu.EducationID,
		&edu.SubjectID,
		&edu.SchoolName,
		&edu.FieldOfStudy,
		&edu.StartDate,
		&edu.EndDate,
		&edu.Grade,
		&edu.ConfidenceScore,
		&edu.IsVerified,
		&validationSignals,
		&edu.ValidationDetails,
		&edu.ValidationRiskScore,
		&edu.Source,
		&sourceDataJSON,
		&edu.ValidatedAt,
		&edu.CreatedAt,
		&edu.UpdatedAt,
		&edu.ExpiresAt,
	)

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get education: %w", err)
	}

	edu.ValidationSignals = []string(validationSignals)

	if err := json.Unmarshal(sourceDataJSON, &edu.SourceData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal source_data: %w", err)
	}

	return &edu, nil
}

// GetEducationConfidence retrieves just the confidence score and verification status
func (r *EducationRepository) GetEducationConfidence(ctx context.Context, subjectID uuid.UUID) (confidence int, isVerified bool, err error) {
	query := `
		SELECT confidence_score, is_verified
		FROM subject_education
		WHERE subject_id = $1
		AND (expires_at IS NULL OR expires_at > now())
	`

	err = r.db.QueryRowContext(ctx, query, subjectID).Scan(&confidence, &isVerified)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get education confidence: %w", err)
	}

	return confidence, isVerified, nil
}

// CountVerifiedEducationBySchool counts how many verified users attended the same school
// (useful for networking features, etc.)
func (r *EducationRepository) CountVerifiedEducationBySchool(ctx context.Context, schoolName string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT subject_id)
		FROM subject_education
		WHERE school_name = $1
		AND is_verified = true
		AND (expires_at IS NULL OR expires_at > now())
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, schoolName).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count education by school: %w", err)
	}

	return count, nil
}

// SaveEducationVerificationRequest tracks paid verification requests.
// Takes only educationID and derives subject_id from the subject_education
// row itself (in the same INSERT, via a subquery) rather than accepting it
// as a separate caller-supplied parameter — subject_id and education_id
// each validate independently against their own foreign keys, so a caller
// passing an educationID that belongs to a DIFFERENT subject than the one
// supplied would otherwise record a verification request (and its eventual
// vendor result) against the wrong subject/education pairing undetected.
func (r *EducationRepository) SaveEducationVerificationRequest(
	ctx context.Context,
	educationID uuid.UUID,
	verificationType string,
	vendor string,
	vendorRequestID string,
) (uuid.UUID, error) {
	query := `
		INSERT INTO education_verification_request (
			subject_id, education_id, verification_type, vendor, vendor_request_id, status
		)
		SELECT subject_id, education_id, $2, $3, $4, 'pending'
		FROM subject_education
		WHERE education_id = $1
		RETURNING request_id
	`

	var requestID uuid.UUID
	err := r.db.QueryRowContext(ctx, query, educationID, verificationType, vendor, vendorRequestID).Scan(&requestID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return uuid.Nil, fmt.Errorf("no education record found for education_id %s", educationID)
		}
		return uuid.Nil, fmt.Errorf("failed to save verification request: %w", err)
	}

	return requestID, nil
}
