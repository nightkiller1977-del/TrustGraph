# Phase 1.5: Measurement and Enforcement Readiness

**Status:** In Progress  
**Branch:** worktree-phase-1.5-measurement  
**Duration:** 3-4 weeks  
**Timeline:** Parallel with Phase 2 planning  

## Objective

Validate Phase 1 (assessment accuracy) is acceptable before enforcement. Measure false-positive rate, false-negative rate, and appeal overturn rate. Only proceed to Phase 2 enforcement after calibration proves the system is reliable.

---

## Four Work Streams

### 1. Shadow Mode Implementation

**Goal:** Run assessments without restricting accounts

**Tasks:**
- [ ] Add `enforcement_enabled` feature flag (default: false)
  - File: `internal/config/config.go`
  - When false: assessments complete, audit logs decision, but trust tier is NOT enforced
  - ConnectionSphere always receives tier but respects own feature flag

- [ ] Implement assessment-only mode in handler
  - File: `internal/api/assessment_handler.go`
  - Still call policy engine, record observations, persist assessment
  - Still audit-log the decision (with `enforcement_enabled=false` marker)
  - Don't fail open; run complete assessment

- [ ] Add audit log marker for shadow mode
  - File: `internal/audit/events.go`
  - New field: `enforcement_mode: "shadow"` vs `"enforced"`
  - Allows analysis of shadow-mode outcomes

- [ ] Update ColdFusion client to respect flag
  - File: `connectionsphere/internal/services/TrustGraphService.cfc`
  - Still receives assessment response
  - ConnectionSphere also has own enforcement flag
  - For Phase 1.5: both return tier but don't restrict

**Acceptance Criteria:**
- [ ] Assessments complete in shadow mode (no capability restrictions)
- [ ] Audit log clearly marks shadow-mode decisions
- [ ] 100+ assessments flow through with decisions logged

### 2. Manual Review Queue

**Goal:** Human reviewers inspect high-risk assessments

**Tasks:**
- [ ] Create admin dashboard
  - File: `internal/api/admin_handler.go` (new)
  - Endpoint: `GET /v1/admin/queue?risk_band=high&status=pending`
  - Returns: assessment_id, subject_id, risk_score, reason_codes, signals, observations
  - Paginates: 20 per page

- [ ] Review submission endpoint
  - File: `internal/api/admin_handler.go`
  - Endpoint: `POST /v1/admin/reviews/{assessmentId}`
  - Request: `{ reviewer_email, outcome, notes }`
  - Outcome: "confirmed_abuse", "legitimate", "inconclusive", "error"
  - Persists review to database

- [ ] Review database table
  - File: `migrations/002_review_system.sql` (new)
  - Table: `assessment_review`
  - Fields: review_id, assessment_id, reviewer_email, outcome, notes, created_at
  - Index on (assessment_id, created_at)

- [ ] Admin authorization
  - File: `internal/api/middleware.go`
  - Require `Authorization: Bearer <admin-token>`
  - Token from environment: `TRUSTGRAPH_ADMIN_TOKEN`
  - Log all admin access to audit_log

**Acceptance Criteria:**
- [ ] Admin can view queue of pending high-risk assessments
- [ ] Admin can submit review with outcome
- [ ] Reviews are persisted and immutable
- [ ] All admin access is audit-logged

### 3. Labeling and Calibration

**Goal:** Measure assessment accuracy against ground truth

**Tasks:**
- [ ] Add review outcome lookup
  - File: `internal/store/review_repo.go` (new)
  - Function: `GetReviewByAssessmentID(ctx, assessmentID) -> Review`
  - Used to label assessments with ground truth

- [ ] Calibration query
  - File: `internal/store/calibration.go` (new)
  - Queries:
    - `CountByRiskBandAndOutcome(riskBand, outcome)` → confusion matrix
    - `FalsePositiveRate()` → legitimate accounts flagged as high-risk
    - `FalseNegativeRate()` → abuse accounts flagged as low-risk
    - `AppealOverturnRate()` → reviews reversed by second reviewer
    - `PerformanceByReasonCode(reasonCode)` → which signals predict abuse?

