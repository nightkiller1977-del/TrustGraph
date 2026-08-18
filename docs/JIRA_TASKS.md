# TrustGraph Phase 1 - JIRA Tasks & Epics

## Epic: Plane B - Consented Verification (Phase 1)

**Epic Key:** TG-PLANE-B  
**Timeline:** 7 weeks (Weeks 9-15 of Phase 1)  
**Team:** 2 engineers  
**Status:** READY TO START  

---

## Sprint 1: LinkedIn OAuth Integration (Week 9-10)

### Story: TG-LI-001 - LinkedIn OAuth Flow Setup

**Type:** Story  
**Points:** 8  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 9  

**Description:**
Implement OAuth 2.0 flow for LinkedIn sign-in, capturing employment and education data.

**Acceptance Criteria:**
- [ ] LinkedIn app created and configured
- [ ] OAuth callback handler implemented
- [ ] Access token stored securely
- [ ] Consent screen shows what data is requested
- [ ] Error handling for rejected permissions
- [ ] Can refresh expired tokens

**Subtasks:**
- [ ] Design OAuth callback handler (TG-LI-001-1)
- [ ] Implement LinkedIn scopes (profile, email, public_profile) (TG-LI-001-2)
- [ ] Add token storage to database (TG-LI-001-3)
- [ ] Create UI flow for "Link LinkedIn" button (TG-LI-001-4)
- [ ] Add error handling for permission denial (TG-LI-001-5)

**Related Files:**
- `internal/api/oauth_handler.go` (new)
- `migrations/003_linkedin_integration.sql` (new)

---

### Story: TG-LI-002 - LinkedIn Data Extraction & Storage

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** Mid Week 9  

**Description:**
Extract employment and education data from LinkedIn OAuth response. Store in database.

**Acceptance Criteria:**
- [ ] Parse LinkedIn profile API response
- [ ] Extract job history, titles, companies, dates
- [ ] Extract education, schools, degrees, years
- [ ] Store in subject_employment table
- [ ] Store in subject_education table
- [ ] Handle missing/incomplete data gracefully

**Subtasks:**
- [ ] Create LinkedIn data model (TG-LI-002-1)
- [ ] Parse employment array from response (TG-LI-002-2)
- [ ] Parse education array from response (TG-LI-002-3)
- [ ] Implement data persistence (TG-LI-002-4)
- [ ] Add data validation (TG-LI-002-5)

**Related Files:**
- `internal/models/linkedin.go` (new)
- `internal/store/linkedin_repo.go` (new)

---

### Story: TG-LI-003 - Employment Validator (Free Validation)

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 9  

**Description:**
Build free employment validation (similar to education validator).

**Acceptance Criteria:**
- [ ] Validate employment timeline (start before current job ends)
- [ ] Check company existence (basic)
- [ ] Match job title to career field
- [ ] Weight by tenure (longer employment = more trustworthy)
- [ ] Generate confidence score (0-100)
- [ ] Identify verified vs. self-reported

**Subtasks:**
- [ ] Create EmploymentValidator struct (TG-LI-003-1)
- [ ] Implement timeline validation (TG-LI-003-2)
- [ ] Add company verification logic (TG-LI-003-3)
- [ ] Implement job-education alignment (TG-LI-003-4)
- [ ] Create confidence scoring (TG-LI-003-5)
- [ ] Write unit tests (min 20 cases) (TG-LI-003-6)

**Related Files:**
- `internal/verification/employment_validator.go` (new)
- `internal/verification/employment_validator_test.go` (new)

---

### Story: TG-LI-004 - Employment Signal Provider

**Type:** Story  
**Points:** 3  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 9  

**Description:**
Create employment signal for assessment pipeline. Contribute to risk scoring.

**Acceptance Criteria:**
- [ ] Implement EmploymentProvider signal
- [ ] Calculate score from employment confidence
- [ ] Generate reason codes (EMPLOYMENT_VERIFIED, EMPLOYMENT_DURATION_ESTABLISHED, etc.)
- [ ] Integrate with policy engine
- [ ] Score range: 0-30 points

