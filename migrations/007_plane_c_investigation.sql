-- Phase 1 Plane C: investigation case management, OSINT tooling audit, and
-- break-glass access.
--
-- Plane C is the highest-privilege part of the system: it lets a vetted
-- investigator pull together a subject's signals, run OSINT tools, and record
-- findings. Every read and tool call is separately audited (see the
-- investigation_tool_query / investigation_access_log tables below), because a
-- generic "someone viewed a case" audit entry is not enough to answer "who ran
-- which OSINT query against which subject, and why".

CREATE TABLE IF NOT EXISTS investigation_case (
    case_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_number    VARCHAR(32) UNIQUE NOT NULL,
    subject_id     UUID REFERENCES subject(subject_id),
    title          VARCHAR(255) NOT NULL,
    description    TEXT,
    -- open | investigating | escalated | resolved | closed
    status         VARCHAR(50) NOT NULL DEFAULT 'open',
    priority       VARCHAR(20) NOT NULL DEFAULT 'normal',
    -- Which policy/compliance reason this case exists (e.g. fraud report id).
    trigger_type   VARCHAR(50),
    trigger_ref    VARCHAR(255),
    assigned_to    VARCHAR(255),
    created_by     VARCHAR(255) NOT NULL,
    findings       TEXT,
    resolution     VARCHAR(100),
    opened_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at      TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT investigation_case_status_check CHECK (
        status IN ('open', 'investigating', 'escalated', 'resolved', 'closed')
    )
);

CREATE INDEX IF NOT EXISTS idx_investigation_case_subject ON investigation_case(subject_id);
CREATE INDEX IF NOT EXISTS idx_investigation_case_status ON investigation_case(status);
CREATE INDEX IF NOT EXISTS idx_investigation_case_assigned ON investigation_case(assigned_to);

-- Case notes: the running record of what an investigator observed and did.
CREATE TABLE IF NOT EXISTS investigation_note (
    note_id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id     UUID NOT NULL REFERENCES investigation_case(case_id) ON DELETE CASCADE,
    author      VARCHAR(255) NOT NULL,
    author_role VARCHAR(50),
    note_type   VARCHAR(50) DEFAULT 'note',
    content     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_investigation_note_case ON investigation_note(case_id);

-- One row per OSINT tool invocation. Holds enough to reconstruct the query
-- (tool + params + result summary) without storing raw harvested PII.
CREATE TABLE IF NOT EXISTS investigation_tool_query (
    query_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id        UUID REFERENCES investigation_case(case_id) ON DELETE SET NULL,
    subject_id     UUID REFERENCES subject(subject_id),
    tool_name      VARCHAR(100) NOT NULL,
    query_params   JSONB,
    result_summary JSONB,
    result_count   INTEGER,
    status         VARCHAR(50) NOT NULL DEFAULT 'success',
    error_message  TEXT,
    cost_usd       DECIMAL(6,2) DEFAULT 0,
    duration_ms    INTEGER,
    actor          VARCHAR(255) NOT NULL,
    actor_role     VARCHAR(50),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_investigation_tool_case ON investigation_tool_query(case_id);
CREATE INDEX IF NOT EXISTS idx_investigation_tool_name ON investigation_tool_query(tool_name);
CREATE INDEX IF NOT EXISTS idx_investigation_tool_subject ON investigation_tool_query(subject_id);

-- Break-glass access: an investigator can request elevated access in an
-- emergency, but the grant is time-boxed, requires a justification, and is
-- always flagged for review.
CREATE TABLE IF NOT EXISTS investigation_break_glass (
    grant_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor         VARCHAR(255) NOT NULL,
    actor_role    VARCHAR(50) NOT NULL,
    subject_id    UUID REFERENCES subject(subject_id),
    case_id       UUID REFERENCES investigation_case(case_id),
    justification TEXT NOT NULL,
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked_at    TIMESTAMPTZ,
    reviewed_by   VARCHAR(255),
    reviewed_at   TIMESTAMPTZ,
    review_outcome VARCHAR(50)
);

CREATE INDEX IF NOT EXISTS idx_break_glass_actor ON investigation_break_glass(actor);
CREATE INDEX IF NOT EXISTS idx_break_glass_expires ON investigation_break_glass(expires_at);

-- Every Plane C data access, separate from the general audit log so access
-- reports can be produced without scanning all audit rows.
CREATE TABLE IF NOT EXISTS investigation_access_log (
    access_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id      UUID REFERENCES investigation_case(case_id) ON DELETE SET NULL,
    subject_id   UUID REFERENCES subject(subject_id),
    actor        VARCHAR(255) NOT NULL,
    actor_role   VARCHAR(50),
    action       VARCHAR(100) NOT NULL,
    resource     VARCHAR(100),
    resource_ref VARCHAR(255),
    granted      BOOLEAN NOT NULL DEFAULT true,
    reason       TEXT,
    break_glass  BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_investigation_access_case ON investigation_access_log(case_id);
CREATE INDEX IF NOT EXISTS idx_investigation_access_actor ON investigation_access_log(actor);
CREATE INDEX IF NOT EXISTS idx_investigation_access_break_glass ON investigation_access_log(break_glass);