- [ ] Calibration dashboard
  - File: `internal/api/metrics_handler.go` (new)
  - Endpoint: `GET /v1/metrics/calibration`
  - Response:
    ```json
    {
      "total_reviews": 150,
      "false_positive_rate": 0.08,
      "false_negative_rate": 0.12,
      "appeal_overturn_rate": 0.03,
      "by_risk_band": {
        "high": {"reviews": 45, "abuse_rate": 0.89},
        "elevated": {"reviews": 38, "abuse_rate": 0.45},
        "low": {"reviews": 67, "abuse_rate": 0.02}
      },
      "by_reason_code": {
        "DISPOSABLE_EMAIL": {"precision": 0.76, "recall": 0.62},
        "HIGH_REGISTRATION_VELOCITY": {"precision": 0.91, "recall": 0.48}
      }
    }
    ```

- [ ] Export data for analysis
  - File: `cmd/trustgraph-metrics/main.go` (new)
  - CLI tool: `go run ./cmd/trustgraph-metrics export --format csv --output metrics.csv`
  - Exports: assessment_id, trust_tier, risk_score, reason_codes, review_outcome, created_at

**Acceptance Criteria:**
- [ ] Calibration endpoint returns accuracy metrics
- [ ] Confusion matrix calculated correctly
- [ ] Metrics exported to CSV for analysis
- [ ] Dashboard updated daily from assessments + reviews

### 4. Policy Simulation and Appeal Workflow

**Goal:** Test policy changes before enforcement; establish appeals process

**Tasks:**
- [ ] Policy simulation engine
  - File: `internal/policy/simulation.go` (new)
  - Function: `SimulatePolicy(ctx, assessments []Assessment, newPolicy Policy) -> Results`
  - Takes 100+ assessments with known outcomes
  - Simulates what would happen with different thresholds
  - Output: new FP rate, FN rate, etc.

- [ ] Threshold testing
  - File: `cmd/trustgraph-simulate/main.go` (new)
  - CLI: `go run ./cmd/trustgraph-simulate --risk-threshold 60 --output report.json`
  - Tests thresholds 50, 60, 70, 80
  - Reports impact on FP/FN rates

- [ ] Appeal workflow database
  - File: `migrations/002_review_system.sql`
  - Table: `assessment_appeal`
  - Fields: appeal_id, assessment_id, user_message, reviewer_email, outcome, created_at
  - Allow one appeal per assessment

- [ ] Appeal submission endpoint (future)
  - File: `internal/api/appeal_handler.go` (stub for now)
  - Will be called by ConnectionSphere after enforcement begins

**Acceptance Criteria:**
- [ ] Can simulate policy with different thresholds
- [ ] Simulation accurately predicts FP/FN rates
- [ ] Appeal schema ready for Phase 2 enforcement

---

## Success Criteria (Gate to Phase 2)

Before enforcement is enabled, all of these must be met:

- [ ] **100+ assessments labeled** with ground-truth outcomes (reviewer confirmed abuse/legitimate)
- [ ] **False-positive rate < 10%** (legitimate accounts wrongly flagged)
- [ ] **False-negative rate < 20%** (actual abuse missed)
- [ ] **Appeal overturn rate < 5%** (reviewers consistent)
- [ ] **Per-reason-code analysis** showing which signals predict abuse vs. FP
- [ ] **Policy simulation** demonstrates enforcement at different thresholds
- [ ] **No critical audit gaps** (all decisions logged, all reviews attributed)
- [ ] **Admin dashboard working** (queue visible, reviews submittable)
- [ ] **Performance acceptable** (metrics query sub-second)
- [ ] **Enforcement flag ready** to toggle on/off

---

## Week-by-Week Plan

### Week 1: Shadow Mode + Admin Dashboard
- [ ] Feature flag implementation
- [ ] Admin handler (queue + review submission)
- [ ] Review database table and queries
- [ ] Deploy to staging, begin collecting assessments

### Week 2: Labeling and Calibration  
- [ ] Manual review process (train reviewers)
- [ ] Calibration queries
- [ ] Confusion matrix calculation
- [ ] 50-75 assessments reviewed

### Week 3: Analysis + Policy Simulation
- [ ] Analyze results: FP/FN rates, per-signal performance
- [ ] Policy simulation engine
- [ ] Threshold testing
- [ ] 100+ assessments labeled, metrics stable
- [ ] Stakeholder review of results

