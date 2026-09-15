-- Phase 1 Plane B: LinkedIn / employment verification
--
-- Employment data captured from LinkedIn OAuth, plus the LinkedIn profile
-- record itself. Kept in its own migration (rather than appended to 002) for
-- the same reason documented in 004: schema_migrations skips a version once
-- applied, so editing an already-shipped migration would never re-run.

-- Employment history. A subject can hold many jobs, so this is keyed by
-- employment_id and permits several rows per subject — unlike subject_education,
-- which models a single highest qualification per subject.
CREATE TABLE IF NOT EXISTS subject_employment (
    employment_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id         UUID NOT NULL REFERENCES subject(subject_id),
    company_name       VARCHAR(255) NOT NULL,
    title              VARCHAR(255),
    start_date         TIMESTAMPTZ,
    end_date           TIMESTAMPTZ,
    is_current         BOOLEAN DEFAULT false,
    location           VARCHAR(255),

    -- Validation results (free validation, no vendor)
    confidence_score   INTEGER,
    is_verified        BOOLEAN,
    validation_signals TEXT[],
    validation_details TEXT,
    validation_risk_score INTEGER,

    source             VARCHAR(50),
    source_data        JSONB,
    validated_at      TIMESTAMPTZ,
    created_at         TIMESTAMPTZ DEFAULT now(),
    updated_at         TIMESTAMPTZ DEFAULT now(),
    expires_at         TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_subject_employment_subject ON subject_employment(subject_id);
CREATE INDEX IF NOT EXISTS idx_subject_employment_company ON subject_employment(company_name);
CREATE INDEX IF NOT EXISTS idx_subject_employment_current ON subject_employment(is_current);

-- LinkedIn profile + OAuth token handling. One row per subject.
-- The access token is stored encrypted-at-rest by the deployment platform; it
-- is never returned by any API response (see store.LinkedInRepository, which
-- selects explicit columns for reads).
CREATE TABLE IF NOT EXISTS subject_linkedin_profile (
    linkedin_profile_id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id            UUID NOT NULL UNIQUE REFERENCES subject(subject_id),
    linkedin_id           VARCHAR(255),
    email                 VARCHAR(255),
    first_name            VARCHAR(255),
    last_name             VARCHAR(255),
    profile_url           VARCHAR(512),
    headline              VARCHAR(512),
    connections_count     INTEGER,
    access_token          TEXT,
    refresh_token         TEXT,
    token_expires_at      TIMESTAMPTZ,
    scopes                TEXT[],
    consent_id            UUID REFERENCES subject_consent(consent_id),
    raw_profile           JSONB,
    created_at            TIMESTAMPTZ DEFAULT now(),
    updated_at            TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_linkedin_profile_subject ON subject_linkedin_profile(subject_id);
CREATE INDEX IF NOT EXISTS idx_linkedin_profile_linkedin_id ON subject_linkedin_profile(linkedin_id);

-- OAuth state is single-use and short-lived; it exists to stop CSRF on the
-- authorization callback. Kept in the database (not memory) so it survives a
-- restart mid-flow and works across more than one API instance.
CREATE TABLE IF NOT EXISTS oauth_state (
    state         VARCHAR(128) PRIMARY KEY,
    subject_id    UUID NOT NULL REFERENCES subject(subject_id),
    provider      VARCHAR(50) NOT NULL,
    redirect_uri  VARCHAR(1024),
    created_at    TIMESTAMPTZ DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT now() + interval '15 minutes',
    consumed_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_oauth_state_expires ON oauth_state(expires_at);

-- Employment verification requests (for future paid/credentialed verification),
-- mirroring education_verification_request.
CREATE TABLE IF NOT EXISTS employment_verification_request (
    request_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id        UUID NOT NULL REFERENCES subject(subject_id),
    employment_id     UUID REFERENCES subject_employment(employment_id),
    verification_type VARCHAR(50),
    vendor            VARCHAR(100),
    vendor_request_id VARCHAR(255),
    status            VARCHAR(50) DEFAULT 'pending',
    result            JSONB,
    error_message     TEXT,
    cost_usd          DECIMAL(5,2),
    created_at        TIMESTAMPTZ DEFAULT now(),
    completed_at      TIMESTAMPTZ,
    expires_at        TIMESTAMPTZ DEFAULT now() + interval '30 days'
);

CREATE INDEX IF NOT EXISTS idx_employment_verification_subject ON employment_verification_request(subject_id);
CREATE INDEX IF NOT EXISTS idx_employment_verification_status ON employment_verification_request(status);
