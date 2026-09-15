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

// LinkedInProfile is the stored LinkedIn identity for a subject.
//
// AccessToken and RefreshToken are present so the OAuth flow can persist
// credentials, but reads deliberately select explicit columns (see
// GetProfileBySubject) so tokens never leak into an API response that decodes
// this struct wholesale.
type LinkedInProfile struct {
	LinkedInProfileID uuid.UUID
	SubjectID         uuid.UUID
	LinkedInID        string
	Email             string
	FirstName         string
	LastName          string
	ProfileURL        string
	Headline          string
	ConnectionsCount  int
	AccessToken       string
	RefreshToken      string
	TokenExpiresAt    *time.Time
	Scopes            []string
	RawProfile        map[string]interface{}
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// LinkedInRepository persists LinkedIn profiles and OAuth state.
type LinkedInRepository struct {
	db *PostgresDB
}

func NewLinkedInRepository(db *PostgresDB) *LinkedInRepository {
	return &LinkedInRepository{db: db}
}

// UpsertProfile stores or refreshes a subject's LinkedIn profile.
func (r *LinkedInRepository) UpsertProfile(ctx context.Context, p *LinkedInProfile, consentID *uuid.UUID) error {
	raw, err := json.Marshal(p.RawProfile)
	if err != nil {
		return fmt.Errorf("marshal linkedin profile: %w", err)
	}

	query := `
		INSERT INTO subject_linkedin_profile (
			subject_id, linkedin_id, email, first_name, last_name, profile_url, headline,
			connections_count, access_token, refresh_token, token_expires_at, scopes,
			consent_id, raw_profile
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (subject_id) DO UPDATE SET
			linkedin_id = EXCLUDED.linkedin_id,
			email = EXCLUDED.email,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			profile_url = EXCLUDED.profile_url,
			headline = EXCLUDED.headline,
			connections_count = EXCLUDED.connections_count,
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_expires_at = EXCLUDED.token_expires_at,
			scopes = EXCLUDED.scopes,
			consent_id = EXCLUDED.consent_id,
			raw_profile = EXCLUDED.raw_profile,
			updated_at = now()
		RETURNING linkedin_profile_id
	`

	err = r.db.QueryRowContext(ctx, query,
		p.SubjectID, p.LinkedInID, p.Email, p.FirstName, p.LastName, p.ProfileURL, p.Headline,
		p.ConnectionsCount, p.AccessToken, p.RefreshToken, p.TokenExpiresAt, pq.Array(p.Scopes),
		consentID, raw,
	).Scan(&p.LinkedInProfileID)
	if err != nil {
		return fmt.Errorf("upsert linkedin profile: %w", err)
	}
	return nil
}

// GetProfileBySubject loads the non-secret fields of a subject's LinkedIn
// profile. Tokens are intentionally not selected: this result is returned by
// the verification API, and there is no reason for an API caller to see them.
func (r *LinkedInRepository) GetProfileBySubject(ctx context.Context, subjectID uuid.UUID) (*LinkedInProfile, error) {
	query := `
		SELECT linkedin_profile_id, linkedin_id, email, first_name, last_name,
		       profile_url, headline, connections_count, token_expires_at, scopes,
		       created_at, updated_at
		FROM subject_linkedin_profile
		WHERE subject_id = $1
	`

	var p LinkedInProfile
	var tokenExpiresAt sql.NullTime
	// scopes is stored NULL when the vendor returns no scope list; pq.Array
	// cannot scan NULL into a []string, so it is read as nullable and mapped
	// after the scan.
	var scopes pq.StringArray
	err := r.db.QueryRowContext(ctx, query, subjectID).Scan(
		&p.LinkedInProfileID, &p.LinkedInID, &p.Email, &p.FirstName, &p.LastName,
		&p.ProfileURL, &p.Headline, &p.ConnectionsCount, &tokenExpiresAt, &scopes,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get linkedin profile: %w", err)
	}

	p.SubjectID = subjectID
	p.Scopes = []string(scopes)
	if tokenExpiresAt.Valid {
		p.TokenExpiresAt = &tokenExpiresAt.Time
	}
	return &p, nil
}

// CreateOAuthState records a single-use state value for the authorization
// callback.
func (r *LinkedInRepository) CreateOAuthState(ctx context.Context, state string, subjectID uuid.UUID, provider, redirectURI string) error {
	query := `
		INSERT INTO oauth_state (state, subject_id, provider, redirect_uri)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := r.db.ExecContext(ctx, query, state, subjectID, provider, redirectURI); err != nil {
		return fmt.Errorf("create oauth state: %w", err)
	}
	return nil
}

// ConsumeOAuthState atomically validates and marks a state value used. A state
// that is unknown, expired, or already consumed returns (uuid.Nil, "", "").
func (r *LinkedInRepository) ConsumeOAuthState(ctx context.Context, state, provider string) (uuid.UUID, string, error) {
	query := `
		UPDATE oauth_state
		SET consumed_at = now()
		WHERE state = $1 AND provider = $2
		  AND consumed_at IS NULL
		  AND expires_at > now()
		RETURNING subject_id, COALESCE(redirect_uri, '')
	`

	var subjectID uuid.UUID
	var redirectURI string
	err := r.db.QueryRowContext(ctx, query, state, provider).Scan(&subjectID, &redirectURI)
	if err != nil {
		if err == sql.ErrNoRows {
			return uuid.Nil, "", nil
		}
		return uuid.Nil, "", fmt.Errorf("consume oauth state: %w", err)
	}
	return subjectID, redirectURI, nil
}

// DeleteProfile removes a subject's LinkedIn data.
func (r *LinkedInRepository) DeleteProfile(ctx context.Context, subjectID uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM subject_linkedin_profile WHERE subject_id = $1`, subjectID); err != nil {
		return fmt.Errorf("delete linkedin profile: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM subject_employment WHERE subject_id = $1`, subjectID); err != nil {
		return fmt.Errorf("delete employment: %w", err)
	}
	return nil
}
