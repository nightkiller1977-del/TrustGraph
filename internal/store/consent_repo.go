package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

// ConsentRepository stores Plane B consent grants and withdrawals.
type ConsentRepository struct {
	db *PostgresDB
}

func NewConsentRepository(db *PostgresDB) *ConsentRepository {
	return &ConsentRepository{db: db}
}

// GrantConsent records (or re-activates) a subject's consent for one
// purpose. Re-granting after a withdrawal revives the same row rather than
// creating a second one, which the (subject, plane, consent_type) unique
// constraint requires, and clears the previous withdrawal.
func (r *ConsentRepository) GrantConsent(ctx context.Context, subjectID uuid.UUID, plane, consentType, policyVersion string, terms map[string]interface{}) (*models.SubjectConsent, error) {
	termsJSON, err := json.Marshal(terms)
	if err != nil {
		return nil, fmt.Errorf("marshal terms: %w", err)
	}

	query := `
		INSERT INTO subject_consent (
			subject_id, plane, consent_type, consent_status, policy_version,
			granted_at, withdrawn_at, terms_accepted
		) VALUES ($1, $2, $3, $4, $5, now(), NULL, $6)
		ON CONFLICT (subject_id, plane, consent_type) DO UPDATE SET
			consent_status = EXCLUDED.consent_status,
			policy_version = EXCLUDED.policy_version,
			granted_at = EXCLUDED.granted_at,
			withdrawn_at = NULL,
			terms_accepted = EXCLUDED.terms_accepted
		RETURNING consent_id, granted_at, created_at
	`

	var consent models.SubjectConsent
	err = r.db.QueryRowContext(ctx, query,
		subjectID, plane, consentType, models.ConsentStatusGranted, policyVersion, termsJSON,
	).Scan(&consent.ConsentID, &consent.GrantedAt, &consent.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("grant consent: %w", err)
	}

	consent.SubjectID = subjectID.String()
	consent.Plane = plane
	consent.ConsentType = consentType
	consent.ConsentStatus = models.ConsentStatusGranted
	consent.PolicyVersion = policyVersion
	consent.TermsAccepted = terms

	return &consent, nil
}

