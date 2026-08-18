-- Phase 1 Plane B: Education Verification Schema
-- Extends the existing schema to support consented education verification

-- Subject education (data from LinkedIn OAuth)
CREATE TABLE IF NOT EXISTS subject_education (
    education_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL UNIQUE REFERENCES subject(subject_id),
    school_name VARCHAR(255),
    field_of_study VARCHAR(255),
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    -- The Go model exposes an unrestricted string (GPAs, but also
    -- classifications like "First Class Honours" or "Summa Cum Laude"),
    -- so this can't be bounded to short numeric-GPA-sized values.
    grade TEXT,

    -- Validation results (from free validation logic)
    confidence_score INTEGER,  -- 0-100
    is_verified BOOLEAN,
    validation_signals TEXT[],  -- ['TIMELINE_PLAUSIBLE', 'KNOWN_UNIVERSITY', ...]
    validation_details TEXT,
    validation_risk_score INTEGER,  -- 0-100 (inverted: low = low risk)

    -- Audit fields
    source VARCHAR(50),  -- 'linkedin_oauth', 'manual_input', 'government_id'
    source_data JSONB,  -- Raw OAuth response
    validated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ  -- Education data can expire (diploma revoked, etc.)
);

CREATE INDEX idx_subject_education_subject ON subject_education(subject_id);
CREATE INDEX idx_subject_education_verified ON subject_education(is_verified);
CREATE INDEX idx_subject_education_created ON subject_education(created_at DESC);

-- Education verification status tracking (for future paid verification)
CREATE TABLE IF NOT EXISTS education_verification_request (
    request_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES subject(subject_id),
    education_id UUID REFERENCES subject_education(education_id),

    verification_type VARCHAR(50),  -- 'free_validation', 'persona', 'onfido', 'truework'
    vendor VARCHAR(100),
    vendor_request_id VARCHAR(255),

    status VARCHAR(50) DEFAULT 'pending',  -- pending, processing, verified, failed, rejected
    result JSONB,  -- Vendor response (minimal PII)
    error_message TEXT,

    cost_usd DECIMAL(5,2),  -- 0 for free, 1-3 for paid

    created_at TIMESTAMPTZ DEFAULT now(),
    completed_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ DEFAULT now() + interval '30 days'
);

CREATE INDEX idx_education_verification_subject ON education_verification_request(subject_id);
CREATE INDEX idx_education_verification_status ON education_verification_request(status);
CREATE INDEX idx_education_verification_created ON education_verification_request(created_at DESC);

-- Update assessment table to include education verification badge
-- (optional, if you want education confidence included in risk scoring)
ALTER TABLE assessment
ADD COLUMN IF NOT EXISTS education_confidence_score INTEGER,
ADD COLUMN IF NOT EXISTS education_verified BOOLEAN;

-- Consent tracking for Plane B data collection.
-- subject_consent and idx_consent_subject already exist from migration 001
-- (scaffold, unused in Phase 1) with a single-row-per-subject UNIQUE(subject_id).
-- Recreating the table here is a no-op at best; recreating the index is a
-- hard "relation already exists" failure that rolls back this entire
-- migration and terminates API startup. Alter the existing table instead:
-- widen uniqueness to (subject_id, plane, consent_type) so a subject can
-- hold one consent record per plane/type combination rather than exactly
-- one ever, and add the columns/indexes Plane B needs. No Go code
-- references subject_consent yet, so relaxing this constraint is safe.
ALTER TABLE subject_consent
    ADD COLUMN IF NOT EXISTS ip_address INET,
    ADD COLUMN IF NOT EXISTS user_agent TEXT;

ALTER TABLE subject_consent DROP CONSTRAINT IF EXISTS subject_consent_subject_id_key;
ALTER TABLE subject_consent ADD CONSTRAINT subject_consent_subject_plane_type_key UNIQUE (subject_id, plane, consent_type);

CREATE INDEX IF NOT EXISTS idx_consent_plane ON subject_consent(plane);
CREATE INDEX IF NOT EXISTS idx_consent_status ON subject_consent(consent_status);
