-- Plane B: persistent minimum-age restriction.
--
-- The registration-time age gate only checks a self-reported date of birth.
-- When the government-ID vendor later returns an authoritative DOB that shows
-- the subject is a minor, that finding has to outlive the request: the identity
-- check itself still succeeded, so the verification row remains, but the
-- age-gate restriction is recorded on the subject so it can be enforced by
-- capability gating and surfaced in the status/badge reads.

ALTER TABLE subject
    ADD COLUMN IF NOT EXISTS age_blocked BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS age_blocked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS age_blocked_source VARCHAR(50);