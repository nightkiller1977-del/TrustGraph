-- Phase 1: Initial schema with Plane A (first-party signals) support
-- Database: trustgraph (created in ConnectionSphere's PostgreSQL 17 instance)

-- Subject table (maps to ConnectionSphere user)
CREATE TABLE IF NOT EXISTS subject (
    subject_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connection_sphere_user_id VARCHAR(255) UNIQUE NOT NULL,
    external_id_verified BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_subject_cs_user ON subject(connection_sphere_user_id);

-- Assessment table (trust decision)
CREATE TABLE IF NOT EXISTS assessment (
    assessment_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_version VARCHAR(10) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    subject_id UUID NOT NULL REFERENCES subject(subject_id),
    assessment_type VARCHAR(50),
    trust_tier VARCHAR(50) DEFAULT 'provisional',
    risk_band VARCHAR(50),
    risk_score INTEGER,
    decision VARCHAR(50),
    reason_codes TEXT[],
    required_actions TEXT[],
    policy_version VARCHAR(50),
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT risk_score_range CHECK (risk_score >= 0 AND risk_score <= 100)
);

CREATE INDEX idx_assessment_subject ON assessment(subject_id);
-- Not a UNIQUE index: idempotency is enforced app-side, within a rolling
-- TTL window (assessment_handler.go's GetAssessmentByIdempotencyKey +
-- cfg.IdempotencyTTLHours) — a key is legitimately reusable once its
-- window has passed. A true rolling-window uniqueness constraint isn't
-- expressible as a static Postgres index (predicates must be IMMUTABLE,
-- and now() is only STABLE), and a permanent UNIQUE index would silently
-- break that reuse. This index exists purely to make the TTL lookup's
-- WHERE idempotency_key = $1 fast. The race between the TTL check and the
-- insert is closed at the application layer instead, by
-- AssessmentRepository.CreateAssessmentIfAbsent's transaction-scoped
-- pg_advisory_xact_lock.
CREATE INDEX idx_assessment_idempotency ON assessment(idempotency_key);
CREATE INDEX idx_assessment_created ON assessment(created_at DESC);

-- Observation table (raw signals)
CREATE TABLE IF NOT EXISTS observation (
    observation_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id UUID REFERENCES assessment(assessment_id),
    subject_id UUID NOT NULL REFERENCES subject(subject_id),
    observation_type VARCHAR(50),
    plane VARCHAR(10),
    source VARCHAR(100),
    source_data JSONB,
    confidence NUMERIC(3,2),
    created_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_observation_subject ON observation(subject_id);
CREATE INDEX idx_observation_assessment ON observation(assessment_id);
CREATE INDEX idx_observation_created ON observation(created_at DESC);

-- Evidence table (detailed signal data)
CREATE TABLE IF NOT EXISTS evidence (
    evidence_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    observation_id UUID REFERENCES observation(observation_id),
    plane VARCHAR(10),
    evidence_type VARCHAR(50),
    content_type VARCHAR(100),
    object_storage_path VARCHAR(1024),
    content_inline TEXT,
    hash_sha256 VARCHAR(64),
    created_at TIMESTAMPTZ DEFAULT now(),
    accessed_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_evidence_observation ON evidence(observation_id);
CREATE INDEX idx_evidence_created ON evidence(created_at DESC);

-- Signal source configuration
CREATE TABLE IF NOT EXISTS signal_source (
    signal_source_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_name VARCHAR(100),
    provider VARCHAR(100),
    is_first_party BOOLEAN,
    timeout_ms INTEGER,
    circuit_breaker_enabled BOOLEAN,
    max_failures INTEGER DEFAULT 5,
    failure_window_minutes INTEGER DEFAULT 5,
    failed_count INTEGER DEFAULT 0,
    last_failure_at TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Audit log (all actions)
CREATE TABLE IF NOT EXISTS audit_log (
    audit_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plane VARCHAR(10),
    action VARCHAR(100),
    actor VARCHAR(255),
    actor_type VARCHAR(50),
    resource_type VARCHAR(100),
    resource_id UUID,
    subject_id UUID,
    details JSONB,
    result VARCHAR(50),
    error_message TEXT,
    request_id VARCHAR(255),
    ip_address INET,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL
);

CREATE INDEX idx_audit_subject ON audit_log(subject_id);
CREATE INDEX idx_audit_action ON audit_log(action);
CREATE INDEX idx_audit_created ON audit_log(created_at DESC);

-- Subject consent (scaffold for Plane B, unused in Phase 1)
CREATE TABLE IF NOT EXISTS subject_consent (
    consent_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL UNIQUE REFERENCES subject(subject_id),
    plane VARCHAR(10),
    consent_type VARCHAR(50),
    consent_status VARCHAR(50),
    policy_version VARCHAR(50),
    granted_at TIMESTAMPTZ,
    withdrawn_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    terms_accepted JSONB,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_consent_subject ON subject_consent(subject_id);

-- Verification token (scaffold for Plane B, unused in Phase 1)
CREATE TABLE IF NOT EXISTS verification_token (
    token_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_id UUID NOT NULL REFERENCES subject(subject_id),
    consent_id UUID REFERENCES subject_consent(consent_id),
    verification_type VARCHAR(50),
    vendor VARCHAR(100),
    vendor_reference_id VARCHAR(255),
    status VARCHAR(50),
    result JSONB,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_verification_subject ON verification_token(subject_id);
CREATE INDEX idx_verification_consent ON verification_token(consent_id);
