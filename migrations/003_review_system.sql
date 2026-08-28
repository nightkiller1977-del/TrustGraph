-- Phase 1.5: human-review and appeal tables

CREATE TABLE assessment_review (
    review_id      UUID PRIMARY KEY,
    assessment_id  UUID UNIQUE NOT NULL REFERENCES assessment(assessment_id),
    reviewer_email VARCHAR(255) NOT NULL,
    outcome        VARCHAR(50)  NOT NULL CHECK (outcome IN ('confirmed_abuse','legitimate','inconclusive','error')),
    notes          TEXT,
    created_at     TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_assessment_review_assessment ON assessment_review(assessment_id);
CREATE INDEX idx_assessment_review_outcome    ON assessment_review(outcome);

CREATE TABLE assessment_appeal (
    appeal_id      UUID PRIMARY KEY,
    assessment_id  UUID UNIQUE NOT NULL REFERENCES assessment(assessment_id),
    user_message   TEXT,
    reviewer_email VARCHAR(255),
    outcome        VARCHAR(50) DEFAULT 'pending' CHECK (outcome IN ('pending','approved','rejected')),
    created_at     TIMESTAMPTZ DEFAULT now(),
    reviewed_at    TIMESTAMPTZ
);

CREATE INDEX idx_assessment_appeal_assessment ON assessment_appeal(assessment_id);
CREATE INDEX idx_assessment_appeal_outcome    ON assessment_appeal(outcome);