// WithdrawConsentAndEraseData marks a consent withdrawn and erases the data held
// under that purpose in a single transaction. Withdrawal and erasure must be
// atomic: if they were separate, a failure between them would leave a consent
// that claims to be revoked while the data is still present, and a retry could
// not repair it because the status is already withdrawn.
//
// A retry after a partial failure is therefore supported: an already-withdrawn
// consent still erases its data and reports success. A consent that was never
// granted returns nil so the caller can return 404.
func (r *ConsentRepository) WithdrawConsentAndEraseData(ctx context.Context, subjectID uuid.UUID, plane, consentType string) (*models.SubjectConsent, error) {
	scope, ok := consentScopedDeletes[consentType]
	if !ok {
		return nil, fmt.Errorf("unknown consent type %q", consentType)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin withdraw tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var consent models.SubjectConsent
	var grantedAt, withdrawnAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		UPDATE subject_consent
		SET consent_status = $4, withdrawn_at = now()
		WHERE subject_id = $1 AND plane = $2 AND consent_type = $3
		  AND consent_status <> $4
		RETURNING consent_id, granted_at, withdrawn_at, created_at
	`, subjectID, plane, consentType, models.ConsentStatusWithdrawn,
	).Scan(&consent.ConsentID, &grantedAt, &withdrawnAt, &consent.CreatedAt)

	switch {
	case err == nil:
		// Freshly withdrawn.
	case err == sql.ErrNoRows:
		// Either the consent was already withdrawn (a retry) or it never
		// existed. Distinguish the two so a retry still erases its data.
		retryErr := tx.QueryRowContext(ctx, `
			SELECT consent_id, granted_at, withdrawn_at, created_at
			FROM subject_consent
			WHERE subject_id = $1 AND plane = $2 AND consent_type = $3
		`, subjectID, plane, consentType,
		).Scan(&consent.ConsentID, &grantedAt, &withdrawnAt, &consent.CreatedAt)
		if retryErr == sql.ErrNoRows {
			return nil, nil
		}
		if retryErr != nil {
			return nil, fmt.Errorf("load withdrawn consent: %w", retryErr)
		}
	default:
		return nil, fmt.Errorf("withdraw consent: %w", err)
	}

	if err := deleteConsentScopedDataTx(ctx, tx, subjectID, consentType, scope); err != nil {
		return nil, err
	}
	if err := refreshVerificationFlagsTx(ctx, tx, subjectID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit withdraw: %w", err)
	}

	consent.SubjectID = subjectID.String()
	consent.Plane = plane
	consent.ConsentType = consentType
	consent.ConsentStatus = models.ConsentStatusWithdrawn
	if grantedAt.Valid {
		consent.GrantedAt = &grantedAt.Time
	}
	if withdrawnAt.Valid {
		consent.WithdrawnAt = &withdrawnAt.Time
	}

	return &consent, nil
}

// IsGranted reports whether an active consent exists for a purpose. It is the
// gate every Plane B write path must pass before storing user data.
func (r *ConsentRepository) IsGranted(ctx context.Context, subjectID uuid.UUID, plane, consentType string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM subject_consent
			WHERE subject_id = $1 AND plane = $2 AND consent_type = $3
			  AND consent_status = $4
			  AND (expires_at IS NULL OR expires_at > now())
		)
	`

	var granted bool
	if err := r.db.QueryRowContext(ctx, query, subjectID, plane, consentType, models.ConsentStatusGranted).Scan(&granted); err != nil {
		return false, fmt.Errorf("check consent: %w", err)
	}
	return granted, nil
}

// ListBySubject returns every consent row for a subject, newest first.
func (r *ConsentRepository) ListBySubject(ctx context.Context, subjectID uuid.UUID) ([]models.SubjectConsent, error) {
	query := `
		SELECT consent_id, plane, consent_type, consent_status, policy_version,
		       granted_at, withdrawn_at, expires_at, created_at, terms_accepted
		FROM subject_consent
		WHERE subject_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, subjectID)
	if err != nil {
		return nil, fmt.Errorf("list consents: %w", err)
	}
	defer rows.Close()

	consents := make([]models.SubjectConsent, 0)
	for rows.Next() {
		var c models.SubjectConsent
		var policyVersion sql.NullString
		var grantedAt, withdrawnAt, expiresAt sql.NullTime
		var termsRaw []byte
		if err := rows.Scan(&c.ConsentID, &c.Plane, &c.ConsentType, &c.ConsentStatus,
			&policyVersion, &grantedAt, &withdrawnAt, &expiresAt, &c.CreatedAt, &termsRaw); err != nil {
			return nil, fmt.Errorf("scan consent: %w", err)
		}
		c.SubjectID = subjectID.String()
		if policyVersion.Valid {
			c.PolicyVersion = policyVersion.String
		}
		if len(termsRaw) > 0 {
			if err := json.Unmarshal(termsRaw, &c.TermsAccepted); err != nil {
				return nil, fmt.Errorf("decode terms accepted: %w", err)
			}
		}
		if grantedAt.Valid {
			c.GrantedAt = &grantedAt.Time
		}
		if withdrawnAt.Valid {
			c.WithdrawnAt = &withdrawnAt.Time
		}
		if expiresAt.Valid {
			c.ExpiresAt = &expiresAt.Time
		}
		consents = append(consents, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate consents: %w", err)
	}

	return consents, nil
}

// DeleteSubjectPlaneBData removes Plane B data for a subject. It backs the
// privacy right-to-erasure path: consent rows, verification attempts, the
// derived attributes, LinkedIn profiles and tokens, employment history, and
// any pending OAuth state all go, and the subject's verification flags are
// reset. There is no per-purpose variant on purpose — a partial erase is the
// failure mode that leaves PII behind after the user asked for deletion, so
// the whole Plane B set is removed in one transaction.
func (r *ConsentRepository) DeleteSubjectPlaneBData(ctx context.Context, subjectID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	statements := []string{
		`DELETE FROM verification_token WHERE subject_id = $1`,
		`DELETE FROM government_id_verification WHERE subject_id = $1`,
		`DELETE FROM liveness_verification WHERE subject_id = $1`,
		`DELETE FROM image_verification WHERE subject_id = $1`,
		`DELETE FROM education_verification_request WHERE subject_id = $1`,
		`DELETE FROM subject_education WHERE subject_id = $1`,
		`DELETE FROM employment_verification_request WHERE subject_id = $1`,
		`DELETE FROM subject_employment WHERE subject_id = $1`,
		`DELETE FROM subject_linkedin_profile WHERE subject_id = $1`,
		`DELETE FROM oauth_state WHERE subject_id = $1`,
		`DELETE FROM subject_consent WHERE subject_id = $1`,
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt, subjectID); err != nil {
			return fmt.Errorf("delete plane B data: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE subject SET has_government_id = false, has_liveness = false, verified_at = NULL,
		        age_blocked = false, age_blocked_at = NULL, age_blocked_source = NULL
		 WHERE subject_id = $1`,
		subjectID); err != nil {
		return fmt.Errorf("reset subject verification flags: %w", err)
	}

	return tx.Commit()
}

// ErrConsentRequired maps "no active consent" to a typed error so handlers can
// return 403 rather than 500.
var ErrConsentRequired = fmt.Errorf("active consent required")

// consentDeleteScope names the tables and token types a single consent purpose
// covers.
type consentDeleteScope struct {
	tables     []string
	tokenTypes []string
}

