package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/verification"
)

// VerificationRepository persists verification attempts and derives the
// subject-level verification state used by badges and capability gating.
type VerificationRepository struct {
	db *PostgresDB
}

func NewVerificationRepository(db *PostgresDB) *VerificationRepository {
	return &VerificationRepository{db: db}
}

// CreateVerification records a new verification attempt and returns its ID.
func (r *VerificationRepository) CreateVerification(
	ctx context.Context,
	subjectID uuid.UUID,
	consentID *uuid.UUID,
	verificationType, vendor, status string,
) (uuid.UUID, error) {
	query := `
		INSERT INTO verification_token (
			subject_id, consent_id, verification_type, vendor, status
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING token_id
	`

	var id uuid.UUID
	if err := r.db.QueryRowContext(ctx, query, subjectID, consentID, verificationType, vendor, status).Scan(&id); err != nil {
		return uuid.Nil, fmt.Errorf("create verification: %w", err)
	}
	return id, nil
}

// CompleteVerification sets the terminal outcome of an attempt.
func (r *VerificationRepository) CompleteVerification(
	ctx context.Context,
	verificationID uuid.UUID,
	status, errorMessage string,
	costUSD float64,
	result map[string]interface{},
) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal verification result: %w", err)
	}

	query := `
		UPDATE verification_token
		SET status = $2, error_message = NULLIF($3, ''), cost_usd = $4,
		    result = $5, completed_at = now(), updated_at = now()
		WHERE token_id = $1
	`
	if _, err := r.db.ExecContext(ctx, query, verificationID, status, errorMessage, costUSD, resultJSON); err != nil {
		return fmt.Errorf("complete verification: %w", err)
	}
	return nil
}