**Subtasks:**
- [ ] Create signal provider (TG-LI-004-1)
- [ ] Map validation signals to reason codes (TG-LI-004-2)
- [ ] Add to assessment evaluator (TG-LI-004-3)
- [ ] Test integration with policy engine (TG-LI-004-4)

**Related Files:**
- `internal/signals/employment_plane_b.go` (new)

---

### Story: TG-LI-005 - LinkedIn Integration Testing

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** End of Week 10  

**Description:**
E2E testing of LinkedIn OAuth flow with real profiles.

**Acceptance Criteria:**
- [ ] Test OAuth callback with real LinkedIn account
- [ ] Verify all employment data extracted correctly
- [ ] Test with incomplete profiles (missing jobs, education)
- [ ] Test with old job history (10+ years back)
- [ ] Verify data stored correctly in database
- [ ] Test permission denial flow
- [ ] Test token refresh

**Subtasks:**
- [ ] Create test LinkedIn account (TG-LI-005-1)
- [ ] Test complete flow with full data (TG-LI-005-2)
- [ ] Test incomplete data handling (TG-LI-005-3)
- [ ] Test error scenarios (TG-LI-005-4)
- [ ] Load test with 1000 profiles (TG-LI-005-5)

---

### Story: TG-LI-006 - LinkedIn Badges & UI

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** End of Week 10  

**Description:**
Render employment verification badges on user profile.

**Acceptance Criteria:**
- [ ] Show "✅ Google" badge if verified
- [ ] Show "📼 Google" badge if self-reported
- [ ] Display job title and tenure
- [ ] Link to LinkedIn profile (optional)
- [ ] Render on both mobile and web
- [ ] Show to other users (verified status)

**Subtasks:**
- [ ] Design badge component (TG-LI-006-1)
- [ ] Implement profile UI updates (TG-LI-006-2)
- [ ] Add mobile view (TG-LI-006-3)
- [ ] Add privacy controls (hide if user wants) (TG-LI-006-4)

---

---

## Sprint 2: Government ID Verification (Week 11-12)

### Story: TG-ID-001 - Persona API Integration

**Type:** Story  
**Points:** 8  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** End of Week 11  

**Description:**
Integrate Persona API for government ID verification and age gate.

**Acceptance Criteria:**
- [ ] Persona account created and API key configured
- [ ] Document upload flow implemented
- [ ] Selfie/liveness verification works
- [ ] Age extracted from ID
- [ ] Verified name stored
- [ ] Vendor response logged (PII stripped)
- [ ] Error handling for failed verifications

**Subtasks:**
- [ ] Set up Persona account and credentials (TG-ID-001-1)
- [ ] Implement document upload endpoint (TG-ID-001-2)
- [ ] Add selfie capture UI (TG-ID-001-3)
- [ ] Parse Persona response (TG-ID-001-4)
- [ ] Store verification result (TG-ID-001-5)
- [ ] Add error handling (TG-ID-001-6)

**Related Files:**
- `internal/verification/id_verifier.go` (new)
- `migrations/004_id_verification.sql` (new)

---

### Story: TG-ID-002 - Age Gate Implementation

**Type:** Story  
**Points:** 3  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** Mid Week 11  

**Description:**
Enforce age gate in assessment. Block users under 18.

**Acceptance Criteria:**
- [ ] Check DOB from government ID verification
- [ ] Calculate age at assessment time
- [ ] Block assessment if age < 18
- [ ] Return specific error code (UNDERAGE_USER)
- [ ] Log underage attempts
- [ ] Alert admin to potential child protection issue

**Subtasks:**
- [ ] Create age gate policy rule (TG-ID-002-1)
- [ ] Implement in assessment handler (TG-ID-002-2)
- [ ] Add logging and alerts (TG-ID-002-3)
- [ ] Test with various birthdates (TG-ID-002-4)

**Related Files:**
- `internal/policy/age_gate.go` (new)

---

### Story: TG-ID-003 - Government ID Verification Testing

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** End of Week 12  