// consentScopedDeletes maps a consent type to the statements that erase only
// the data collected under that purpose. Withdrawing image-verification consent
// must not take the subject's LinkedIn profile with it, and vice versa. Each
// entry also clears the verification_token rows of the matching type so a
// pending or historical attempt does not outlive its consent.
var consentScopedDeletes = map[string]consentDeleteScope{
	models.ConsentTypeLinkedIn: {
		tables: []string{
			`DELETE FROM subject_linkedin_profile WHERE subject_id = $1`,
			`DELETE FROM subject_employment WHERE subject_id = $1`,
			`DELETE FROM employment_verification_request WHERE subject_id = $1`,
			// Education is sourced from the LinkedIn OAuth profile, so it is
			// covered by this consent too; leaving it behind would keep the
			// education badge after the deletion reported success. The request
			// rows reference subject_education, so they must go first.
			`DELETE FROM education_verification_request WHERE subject_id = $1`,
			`DELETE FROM subject_education WHERE subject_id = $1`,
			`DELETE FROM oauth_state WHERE subject_id = $1`,
		},
		tokenTypes: []string{models.VerificationTypeEmployment, models.VerificationTypeEducation},
	},
	models.ConsentTypeGovernmentID: {
		tables:     []string{`DELETE FROM government_id_verification WHERE subject_id = $1`},
		tokenTypes: []string{models.VerificationTypeGovernmentID},
	},
	models.ConsentTypeLiveness: {
		tables:     []string{`DELETE FROM liveness_verification WHERE subject_id = $1`},
		tokenTypes: []string{models.VerificationTypeLiveness},
	},
	models.ConsentTypeImageVerification: {
		tables:     []string{`DELETE FROM image_verification WHERE subject_id = $1`},
		tokenTypes: []string{models.VerificationTypeImage},
	},
}

// DeleteSubjectPlaneBDataForConsent erases the Plane B data held under a single
// consent purpose. It is the correct call for a consent withdrawal: only the
// data that consent covered is removed, so withdrawing one purpose cannot
// destroy unrelated data the subject still consented to.
func (r *ConsentRepository) DeleteSubjectPlaneBDataForConsent(ctx context.Context, subjectID uuid.UUID, consentType string) error {
	scope, ok := consentScopedDeletes[consentType]
	if !ok {
		return fmt.Errorf("unknown consent type %q", consentType)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := deleteConsentScopedDataTx(ctx, tx, subjectID, consentType, scope); err != nil {
		return err
	}

	if err := refreshVerificationFlagsTx(ctx, tx, subjectID); err != nil {
		return err
	}

	return tx.Commit()
}

// deleteConsentScopedDataTx erases the rows a single consent purpose covered.
// It runs inside the caller's transaction so a withdrawal can change consent
// status and erase its data atomically.
func deleteConsentScopedDataTx(ctx context.Context, tx *sql.Tx, subjectID uuid.UUID, consentType string, scope consentDeleteScope) error {
	for _, stmt := range scope.tables {
		if _, err := tx.ExecContext(ctx, stmt, subjectID); err != nil {
			return fmt.Errorf("delete %s data: %w", consentType, err)
		}
	}
	for _, vType := range scope.tokenTypes {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM verification_token WHERE subject_id = $1 AND verification_type = $2`,
			subjectID, vType); err != nil {
			return fmt.Errorf("delete %s tokens: %w", consentType, err)
		}
	}
	return nil
}

// refreshVerificationFlagsTx recomputes the subject's denormalised verification
// flags from the surviving verification rows. Recomputing (rather than blindly
// clearing) keeps the flags correct after a purpose-scoped erase: withdrawing
// image consent must not un-verify a government ID.
func refreshVerificationFlagsTx(ctx context.Context, tx *sql.Tx, subjectID uuid.UUID) error {
	var hasID, hasLiveness bool
	if err := tx.QueryRowContext(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM government_id_verification WHERE subject_id = $1 AND status = 'verified'),
			EXISTS (SELECT 1 FROM liveness_verification WHERE subject_id = $1 AND status = 'verified')
	`, subjectID).Scan(&hasID, &hasLiveness); err != nil {
		return fmt.Errorf("recompute verification flags: %w", err)
	}

	var verifiedAt interface{}
	if hasID || hasLiveness {
		verifiedAt = time.Now()
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE subject SET has_government_id = $2, has_liveness = $3, verified_at = $4 WHERE subject_id = $1`,
		subjectID, hasID, hasLiveness, verifiedAt); err != nil {
		return fmt.Errorf("update subject verification flags: %w", err)
	}
	return nil
}

// RequireConsent returns ErrConsentRequired unless an active consent exists.
func (r *ConsentRepository) RequireConsent(ctx context.Context, subjectID uuid.UUID, plane, consentType string) error {
	granted, err := r.IsGranted(ctx, subjectID, plane, consentType)
	if err != nil {
		return err
	}
	if !granted {
		return ErrConsentRequired
	}
	return nil
}
