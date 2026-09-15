-- Phase 1 Plane B: verification infrastructure
--
-- subject_consent and verification_token already exist from migration 001
-- (created as unused Phase 1 scaffolding) and were extended by 002. This
-- migration adds what the consent + verification APIs need, plus the
-- vendor-backed verification tables for government ID, liveness, and image
-- checks.

-- verification_token gains the fields the verification endpoints write:
-- cost tracking, a human-readable failure reason, and an updated_at stamp.
-- vendor_reference_id / result / status / completed_at already exist.
ALTER TABLE verification_token
    ADD COLUMN IF NOT EXISTS cost_usd DECIMAL(6,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS error_message TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_verification_status ON verification_token(status);
CREATE INDEX IF NOT EXISTS idx_verification_type ON verification_token(verification_type);

-- Government ID verification. Deliberately stores only the derived attributes
-- the product needs (an age gate needs a date of birth, a badge needs a
-- verified name) plus the vendor's opaque reference. The document image itself
-- is never persisted here — it lives with the vendor and is referenced only.
CREATE TABLE IF NOT EXISTS government_id_verification (
    verification_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id       UUID NOT NULL REFERENCES subject(subject_id),
    consent_id       UUID REFERENCES subject_consent(consent_id),
    vendor           VARCHAR(100),
    vendor_reference_id VARCHAR(255),
    status           VARCHAR(50) DEFAULT 'pending',
    id_type          VARCHAR(50),
    verified_name    VARCHAR(255),
    date_of_birth    DATE,
    address_line     VARCHAR(255),
    country_code     VARCHAR(2),
    failure_reason   TEXT,
    cost_usd         DECIMAL(6,2) DEFAULT 0,
    created_at       TIMESTAMPTZ DEFAULT now(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_gov_id_subject ON government_id_verification(subject_id);
CREATE INDEX IF NOT EXISTS idx_gov_id_status ON government_id_verification(status);

-- Liveness verification (proof of person). Links to the government ID check
-- when both ran so an investigator can confirm they matched the same person.
CREATE TABLE IF NOT EXISTS liveness_verification (
    verification_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id       UUID NOT NULL REFERENCES subject(subject_id),
    consent_id       UUID REFERENCES subject_consent(consent_id),
    government_id_verification_id UUID REFERENCES government_id_verification(verification_id),
    vendor           VARCHAR(100),
    vendor_reference_id VARCHAR(255),
    status           VARCHAR(50) DEFAULT 'pending',
    liveness_score   NUMERIC(4,3),
    matched_identity BOOLEAN,
    failure_reason   TEXT,
    cost_usd         DECIMAL(6,2) DEFAULT 0,
    created_at       TIMESTAMPTZ DEFAULT now(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_liveness_subject ON liveness_verification(subject_id);
CREATE INDEX IF NOT EXISTS idx_liveness_status ON liveness_verification(status);

-- Image verification: reverse-image search and synthetic-image detection.
-- Both checks run against the same submitted image, so one row records both
-- outcomes rather than forcing callers to join two tables.
CREATE TABLE IF NOT EXISTS image_verification (
    verification_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id       UUID NOT NULL REFERENCES subject(subject_id),
    consent_id       UUID REFERENCES subject_consent(consent_id),
    image_hash       VARCHAR(64),
    status           VARCHAR(50) DEFAULT 'pending',
    -- Reverse image search
    reverse_search_provider VARCHAR(100),
    match_count      INTEGER DEFAULT 0,
    matches          JSONB,
    -- Synthetic image detection
    synthetic_provider VARCHAR(100),
    synthetic_score  NUMERIC(4,3),
    is_synthetic     BOOLEAN,
    failure_reason   TEXT,
    cost_usd         DECIMAL(6,2) DEFAULT 0,
    created_at       TIMESTAMPTZ DEFAULT now(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_image_verification_subject ON image_verification(subject_id);
CREATE INDEX IF NOT EXISTS idx_image_verification_hash ON image_verification(image_hash);

-- Subject-level verification flags, so capability gating and badge rendering
-- do not need to scan the verification tables on every read.
ALTER TABLE subject
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS has_government_id BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS has_liveness BOOLEAN DEFAULT false;