**Description:**
Test ID verification with real documents (test accounts from Persona).

**Acceptance Criteria:**
- [ ] Test with valid driver's license
- [ ] Test with valid passport
- [ ] Test with expired ID (should fail)
- [ ] Test with fake ID (should fail)
- [ ] Verify age extraction accurate
- [ ] Test age gate blocking minors
- [ ] Verify vendor cost tracking

**Subtasks:**
- [ ] Create test documents (TG-ID-003-1)
- [ ] Test valid ID flow (TG-ID-003-2)
- [ ] Test invalid ID rejection (TG-ID-003-3)
- [ ] Test age gate with test users (TG-ID-003-4)

---

---

## Sprint 3: Liveness & Image Verification (Week 13-14)

### Story: TG-LIV-001 - Liveness Check Integration

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 13  

**Description:**
Integrate liveness check (video proof user is real person).

**Acceptance Criteria:**
- [ ] Use Persona liveness or standalone
- [ ] Capture video selfie
- [ ] Verify match to government ID
- [ ] Store verification result
- [ ] Error handling for failed liveness
- [ ] Vendor cost tracking

---

### Story: TG-IMG-001 - Reverse Image Search Integration

**Type:** Story  
**Points:** 4  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** End of Week 13  

**Description:**
Check profile images against internet for catfishing detection.

**Acceptance Criteria:**
- [ ] Scan profile image via Google Images API
- [ ] Check TinEye for duplicates
- [ ] Flag if image found elsewhere online
- [ ] Alert user of duplicate detection
- [ ] Store results in database

---

### Story: TG-IMG-002 - Synthetic Image Detection

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 14  

**Description:**
Detect AI-generated/synthetic profile images.

**Acceptance Criteria:**
- [ ] Use free ML model (deepfakes detection)
- [ ] Flag suspicious images
- [ ] Score confidence (0-100)
- [ ] Alert user if synthetic detected
- [ ] Store detection results

---

---

## Sprint 4: Plane B Completion (Week 15)

### Story: TG-PB-001 - Consent Management

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** Mid Week 15  

**Description:**
Implement consent tracking for all Plane B verification types.

**Acceptance Criteria:**
- [ ] Track opt-in for LinkedIn
- [ ] Track opt-in for government ID
- [ ] Track opt-in for liveness
- [ ] Support withdrawal of consent
- [ ] GDPR compliance (right to deletion)
- [ ] Show consent screen before each verification

---

### Story: TG-PB-002 - Verification API Endpoints

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 15  

**Description:**
Create REST API endpoints for verification flows.

**Acceptance Criteria:**
- [ ] POST /v1/verification/linkedin/authorize
- [ ] GET /v1/verification/linkedin/callback
- [ ] POST /v1/verification/id/upload
- [ ] POST /v1/verification/liveness/upload
- [ ] GET /v1/verification/status
- [ ] DELETE /v1/verification/{type}
- [ ] GET /v1/user/profile/badges

**Related Files:**
- `internal/api/verification_handler.go` (new)

---

### Story: TG-PB-003 - Plane B Launch & Monitoring

**Type:** Story  
**Points:** 3  
**Status:** READY  
**Assignee:** [Both]  
**Due Date:** End of Week 15  

**Description:**
Deploy Plane B to production with monitoring.

**Acceptance Criteria:**
- [ ] Shadow deploy to 10% of users
- [ ] Monitor vendor latency (<500ms)
- [ ] Track verification success rate
- [ ] Monitor cost (per-user vendor spend)
- [ ] Create runbook for troubleshooting
- [ ] Full rollout to 100%

---

---

## Epic: Plane C - Investigation Tools (Phase 1)

**Epic Key:** TG-PLANE-C  
**Timeline:** 5-6 weeks (Weeks 16-21 of Phase 1)  
**Team:** 2 engineers  
**Status:** READY TO START (after Plane B)  

---

## Sprint 5: Investigator Auth & Case Management (Week 16-17)

### Story: TG-INV-001 - Investigator RBAC

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 16  

**Description:**
Implement role-based access control for investigators.

