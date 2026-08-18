# TrustGraph Phase 1 Plane B & C - AI Commander Task Format

## AI Commander Integration

This roadmap is structured to be automatically ingested by **Aicc-Coordinator** (AI Commander's operations coordinator).

### Required JIRA Configuration

For tasks to appear in AI Commander, they must have:

1. **Status:** `Ready for AI Commander`
2. **Label:** `ai-commander-ready` (opt-in gate)
3. **Project:** One of the configured projects with a repository mapping
4. **NOT labeled:** `human-only`, `security-sensitive` (exclusion veto)

---

## Epic 1: Plane B - Consented Verification

```
Type: Epic
Key: TG-PLANE-B
Title: Plane B - Consented Verification (Phase 1)
Description: Implement LinkedIn OAuth, Government ID, Liveness, and Image verification for user-initiated trust signals.
Status: Ready for AI Commander
Labels: ai-commander-ready, plane-b, phase-1
Timeline: 7 weeks (Weeks 9-15)
Team: 2 engineers
Priority: HIGH
Repository: trustgraph
```

---

## Sprint 1: LinkedIn OAuth Integration (Week 9-10)

### Task: TG-LI-001 - LinkedIn OAuth Flow Setup

```
Type: Story
Key: TG-LI-001
Title: LinkedIn OAuth Flow Setup
Description: |
  Implement OAuth 2.0 flow for LinkedIn sign-in, capturing employment and education data.
  
  ## Acceptance Criteria
  - [ ] LinkedIn app created and configured
  - [ ] OAuth callback handler implemented
  - [ ] Access token stored securely
  - [ ] Consent screen shows what data is requested
  - [ ] Error handling for rejected permissions
  - [ ] Can refresh expired tokens
  
  ## Implementation Details
  File: `internal/api/oauth_handler.go`
  
  ## Related Files
  - `migrations/003_linkedin_integration.sql`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-1, linkedin, plane-b
Story Points: 8
Assignee: [Engineer A]
Due Date: 2025-03-15
Epic: TG-PLANE-B
Repository: trustgraph

## Subtasks
- [ ] TG-LI-001-1: Design OAuth callback handler
- [ ] TG-LI-001-2: Implement LinkedIn scopes (profile, email, public_profile)
- [ ] TG-LI-001-3: Add token storage to database
- [ ] TG-LI-001-4: Create UI flow for "Link LinkedIn" button
- [ ] TG-LI-001-5: Add error handling for permission denial
```

### Task: TG-LI-002 - LinkedIn Data Extraction & Storage

```
Type: Story
Key: TG-LI-002
Title: LinkedIn Data Extraction & Storage
Description: |
  Extract employment and education data from LinkedIn OAuth response. Store in database.
  
  ## Acceptance Criteria
  - [ ] Parse LinkedIn profile API response
  - [ ] Extract job history, titles, companies, dates
  - [ ] Extract education, schools, degrees, years
  - [ ] Store in subject_employment table
  - [ ] Store in subject_education table
  - [ ] Handle missing/incomplete data gracefully
  
  ## Data Model
  ```go
  type LinkedInProfile struct {
    ID          string
    Email       string
    FirstName   string
    LastName    string
    Employment  []LinkedInJob
    Education   []LinkedInEducation
  }
  ```

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-1, linkedin, plane-b
Story Points: 5
Assignee: [Engineer A]
Due Date: 2025-03-13
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-LI-003 - Employment Validator (Free Validation)

```
Type: Story
Key: TG-LI-003
Title: Employment Validator (Free Validation)
Description: |
  Build free employment validation (similar to education validator).
  
  ## Validation Signals
  - Timeline plausibility (20 points) — employment start before current job ends
  - Company existence (30 points) — check if company is known
  - Job-career alignment (25 points) — match job title to education field
  - Employment tenure (15 points) — longer employment = more trustworthy
  - Verification status (10 points) — explicitly verified
  
  ## Acceptance Criteria
  - [ ] Validate employment timeline
  - [ ] Check company existence (basic)
  - [ ] Match job title to career field
  - [ ] Weight by tenure
  - [ ] Generate confidence score (0-100)
  - [ ] Identify verified vs. self-reported
  - [ ] Write 20+ unit tests
  
  ## Files
  - `internal/verification/employment_validator.go`
  - `internal/verification/employment_validator_test.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-1, linkedin, plane-b, validation
Story Points: 5
Assignee: [Engineer A]
Due Date: 2025-03-15
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-LI-004 - Employment Signal Provider

```
Type: Story
Key: TG-LI-004
Title: Employment Signal Provider
Description: |
  Create employment signal for assessment pipeline. Contribute to risk scoring.
  
  ## Acceptance Criteria
  - [ ] Implement EmploymentProvider signal
  - [ ] Calculate score from employment confidence
  - [ ] Generate reason codes:
    - EMPLOYMENT_VERIFIED
    - EMPLOYMENT_DURATION_ESTABLISHED
    - EMPLOYMENT_ALIGNED_WITH_EDUCATION
  - [ ] Integrate with policy engine
  - [ ] Score range: 0-30 points
  
  ## Files
  - `internal/signals/employment_plane_b.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-1, linkedin, plane-b, signals
Story Points: 3
Assignee: [Engineer A]
Due Date: 2025-03-15
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-LI-005 - LinkedIn Integration Testing

```
Type: Story
Key: TG-LI-005
Title: LinkedIn Integration Testing
Description: |
  E2E testing of LinkedIn OAuth flow with real profiles.
  
  ## Acceptance Criteria
  - [ ] Test OAuth callback with real LinkedIn account
  - [ ] Verify all employment data extracted correctly
  - [ ] Test with incomplete profiles (missing jobs, education)
  - [ ] Test with old job history (10+ years back)
  - [ ] Verify data stored correctly in database
  - [ ] Test permission denial flow
  - [ ] Test token refresh
  - [ ] Load test with 1000 profiles
  
  ## Files
  - Test account setup
  - E2E test suite

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-1, linkedin, plane-b, testing
Story Points: 5
Assignee: [Engineer B]
Due Date: 2025-03-16
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-LI-006 - LinkedIn Badges & UI

```
Type: Story
Key: TG-LI-006
Title: LinkedIn Badges & UI
Description: |
  Render employment verification badges on user profile.
  
  ## Acceptance Criteria
  - [ ] Show "✅ Google" badge if verified
  - [ ] Show "📼 Google" badge if self-reported
  - [ ] Display job title and tenure
  - [ ] Link to LinkedIn profile (optional)
  - [ ] Render on both mobile and web
  - [ ] Show to other users (verified status)
  
  ## UI Components
  - Employment badge component
  - Profile employment section
  - Mobile responsive layout

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-1, linkedin, plane-b, ui
Story Points: 5
Assignee: [Engineer B]
Due Date: 2025-03-16
Epic: TG-PLANE-B
Repository: trustgraph
```

---

## Sprint 2: Government ID Verification (Week 11-12)

### Task: TG-ID-001 - Persona API Integration

```
Type: Story
Key: TG-ID-001
Title: Persona API Integration
Description: |
  Integrate Persona API for government ID verification and age gate.
  
  Vendor: Persona ($2.99/verification)
  
  ## Acceptance Criteria
  - [ ] Persona account created and API key configured
  - [ ] Document upload flow implemented
  - [ ] Selfie/liveness verification works
  - [ ] Age extracted from ID
  - [ ] Verified name stored
  - [ ] Vendor response logged (PII stripped)
  - [ ] Error handling for failed verifications
  - [ ] Cost tracking per user
  
  ## Files
  - `internal/verification/id_verifier.go`
  - `migrations/004_id_verification.sql`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-2, government-id, plane-b, vendor
Story Points: 8
Assignee: [Engineer B]
Due Date: 2025-03-29
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-ID-002 - Age Gate Implementation

```
Type: Story
Key: TG-ID-002
Title: Age Gate Implementation (CRITICAL)
Description: |
  Enforce age gate in assessment. Block users under 18.
  
  This is CRITICAL for dating app compliance.
  
  ## Acceptance Criteria
  - [ ] Check DOB from government ID verification
  - [ ] Calculate age at assessment time
  - [ ] Block assessment if age < 18
  - [ ] Return specific error code (UNDERAGE_USER)
  - [ ] Log underage attempts
  - [ ] Alert admin to potential child protection issue
  
  ## Files
  - `internal/policy/age_gate.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-2, government-id, plane-b, critical
Story Points: 3
Assignee: [Engineer A]
Due Date: 2025-03-27
Epic: TG-PLANE-B
Repository: trustgraph
Priority: CRITICAL
```

### Task: TG-ID-003 - Government ID Testing

```
Type: Story
Key: TG-ID-003
Title: Government ID Testing
Description: |
  Test ID verification with real documents (test accounts from Persona).
  
  ## Acceptance Criteria
  - [ ] Test with valid driver's license
  - [ ] Test with valid passport
  - [ ] Test with expired ID (should fail)
  - [ ] Test with fake ID (should fail)
  - [ ] Verify age extraction accurate
  - [ ] Test age gate blocking minors
  - [ ] Verify vendor cost tracking
  - [ ] Load test with 500 ID verifications
  
  ## Test Documents
  - Use Persona's test document set
  - Simulate various ID types

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-2, government-id, plane-b, testing
Story Points: 5
Assignee: [Engineer B]
Due Date: 2025-03-30
Epic: TG-PLANE-B
Repository: trustgraph
```

---

## Sprint 3: Liveness & Image Verification (Week 13-14)

### Task: TG-LIV-001 - Liveness Check Integration

```
Type: Story
Key: TG-LIV-001
Title: Liveness Check Integration
Description: |
  Integrate liveness check (video proof user is real person).
  
  Vendor: Persona (included with ID verification)
  
  ## Acceptance Criteria
  - [ ] Use Persona liveness or standalone
  - [ ] Capture video selfie
  - [ ] Verify user is real person
  - [ ] Link to ID verification (ensure same person)
  - [ ] Store verification result
  - [ ] Error handling for failed liveness
  
  ## Files
  - `internal/verification/liveness_verifier.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-3, liveness, plane-b
Story Points: 5
Assignee: [Engineer A]
Due Date: 2025-04-11
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-IMG-001 - Reverse Image Search Integration

```
Type: Story
Key: TG-IMG-001
Title: Reverse Image Search Integration
Description: |
  Check profile images against internet for catfishing detection.
  
  Vendors: Google Images API (free tier), TinEye API ($0.10/check)
  
  ## Acceptance Criteria
  - [ ] Scan profile image via Google Images API
  - [ ] Check TinEye for duplicates
  - [ ] Flag if image found elsewhere online
  - [ ] Alert user of duplicate detection
  - [ ] Store results in database
  
  ## Files
  - `internal/verification/image_search.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-3, image-verification, plane-b
Story Points: 4
Assignee: [Engineer B]
Due Date: 2025-04-11
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-IMG-002 - Synthetic Image Detection

```
Type: Story
Key: TG-IMG-002
Title: Synthetic Image Detection
Description: |
  Detect AI-generated/synthetic profile images (DALL-E, Midjourney, etc).
  
  Vendor: Free ML model (first iteration), Real.pictures ($0.25/check optional)
  
  ## Acceptance Criteria
  - [ ] Use free ML model (deepfakes detection)
  - [ ] Flag suspicious images
  - [ ] Score confidence (0-100)
  - [ ] Alert user if synthetic detected
  - [ ] Store detection results
  
  ## Files
  - `internal/verification/synthetic_detector.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-3, image-verification, plane-b
Story Points: 5
Assignee: [Engineer A]
Due Date: 2025-04-12
Epic: TG-PLANE-B
Repository: trustgraph
```

---

## Sprint 4: Plane B Completion & Launch (Week 15)

### Task: TG-PB-001 - Consent Management

```
Type: Story
Key: TG-PB-001
Title: Consent Management
Description: |
  Implement consent tracking for all Plane B verification types.
  
  ## Acceptance Criteria
  - [ ] Track opt-in for LinkedIn
  - [ ] Track opt-in for government ID
  - [ ] Track opt-in for liveness
  - [ ] Support withdrawal of consent
  - [ ] GDPR compliance (right to deletion)
  - [ ] Show consent screen before each verification
  - [ ] Store consent timestamp and method
  
  ## Files
  - `migrations/005_plane_b_consent.sql`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-4, consent, plane-b, compliance
Story Points: 5
Assignee: [Engineer B]
Due Date: 2025-04-18
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-PB-002 - Verification API Endpoints

```
Type: Story
Key: TG-PB-002
Title: Verification API Endpoints
Description: |
  Create REST API endpoints for verification flows.
  
  ## Endpoints
  - POST /v1/verification/linkedin/authorize
  - GET /v1/verification/linkedin/callback
  - POST /v1/verification/id/upload
  - POST /v1/verification/liveness/upload
  - GET /v1/verification/status
  - DELETE /v1/verification/{type}
  - GET /v1/user/profile/badges
  
  ## Acceptance Criteria
  - [ ] All endpoints require authentication
  - [ ] Proper error handling and validation
  - [ ] Rate limiting per user
  - [ ] Response validation and testing
  
  ## Files
  - `internal/api/verification_handler.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-4, api, plane-b
Story Points: 5
Assignee: [Engineer A]
Due Date: 2025-04-18
Epic: TG-PLANE-B
Repository: trustgraph
```

### Task: TG-PB-003 - Plane B Launch & Monitoring

```
Type: Story
Key: TG-PB-003
Title: Plane B Launch & Monitoring
Description: |
  Deploy Plane B to production with monitoring.
  
  ## Acceptance Criteria
  - [ ] Shadow deploy to 10% of users
  - [ ] Monitor vendor latency (<500ms)
  - [ ] Track verification success rate
  - [ ] Monitor cost (per-user vendor spend)
  - [ ] Create runbook for troubleshooting
  - [ ] Full rollout to 100%
  - [ ] Success metrics tracking
  
  ## Success Metrics
  - 40% of users get at least one verification badge
  - 25% link LinkedIn OAuth
  - 15% verify government ID
  - 10% complete liveness check
  - 99.9% verification success rate
  - <500ms API latency
  
  ## Files
  - Deployment scripts
  - Monitoring dashboards
  - Runbooks

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-4, deployment, plane-b, monitoring
Story Points: 3
Assignee: [Both]
Due Date: 2025-04-19
Epic: TG-PLANE-B
Repository: trustgraph
Priority: HIGH
```

---

## Epic 2: Plane C - Investigation Tools

```
Type: Epic
Key: TG-PLANE-C
Title: Plane C - Investigation Tools (Phase 1)
Description: Implement investigator authentication, case management, and OSINT tools for manual review of high-risk accounts.
Status: Ready for AI Commander
Labels: ai-commander-ready, plane-c, phase-1
Timeline: 5-6 weeks (Weeks 16-21)
Team: 2 engineers
Priority: HIGH
Repository: trustgraph
```

---

## Sprint 5: Investigator Auth & Case Management (Week 16-17)

### Task: TG-INV-001 - Investigator RBAC

```
Type: Story
Key: TG-INV-001
Title: Investigator RBAC
Description: |
  Implement role-based access control for investigators.
  
  ## Roles
  - Junior Investigator: Read cases, view tools
  - Lead Investigator: Create cases, make decisions
  - Admin: Manage investigators, audit logs
  
  ## Acceptance Criteria
  - [ ] Create investigator role
  - [ ] Define permissions (read_cases, open_case, access_tools, etc.)
  - [ ] Enforce 2FA for investigator accounts
  - [ ] Require strong password (12+ chars, special chars)
  - [ ] Require signed data usage agreement
  - [ ] Audit trail of all investigator actions
  
  ## Files
  - `internal/auth/investigator_auth.go`
  - `internal/models/investigator_role.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-5, auth, plane-c, critical
Story Points: 5
Assignee: [Engineer A]
Due Date: 2025-04-25
Epic: TG-PLANE-C
Repository: trustgraph
Priority: CRITICAL
```

### Task: TG-INV-002 - Investigation Case Management

```
Type: Story
Key: TG-INV-002
Title: Investigation Case Management
Description: |
  Build case management system for investigations.
  
  ## Case Statuses
  - open → assigned → resolved → archived
  
  ## Acceptance Criteria
  - [ ] Create investigation case model
  - [ ] Case status flow implementation
  - [ ] Assign case to investigator
  - [ ] Track case metadata (reason, priority, dates)
  - [ ] Support case notes
  - [ ] Support finding attachments (evidence)
  - [ ] Case search and filtering
  - [ ] Auto-archive after 90 days
  
  ## Files
  - `internal/models/investigation_case.go`
  - `internal/store/investigation_repo.go`
  - `migrations/006_investigation_cases.sql`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-5, cases, plane-c
Story Points: 8
Assignee: [Engineer B]
Due Date: 2025-04-26
Epic: TG-PLANE-C
Repository: trustgraph
```

---

## Sprint 6: Investigation Tools (Week 18-19)

### Task: TG-OSINT-001 - Internet Archive Integration

```
Type: Story
Key: TG-OSINT-001
Title: Internet Archive Integration
Description: |
  Query Wayback Machine for historical profiles/websites.
  
  Vendor: Internet Archive API (free)
  
  ## Acceptance Criteria
  - [ ] Query Internet Archive API for snapshots
  - [ ] Display historical versions
  - [ ] Show when profile changed
  - [ ] Detect suspicious changes
  
  ## Files
  - `internal/tools/internet_archive.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-6, osint, plane-c
Story Points: 3
Assignee: [Engineer A]
Due Date: 2025-05-02
Epic: TG-PLANE-C
Repository: trustgraph
```

### Task: TG-OSINT-002 - Sherlock Integration

```
Type: Story
Key: TG-OSINT-002
Title: Sherlock Integration (Username Enumeration)
Description: |
  Search username across 300+ social platforms.
  
  Vendor: Sherlock (open-source, free)
  
  ## Acceptance Criteria
  - [ ] Search username on major platforms
  - [ ] Return found accounts with links
  - [ ] Detect pattern of account creation
  - [ ] Flag suspicious patterns (same username everywhere = catfisher)
  
  ## Files
  - `internal/tools/sherlock.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-6, osint, plane-c
Story Points: 4
Assignee: [Engineer A]
Due Date: 2025-05-03
Epic: TG-PLANE-C
Repository: trustgraph
```

### Task: TG-OSINT-003 - Email/Domain Tools

```
Type: Story
Key: TG-OSINT-003
Title: Email/Domain Tools (theHarvester)
Description: |
  Find associated emails, domains, and breach data.
  
  Vendor: theHarvester (open-source), HaveIBeenPwned API
  
  ## Acceptance Criteria
  - [ ] Query public email sources
  - [ ] Find related domains
  - [ ] Check breach databases
  - [ ] Track social media presence
  - [ ] Link to other identities
  
  ## Files
  - `internal/tools/the_harvester.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-6, osint, plane-c
Story Points: 5
Assignee: [Engineer B]
Due Date: 2025-05-04
Epic: TG-PLANE-C
Repository: trustgraph
```

### Task: TG-OSINT-004 - SpiderFoot Integration

```
Type: Story
Key: TG-OSINT-004
Title: SpiderFoot Integration
Description: |
  Aggregate OSINT findings from multiple sources.
  
  Vendor: SpiderFoot (open-source + optional paid modules)
  
  ## Acceptance Criteria
  - [ ] Run SpiderFoot on email/domain
  - [ ] Correlate findings across sources
  - [ ] Detect linked profiles
  - [ ] Generate investigation report
  - [ ] Track confidence scores
  
  ## Files
  - `internal/tools/spiderfoot.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-6, osint, plane-c
Story Points: 5
Assignee: [Engineer B]
Due Date: 2025-05-05
Epic: TG-PLANE-C
Repository: trustgraph
```

---

## Sprint 7: Investigation APIs & Audit (Week 20-21)

### Task: TG-INV-API-001 - Investigation REST Endpoints

```
Type: Story
Key: TG-INV-API-001
Title: Investigation REST Endpoints
Description: |
  Create API endpoints for investigator workflow.
  
  ## Endpoints
  - POST /v1/investigations/case (create)
  - GET /v1/investigations/case/{id} (view)
  - PUT /v1/investigations/case/{id} (update)
  - GET /v1/investigations/cases (list)
  - POST /v1/investigations/{id}/archive (query)
  - POST /v1/investigations/{id}/approve (approve user)
  - POST /v1/investigations/{id}/suspend (suspend account)
  
  ## Acceptance Criteria
  - [ ] All endpoints implemented
  - [ ] All endpoints require investigator auth
  - [ ] Proper error handling
  - [ ] Request/response validation
  - [ ] Rate limiting enforcement
  
  ## Files
  - `internal/api/investigation_handler.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-7, api, plane-c
Story Points: 5
Assignee: [Engineer A]
Due Date: 2025-05-09
Epic: TG-PLANE-C
Repository: trustgraph
```

### Task: TG-INV-AUDIT-001 - Investigation Audit Logging

```
Type: Story
Key: TG-INV-AUDIT-001
Title: Investigation Audit Logging (CRITICAL)
Description: |
  Log all investigator actions for compliance and audit.
  
  Required for legal compliance and investigator accountability.
  
  ## Acceptance Criteria
  - [ ] Log who accessed what
  - [ ] Log when and why
  - [ ] Log results/findings
  - [ ] Support audit report generation
  - [ ] Immutable log (cannot be deleted)
  - [ ] HIPAA/GDPR compliant
  - [ ] Break-glass alerting on sensitive access
  
  ## Files
  - `internal/audit/investigation_audit.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-7, audit, plane-c, critical, compliance
Story Points: 5
Assignee: [Engineer B]
Due Date: 2025-05-10
Epic: TG-PLANE-C
Repository: trustgraph
Priority: CRITICAL
```

### Task: TG-INV-ALERT-001 - Break-Glass Alerting

```
Type: Story
Key: TG-INV-ALERT-001
Title: Break-Glass Alerting
Description: |
  Alert on suspicious investigator activity.
  
  ## Alerts Triggered On
  - Self-lookups (investigator searching own info)
  - Bulk queries
  - Late-night access
  - Data export
  
  ## Acceptance Criteria
  - [ ] Alert on self-lookups
  - [ ] Alert on bulk queries
  - [ ] Alert on late-night access
  - [ ] Alert on data export
  - [ ] Escalate to admin for review
  - [ ] Immutable alert log
  
  ## Files
  - `internal/audit/break_glass.go`

Status: Ready for AI Commander
Labels: ai-commander-ready, sprint-7, alerting, plane-c, security
Story Points: 3
Assignee: [Engineer A]
Due Date: 2025-05-11
Epic: TG-PLANE-C
Repository: trustgraph
```

---

## Key Dates & Dependencies

```
Sprint 1 Start:  2025-03-10  (LinkedIn OAuth)
Sprint 2 Start:  2025-03-24  (Government ID)
Sprint 3 Start:  2025-04-07  (Liveness + Images)
Sprint 4 Start:  2025-04-21  (Plane B Launch)
Sprint 5 Start:  2025-04-28  (Investigator Auth)
Sprint 6 Start:  2025-05-12  (Investigation Tools)
Sprint 7 Start:  2025-05-26  (Investigation APIs)

Plane B Complete: 2025-04-25
Plane C Complete: 2025-06-07
```

---

## Labels for AI Commander Filtering

All tasks are tagged with:
- `ai-commander-ready` — required for AI Commander ingestion
- `sprint-{N}` — which sprint (sprint-1, sprint-2, etc.)
- `plane-b` or `plane-c` — which plane
- `phase-1` — which phase
- Functional labels: `linkedin`, `government-id`, `liveness`, `image-verification`, `osint`, `api`, `auth`, `consent`, `testing`, `ui`, `deployment`, `monitoring`, `audit`, `alerting`
- Priority labels: `critical`, `high` (implicit medium if not specified)
- Constraint labels: `vendor`, `compliance`, `security`

---

## JIRA Import Command

To import these tasks into JIRA:

```bash
# Install JIRA CLI (if needed)
curl -L https://github.com/go-jira/jira/releases/download/v1.1.2/jira-linux-amd64 -o jira
chmod +x jira

# Create tasks from this format
# (JIRA API integration via Aicc-Coordinator will auto-ingest with correct labels/status)

# Verify tasks are visible to AI Commander:
# 1. Set Status to "Ready for AI Commander"
# 2. Add Label "ai-commander-ready"
# 3. Commit and push
# 4. Aicc-Coordinator will poll and ingest within 2 minutes
```

---

## Verification Checklist for AI Commander Ingestion

- [ ] All tasks have Status: "Ready for AI Commander"
- [ ] All tasks have Label: "ai-commander-ready"
- [ ] No tasks have excluded labels (human-only, security-sensitive)
- [ ] Tasks are in TrustGraph project
- [ ] Project is configured in Aicc-Coordinator
- [ ] Repository mapping exists for TrustGraph
- [ ] Tasks appear in Aicc-Coordinator within 2 minutes of commit
