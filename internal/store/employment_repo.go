package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// SubjectEmployment is one job record for a subject.
type SubjectEmployment struct {
	EmploymentID        uuid.UUID
	SubjectID           uuid.UUID
	CompanyName         string
	Title               string
	StartDate           time.Time
	EndDate             time.Time
	IsCurrent           bool
	Location            string
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

// EmploymentRepository stores employment history.
type EmploymentRepository struct {
	db *PostgresDB
}

func NewEmploymentRepository(db *PostgresDB) *EmploymentRepository {
	return &EmploymentRepository{db: db}
}

// ReplaceEmploymentForSubject atomically swaps a subject's employment history.
// LinkedIn returns the whole history at once, so replacing (rather than
// upserting row-by-row) keeps the stored set consistent with the source and
// avoids leaving stale roles behind after a re-sync.
func (r *EmploymentRepository) ReplaceEmploymentForSubject(ctx context.Context, subjectID uuid.UUID, jobs []SubjectEmployment) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin employment tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM subject_employment WHERE subject_id = $1`, subjectID); err != nil {
		return fmt.Errorf("clear employment: %w", err)
	}

	const insert = `
		INSERT INTO subject_employment (
			subject_id, company_name, title, start_date, end_date, is_current, location,
			confidence_score, is_verified, validation_signals, validation_details,
			validation_risk_score, source, source_data, validated_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	now := time.Now()
	for i := range jobs {
		job := &jobs[i]
		sourceData, err := json.Marshal(job.SourceData)
		if err != nil {
			return fmt.Errorf("marshal employment source_data: %w", err)
		}
		if job.ValidatedAt == nil {
			job.ValidatedAt = &now
		}
		if _, err := tx.ExecContext(ctx, insert,
			subjectID, job.CompanyName, job.Title, nullableTime(job.StartDate), nullableTime(job.EndDate),
			job.IsCurrent, job.Location, job.ConfidenceScore, job.IsVerified,
			pq.Array(job.ValidationSignals), job.ValidationDetails, job.ValidationRiskScore,
			job.Source, sourceData, job.ValidatedAt, job.ExpiresAt,
		); err != nil {
			return fmt.Errorf("insert employment: %w", err)
		}
	}

	return tx.Commit()
}

// ListBySubject returns a subject's employment history, current roles first.
func (r *EmploymentRepository) ListBySubject(ctx context.Context, subjectID uuid.UUID) ([]SubjectEmployment, error) {
	query := `
		SELECT employment_id, company_name, title, start_date, end_date, is_current, location,
		       confidence_score, is_verified, validation_signals, validation_details,
		       validation_risk_score, source, source_data, validated_at, created_at, updated_at, expires_at
		FROM subject_employment
		WHERE subject_id = $1
		ORDER BY is_current DESC, start_date DESC NULLS LAST
	`

	rows, err := r.db.QueryContext(ctx, query, subjectID)
	if err != nil {
		return nil, fmt.Errorf("list employment: %w", err)
	}
	defer rows.Close()

	jobs := make([]SubjectEmployment, 0)
	for rows.Next() {
		var job SubjectEmployment
		var startDate, endDate, validatedAt, expiresAt sql.NullTime
		var signals pq.StringArray
		var sourceData []byte

		if err := rows.Scan(
			&job.EmploymentID, &job.CompanyName, &job.Title, &startDate, &endDate,
			&job.IsCurrent, &job.Location, &job.ConfidenceScore, &job.IsVerified,
			&signals, &job.ValidationDetails, &job.ValidationRiskScore, &job.Source,
			&sourceData, &validatedAt, &job.CreatedAt, &job.UpdatedAt, &expiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan employment: %w", err)
		}

		job.SubjectID = subjectID
		job.ValidationSignals = []string(signals)
		if startDate.Valid {
			job.StartDate = startDate.Time
		}
		if endDate.Valid {
			job.EndDate = endDate.Time
		}
		if validatedAt.Valid {
			job.ValidatedAt = &validatedAt.Time
		}
		if expiresAt.Valid {
			job.ExpiresAt = &expiresAt.Time
		}
		if len(sourceData) > 0 {
			_ = json.Unmarshal(sourceData, &job.SourceData)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate employment: %w", err)
	}

	return jobs, nil
}

// CurrentEmployment returns the subject's current role, or nil when none is
// recorded.
func (r *EmploymentRepository) CurrentEmployment(ctx context.Context, subjectID uuid.UUID) (*SubjectEmployment, error) {
	jobs, err := r.ListBySubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		if jobs[i].IsCurrent {
			return &jobs[i], nil
		}
	}
	if len(jobs) > 0 {
		return &jobs[0], nil
	}
	return nil, nil
}

func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}