**Acceptance Criteria:**
- [ ] Create investigator role
- [ ] Define permissions (read_cases, open_case, access_tools, etc.)
- [ ] Enforce 2FA for investigator accounts
- [ ] Require strong password (12+ chars, special chars)
- [ ] Require signed data usage agreement
- [ ] Audit trail of all investigator actions

---

### Story: TG-INV-002 - Investigation Case Management

**Type:** Story  
**Points:** 8  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** End of Week 17  

**Description:**
Build case management system for investigations.

**Acceptance Criteria:**
- [ ] Create investigation case model
- [ ] Case status flow (open → assigned → resolved → archived)
- [ ] Assign case to investigator
- [ ] Track case metadata (reason, priority, created_at, resolved_at)
- [ ] Support case notes
- [ ] Support finding attachments (evidence)
- [ ] Case search and filtering
- [ ] Auto-archive after 90 days

**Subtasks:**
- [ ] Create case model (TG-INV-002-1)
- [ ] Create database schema (TG-INV-002-2)
- [ ] Implement CRUD operations (TG-INV-002-3)
- [ ] Add case status transitions (TG-INV-002-4)
- [ ] Implement case search (TG-INV-002-5)

**Related Files:**
- `internal/models/investigation_case.go` (new)
- `internal/store/investigation_repo.go` (new)
- `migrations/006_investigation_cases.sql` (new)

---

---

## Sprint 6: Investigation Tools (Week 18-19)

### Story: TG-OSINT-001 - Internet Archive Integration

**Type:** Story  
**Points:** 3  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** Mid Week 18  

**Description:**
Query Wayback Machine for historical profiles/websites.

**Acceptance Criteria:**
- [ ] Query Internet Archive API for snapshots
- [ ] Display historical versions
- [ ] Show when profile changed
- [ ] Detect suspicious changes

---

### Story: TG-OSINT-002 - Sherlock Integration (Username Enumeration)

**Type:** Story  
**Points:** 4  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 18  

**Description:**
Search username across 300+ social platforms.

**Acceptance Criteria:**
- [ ] Search username on major platforms
- [ ] Return found accounts with links
- [ ] Detect pattern of account creation
- [ ] Flag suspicious patterns (same username everywhere = catfisher)

---

### Story: TG-OSINT-003 - Email/Domain Tools (theHarvester)

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** Mid Week 19  

**Description:**
Find associated emails, domains, and breach data.

**Acceptance Criteria:**
- [ ] Query public email sources
- [ ] Find related domains
- [ ] Check breach databases (HaveIBeenPwned)
- [ ] Track social media presence
- [ ] Link to other identities

---

### Story: TG-OSINT-004 - SpiderFoot Integration

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** End of Week 19  

**Description:**
Aggregate OSINT findings from multiple sources.

**Acceptance Criteria:**
- [ ] Run SpiderFoot on email/domain
- [ ] Correlate findings across sources
- [ ] Detect linked profiles
- [ ] Generate investigation report
- [ ] Track confidence scores

---

---

## Sprint 7: Investigation APIs & Audit (Week 20-21)

### Story: TG-INV-API-001 - Investigation REST Endpoints

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** Mid Week 20  

**Description:**
Create API endpoints for investigator workflow.

**Acceptance Criteria:**
- [ ] POST /v1/investigations/case (create)
- [ ] GET /v1/investigations/case/{id} (view)
- [ ] PUT /v1/investigations/case/{id} (update)
- [ ] GET /v1/investigations/cases (list)
- [ ] POST /v1/investigations/{id}/archive (query tools)
- [ ] POST /v1/investigations/{id}/approve (approve user)
- [ ] POST /v1/investigations/{id}/suspend (suspend account)
- [ ] All endpoints require investigator auth

---

### Story: TG-INV-AUDIT-001 - Investigation Audit Logging

**Type:** Story  
**Points:** 5  
**Status:** READY  
**Assignee:** [Engineer B]  
**Due Date:** Mid Week 20  

**Description:**
Log all investigator actions for compliance and audit.