// ListBySubject returns all verification attempts for a subject, newest first.
func (r *VerificationRepository) ListBySubject(ctx context.Context, subjectID uuid.UUID) ([]models.VerificationRecord, error) {
	query := `
		SELECT token_id, consent_id, verification_type, vendor, status,
		       result, error_message, cost_usd, created_at, completed_at
		FROM verification_token
		WHERE subject_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, subjectID)
	if err != nil {
		return nil, fmt.Errorf("list verifications: %w", err)
	}
	defer rows.Close()

	records := make([]models.VerificationRecord, 0)
	for rows.Next() {
		var rec models.VerificationRecord
		var consentID sql.NullString
		var vendor, errMsg sql.NullString
		var resultJSON []byte
		var completedAt sql.NullTime
		var cost float64

		if err := rows.Scan(&rec.VerificationID, &consentID, &rec.Type, &vendor, &rec.Status,
			&resultJSON, &errMsg, &cost, &rec.CreatedAt, &completedAt); err != nil {
			return nil, fmt.Errorf("scan verification: %w", err)
		}

		rec.SubjectID = subjectID.String()
		if consentID.Valid {
			rec.ConsentID = &consentID.String
		}
		if vendor.Valid {
			rec.Vendor = vendor.String
		}
		if errMsg.Valid {
			rec.ErrorMessage = errMsg.String
		}
		rec.CostUSD = cost
		if completedAt.Valid {
			rec.CompletedAt = &completedAt.Time
		}
		if len(resultJSON) > 0 {
			_ = json.Unmarshal(resultJSON, &rec.Result)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate verifications: %w", err)
	}

	return records, nil
}

// MarkVerified sets one of the subject-level verification flags. Only the
// flags that exist as columns are accepted, so a caller cannot inject a column
// name into the statement.
func (r *VerificationRepository) MarkVerified(ctx context.Context, subjectID uuid.UUID, verificationType string) error {
	var query string
	switch verificationType {
	case models.VerificationTypeGovernmentID:
		query = `UPDATE subject SET has_government_id = true, verified_at = now() WHERE subject_id = $1`
	case models.VerificationTypeLiveness:
		query = `UPDATE subject SET has_liveness = true, verified_at = now() WHERE subject_id = $1`
	default:
		// education/employment/image have no dedicated subject flag; they are
		// read back from their own tables when badges are assembled.
		return nil
	}

	if _, err := r.db.ExecContext(ctx, query, subjectID); err != nil {
		return fmt.Errorf("mark verified: %w", err)
	}
	return nil
}

// RecordGovernmentID stores the derived attributes of a successful ID check.
// The document image is never passed here — the vendor holds it.
func (r *VerificationRepository) RecordGovernmentID(ctx context.Context, subjectID uuid.UUID, result *verification.IDVerificationResult, vendorName string) error {
	var dob interface{}
	if result.DateOfBirth != nil {
		dob = *result.DateOfBirth
	}

	query := `
		INSERT INTO government_id_verification (
			subject_id, vendor, vendor_reference_id, status, id_type,
			verified_name, date_of_birth, address_line, country_code,
			failure_reason, cost_usd, completed_at
		) VALUES ($1, $2, $3, $4, '', $5, $6, $7, $8, NULLIF($9, ''), $10, now())
	`
	if _, err := r.db.ExecContext(ctx, query,
		subjectID, vendorName, result.VendorReference, result.Status,
		result.VerifiedName, dob, result.AddressLine, result.CountryCode,
		result.FailureReason, result.CostUSD,
	); err != nil {
		return fmt.Errorf("record government id: %w", err)
	}
	return nil
}

// RecordLiveness stores the outcome of a proof-of-person check.
func (r *VerificationRepository) RecordLiveness(ctx context.Context, subjectID uuid.UUID, result *verification.LivenessResult) error {
	query := `
		INSERT INTO liveness_verification (
			subject_id, vendor, vendor_reference_id, status, liveness_score,
			matched_identity, failure_reason, cost_usd, completed_at
		) VALUES ($1, 'liveness', $2, $3, $4, $5, NULLIF($6, ''), $7, now())
	`
	if _, err := r.db.ExecContext(ctx, query,
		subjectID, result.VendorReference, result.Status, result.LivenessScore,
		result.MatchedIdentity, result.FailureReason, result.CostUSD,
	); err != nil {
		return fmt.Errorf("record liveness: %w", err)
	}
	return nil
}

// ImageVerificationRecord groups the two image checks for one submission.
type ImageVerificationRecord struct {
	SubjectID         uuid.UUID
	ImageHash         string
	ReverseProvider   string
	ReverseMatchCount int
	ReverseMatches    interface{}
	SyntheticProvider string
	SyntheticScore    *float64
	IsSynthetic       bool
}

// RecordImageVerification stores a combined reverse-search + synthetic result.
func (r *VerificationRepository) RecordImageVerification(ctx context.Context, rec *ImageVerificationRecord) error {
	matchesJSON, err := json.Marshal(rec.ReverseMatches)
	if err != nil {
		return fmt.Errorf("marshal reverse matches: %w", err)
	}

	status := models.VerificationStatusVerified
	query := `
		INSERT INTO image_verification (
			subject_id, image_hash, status, reverse_search_provider, match_count, matches,
			synthetic_provider, synthetic_score, is_synthetic, completed_at
		) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, NULLIF($7, ''), $8, $9, now())
	`
	if _, err := r.db.ExecContext(ctx, query,
		rec.SubjectID, rec.ImageHash, status, rec.ReverseProvider, rec.ReverseMatchCount,
		matchesJSON, rec.SyntheticProvider, rec.SyntheticScore, rec.IsSynthetic,
	); err != nil {
		return fmt.Errorf("record image verification: %w", err)
	}
	return nil
}

// SubjectVerificationFlags is the compact verification state used for badges.
type SubjectVerificationFlags struct {
	HasGovernmentID bool
	HasLiveness     bool
	VerifiedAt      *time.Time
	AgeBlocked      bool
	AgeBlockedAt    *time.Time
}

// GetSubjectVerificationFlags loads the subject-level flags.
func (r *VerificationRepository) GetSubjectVerificationFlags(ctx context.Context, subjectID uuid.UUID) (*SubjectVerificationFlags, error) {
	query := `SELECT has_government_id, has_liveness, verified_at, age_blocked, age_blocked_at FROM subject WHERE subject_id = $1`

	var flags SubjectVerificationFlags
	var verifiedAt, ageBlockedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, subjectID).Scan(
		&flags.HasGovernmentID, &flags.HasLiveness, &verifiedAt, &flags.AgeBlocked, &ageBlockedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get verification flags: %w", err)
	}
	if verifiedAt.Valid {
		flags.VerifiedAt = &verifiedAt.Time
	}
	if ageBlockedAt.Valid {
		flags.AgeBlockedAt = &ageBlockedAt.Time
	}
	return &flags, nil
}

// SetAgeBlock records an authoritative underage finding against the subject.
// The identity verification itself stands; this flag is the enforceable age
// restriction that capability gating and status reads consume.
func (r *VerificationRepository) SetAgeBlock(ctx context.Context, subjectID uuid.UUID, source string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE subject SET age_blocked = true, age_blocked_at = now(), age_blocked_source = $2 WHERE subject_id = $1`,
		subjectID, source)
	if err != nil {
		return fmt.Errorf("set age block: %w", err)
	}
	return nil
}