### Week 4: Appeal Workflow + Enforcement Readiness
- [ ] Appeal database schema
- [ ] Enforcement flag tested (shadow ↔ enforced)
- [ ] Legal/compliance review of labeling process
- [ ] Sign-off from: Trust & Safety, Legal, Product
- [ ] Go/no-go decision on Phase 2 enforcement

---

## Implementation Order

1. **First:** Shadow mode (no enforcement) + audit log marker
2. **Parallel:** Admin dashboard + review submission
3. **Parallel:** Review database + calibration queries
4. **Third:** Manual review process (collect 50+ labels)
5. **Fourth:** Analysis + simulation
6. **Fifth:** Appeal workflow (scaffold for Phase 2)
7. **Final:** Enforcement flag toggle + go/no-go decision

---

## Database Changes

**Migration: 002_review_system.sql**
```sql
-- Assessment review (human reviewer outcome)
CREATE TABLE assessment_review (
    review_id UUID PRIMARY KEY,
    assessment_id UUID UNIQUE NOT NULL REFERENCES assessment(assessment_id),
    reviewer_email VARCHAR(255) NOT NULL,
    outcome VARCHAR(50) NOT NULL,  -- confirmed_abuse, legitimate, inconclusive, error
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Appeal (user can appeal limited tier)
CREATE TABLE assessment_appeal (
    appeal_id UUID PRIMARY KEY,
    assessment_id UUID UNIQUE NOT NULL REFERENCES assessment(assessment_id),
    user_message TEXT,
    reviewer_email VARCHAR(255),
    outcome VARCHAR(50),  -- pending, approved, rejected
    created_at TIMESTAMPTZ DEFAULT now(),
    reviewed_at TIMESTAMPTZ
);
```

---

## Files to Create

- `cmd/trustgraph-metrics/main.go` — Metrics export CLI
- `cmd/trustgraph-simulate/main.go` — Policy simulation CLI
- `internal/api/admin_handler.go` — Admin dashboard endpoints
- `internal/api/appeal_handler.go` — Appeal workflow (stub)
- `internal/api/metrics_handler.go` — Calibration metrics endpoint
- `internal/store/review_repo.go` — Review database queries
- `internal/store/appeal_repo.go` — Appeal database queries
- `internal/store/calibration.go` — Calibration metric queries
- `internal/policy/simulation.go` — Policy simulation engine
- `migrations/002_review_system.sql` — Review + appeal tables

---

## Files to Modify

- `internal/config/config.go` — Add `enforcement_enabled` flag
- `internal/api/assessment_handler.go` — Respect enforcement flag
- `internal/audit/events.go` — Mark shadow-mode decisions
- `internal/api/middleware.go` — Admin authorization
- `.env.example` — Add `TRUSTGRAPH_ADMIN_TOKEN`, `TRUSTGRAPH_ENFORCEMENT_ENABLED`

---

## Rollout Plan

**Shadow Mode (Week 1-3):**
- Assessments run, decisions logged
- No capability restrictions yet
- Manual review running in parallel
- Metrics accumulating

**Measurement (Week 2-4):**
- Calibration metrics calculated daily
- Stakeholder review of results
- Policy simulation tested
- Go/no-go decision made

**Phase 2 Enforcement (After sign-off):**
- Set `TRUSTGRAPH_ENFORCEMENT_ENABLED=true`
- ConnectionSphere begins restricting capabilities
- Manual review becomes appeal handling
- Metrics continue monitoring (Phase 3+)

---

## Blockers and Dependencies

- **Requires Phase 1 running** ✅ (already live)
- **Requires Phase 0 sign-off** for enforcement to begin (pending legal/policy)
- **Requires >100 assessments** flowing through system (shadow mode)
- **Requires manual reviewer time** (2-3 hours/week minimum)

---

## Rollback Plan

If metrics show unacceptable FP/FN rates:

1. Set `TRUSTGRAPH_ENFORCEMENT_ENABLED=false`
2. Pause Phase 2 start
3. Iterate on policy engine:
   - Adjust signal weights (email, device, velocity, image)
   - Tune hard-block rules
   - Add new signals (behavioral, velocity refinement)
4. Re-collect 100+ assessments
5. Re-measure calibration metrics
6. Retry Phase 2 decision

---

**Target:** Complete measurement by end of week 4, decision on Phase 2 enforcement by end of week 5.