**Acceptance Criteria:**
- [ ] Log who accessed what
- [ ] Log when and why
- [ ] Log results/findings
- [ ] Support audit report generation
- [ ] Immutable log (cannot be deleted)
- [ ] HIPAA/GDPR compliant

---

### Story: TG-INV-ALERT-001 - Break-Glass Alerting

**Type:** Story  
**Points:** 3  
**Status:** READY  
**Assignee:** [Engineer A]  
**Due Date:** End of Week 21  

**Description:**
Alert on suspicious investigator activity.

**Acceptance Criteria:**
- [ ] Alert on self-lookups (investigator searching own info)
- [ ] Alert on bulk queries
- [ ] Alert on late-night access
- [ ] Alert on data export
- [ ] Escalate to admin for review

---

---

## Dependencies & Blockers

### Plane B Dependencies
- [ ] LinkedIn OAuth app created (external)
- [ ] Persona account & API key (external)
- [ ] Legal review of consent flow (Legal team)
- [ ] Privacy policy updated (Legal team)

### Plane C Dependencies
- [ ] Investigator training materials created
- [ ] Investigation playbook documented
- [ ] Legal review of investigator access (Legal team)
- [ ] Data retention policy defined (Legal team)

---

## Success Criteria (Launch Metrics)

### Plane B Launch Metrics
- [ ] 40% of users get at least one verification badge
- [ ] 25% link LinkedIn OAuth
- [ ] 15% verify government ID
- [ ] 10% complete liveness check
- [ ] 99.9% verification success rate (user completes flow)
- [ ] <500ms API latency
- [ ] Cost tracking within budget ($1-3/user)

### Plane C Launch Metrics
- [ ] 5-10% of assessments trigger investigation
- [ ] Investigation resolution <24 hours
- [ ] Investigator productivity 20-30 cases/day
- [ ] False positive rate <5%
- [ ] Zero investigator abuse incidents
- [ ] All actions audited and logged

---

## Risks & Mitigation

| Risk | Severity | Mitigation | Owner |
|------|----------|-----------|-------|
| Vendor outage (Persona) | HIGH | Multi-vendor support, fallback flow | [Eng Lead] |
| Privacy scandal | CRITICAL | Strong retention policies, encryption | [Legal] |
| Investigator abuse | CRITICAL | 2FA, audit logs, break-glass alerts | [Security] |
| Adoption too slow | MEDIUM | Incentivize verification (matching boost) | [Product] |
| Cost overrun | MEDIUM | Cap vendor spending, monitor usage | [Finance] |

---

## Timeline Summary

```
Sprint 1 (Week 9-10):   LinkedIn OAuth Integration
Sprint 2 (Week 11-12):  Government ID + Age Gate
Sprint 3 (Week 13-14):  Liveness + Image Verification
Sprint 4 (Week 15):     Plane B Launch (Consent, APIs)
────────────────────────────────────
Sprint 5 (Week 16-17):  Investigator Auth + Cases
Sprint 6 (Week 18-19):  Investigation Tools
Sprint 7 (Week 20-21):  Investigation APIs + Audit
────────────────────────────────────
TOTAL: 13 weeks (9 sprints)
```

---

## How to Import into JIRA

### Format for JIRA Import

Copy/paste into JIRA's bulk import:

```
Project Key,Issue Type,Summary,Description,Points,Status,Assignee,Due Date
TG,Epic,Plane B - Consented Verification,Implement LinkedIn + Gov ID + Liveness verification,0,Ready,TBD,2025-04-30
TG,Story,LinkedIn OAuth Flow Setup,Implement OAuth 2.0 flow for LinkedIn sign-in,8,Ready,Engineer A,2025-03-15
TG,Story,LinkedIn Data Extraction & Storage,Extract employment and education data from LinkedIn,5,Ready,Engineer A,2025-03-13
...
```

**Or manually create in JIRA:**
1. Create Epic: "Plane B - Consented Verification"
2. Create Sprint: "Sprint 1: LinkedIn OAuth"
3. Add stories as child issues
4. Assign to engineers
5. Set due dates

