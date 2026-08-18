# Phase 1 Complete: Plane B & C Implementation Roadmap

## Overview

This roadmap outlines shipping both **Plane B (Consented Verification)** and **Plane C (Investigation Tools)** in Phase 1, completing the trust architecture for ConnectionSphere.

**Timeline Estimate:** 8-12 weeks (2 engineers)

---

## Plane B: Consented Verification (User-Initiated Trust Signals)

### Plane B Architecture

```
User Registration
    ↓
Plane A: Automatic (Mandatory)
├─ Email verification + disposable check
├─ Phone verification
├─ Device fingerprint
├─ Registration velocity
└─ Image hash tracking
    ↓
Assessment Decision: Accept/Review/Deny
    ↓
IF Approved, show Plane B options:
    ├─ LinkedIn OAuth (career verification)
    ├─ Government ID (legal identity)
    ├─ Liveness Check (proof of person)
    ├─ Reverse Image Search (detect fake photos)
    └─ Synthetic Image Detection (catch AI-gen images)
    ↓
User chooses (optional):
    ├─ Link LinkedIn → Get ✅ badge
    ├─ Upload ID → Get ✅ badge
    ├─ Liveness video → Get ✅ badge
    └─ Verify images → Get ✅ badge
    ↓
Result: Verification badges on profile
```

---

## Plane B: Implementation Breakdown

### **Sprint 1: LinkedIn OAuth (Week 1-2)**

#### 1.1 OAuth Flow Integration
- [ ] Add LinkedIn app configuration
- [ ] Implement OAuth callback handler
- [ ] Extract education data from LinkedIn response
- [ ] Handle consent/data permissions

**File:** `internal/api/oauth_handler.go`

```go
type LinkedInProfile struct {
    ID           string
    Email        string
    FirstName    string
    LastName     string
    ProfileURL   string
    Employment   []LinkedInJob
    Education    []LinkedInEducation
    Connections  int
}

type LinkedInJob struct {
    Company   string
    Title     string
    StartDate time.Time
    EndDate   *time.Time
    IsCurrent bool
}
```

#### 1.2 Data Storage
- [ ] Update `subject_education` table (already done ✅)
- [ ] Create `subject_employment` table
- [ ] Create `subject_linkedin_profile` table
- [ ] Add consent tracking

**Files:** `migrations/003_linkedin_integration.sql`

#### 1.3 Employment Validator
Similar to education validator—free validation without vendor:
- Check employment timeline plausibility
- Verify company existence (basic check)
- Align with current LinkedIn title
- Weight by tenure (longer = more trustworthy)

**File:** `internal/verification/employment_validator.go`

#### 1.4 LinkedIn Signal Provider
- [ ] Create employment signal for assessment
- [ ] Calculate trust score from LinkedIn profile
- [ ] Generate employment badges

**File:** `internal/signals/employment_plane_b.go`

**Expected Signals:**
- EMPLOYMENT_VERIFIED (30 pts)
- EMPLOYMENT_DURATION_ESTABLISHED (20 pts)
- EMPLOYMENT_ALIGNED_WITH_EDUCATION (15 pts)
- LINKEDIN_CONNECTIONS_VERIFIED (10 pts)

---

### **Sprint 2: Government ID Verification (Week 3-4)**

#### 2.1 ID Verification Vendor Integration
- [ ] Integrate Persona API (or Onfido)
- [ ] Handle document upload flow
- [ ] Validate ID document
- [ ] Extract verified name, DOB, address

**File:** `internal/verification/id_verifier.go`

```go
type IDVerificationResult struct {
    Status      string // "verified", "pending", "failed"
    VerifiedName string
    DateOfBirth time.Time
    Address     Address
    IDType      string // "drivers_license", "passport", etc.
    VendorID    string
    VendorResp  map[string]interface{}
}
```

#### 2.2 Database Schema
- [ ] Create `identity_verification` table
- [ ] Create `verification_vendor_response` table (audit trail)
- [ ] Track verification costs per user

**File:** `migrations/004_id_verification.sql`

#### 2.3 Age Gate (Critical for Dating App)
- [ ] Verify user is 18+ (or jurisdiction minimum)
- [ ] Enforce during Plane A assessment
- [ ] Flag minor accounts for removal

**File:** `internal/policy/age_gate.go`

```go
func (e *Engine) CheckAgeGate(dob time.Time) error {
    age := time.Now().Year() - dob.Year()
    if age < 18 {
        return errors.New("underage user detected")
    }
    return nil
}
```

---

### **Sprint 3: Liveness & Image Verification (Week 5-6)**

#### 3.1 Liveness Check
- [ ] Integrate liveness vendor (Persona, Onfido)
- [ ] Handle video upload
- [ ] Verify user is real person
- [ ] Link to ID verification (ensure same person)

**File:** `internal/verification/liveness_verifier.go`

#### 3.2 Reverse Image Search
- [ ] Check profile images against public internet
- [ ] Detect stolen/catfish photos
- [ ] Use free services (Google Images, TinEye API)
- [ ] Optional paid: SauceNAO, Yandex API

**File:** `internal/verification/image_search.go`

#### 3.3 Synthetic Image Detection
- [ ] Detect AI-generated faces (DALL-E, Midjourney, etc.)
- [ ] Use free models (first iteration)
- [ ] Future: Real.pictures or similar API

**File:** `internal/verification/synthetic_detector.go`

---

### **Sprint 4: Consent & API Endpoints (Week 7)**

#### 4.1 Consent Management
- [ ] Create consent flow for each verification type
- [ ] Track user opt-in/opt-out
- [ ] GDPR/CCPA compliance
- [ ] Right to deletion

**File:** `migrations/005_plane_b_consent.sql`

#### 4.2 REST Endpoints

```
POST   /v1/verification/linkedin/authorize     → Start OAuth
GET    /v1/verification/linkedin/callback      → OAuth return
POST   /v1/verification/id/upload              → Upload ID
POST   /v1/verification/liveness/upload        → Upload video
GET    /v1/verification/status                 → Check status
DELETE /v1/verification/{type}                 → Delete consent
GET    /v1/user/profile/badges                 → Get badges
```

**File:** `internal/api/verification_handler.go`

---

## Plane C: Investigation Tools (Authorized Access Only)

### Plane C Architecture

```
TrustGraph Assessment
    ↓
Decision: ACCEPT / REVIEW / DENY
    ↓
If REVIEW or uncertain case:
    ↓
Create Investigation Case (Plane C)
    ├─ Case ID: auto-generated
    ├─ Assigned to: Investigator
    ├─ Reason: fraud, safety concern, verification fail
    └─ Status: open
    ↓
Investigator Tools Available:
    ├─ Internet Archive (historical profiles)
    ├─ Domain Intelligence (website history)
    ├─ Username Enumeration (Sherlock)
    ├─ Email/Domain Tracking (theHarvester)
    ├─ Public Records (if legal)
    ├─ SpiderFoot (OSINT aggregator)
    └─ Manual Notes
    ↓
Investigator Action:
    ├─ Approve User
    ├─ Flag for Manual Review
    ├─ Suspend Account
    └─ Report to Authorities
    ↓
All Actions Audited:
    ├─ Who accessed what
    ├─ When and why
    ├─ What they found
    └─ Break-glass alerts on sensitive access
```

### Plane C: Implementation Breakdown

### **Sprint 5: Investigator Auth & Case Management (Week 8-9)**

#### 5.1 Investigator Role & Permissions
- [ ] Create investigator role in auth system
- [ ] Implement RBAC (Role-Based Access Control)
- [ ] Require strong password + 2FA
- [ ] Audit all investigator actions

**File:** `internal/auth/investigator_auth.go`

```go
type InvestigatorRole struct {
    ID          uuid.UUID
    Name        string // "junior_investigator", "lead_investigator", "admin"
    Permissions []string // ["read_cases", "open_case", "access_tools", ...]
    MaxCases    int
    CanDeleteData bool
}
```

#### 5.2 Investigation Case Management
- [ ] Create investigation case model
- [ ] Case status flow: open → assigned → resolved → archived
- [ ] Track case metadata (reason, priority, assigned investigator)
- [ ] Support case notes and findings

**File:** `internal/models/investigation_case.go`

```go
type InvestigationCase struct {
    CaseID         uuid.UUID
    SubjectID      uuid.UUID
    AssignedTo     uuid.UUID // Investigator ID
    CaseType       string    // "fraud", "safety_concern", "verification_fail"
    Reason         string
    Status         string    // "open", "assigned", "resolved", "archived"
    Priority       string    // "low", "medium", "high", "critical"
    CreatedAt      time.Time
    ResolvedAt     *time.Time
    Resolution     string
}
```

#### 5.3 Case Repository
- [ ] CRUD operations for cases
- [ ] Query cases by status, priority, investigator
- [ ] Track case timeline

**File:** `internal/store/investigation_repo.go`

---

### **Sprint 6: Investigation Tools Integration (Week 10-11)**

#### 6.1 Internet Archive Integration
- [ ] Query Wayback Machine API
- [ ] Get historical snapshots of websites
- [ ] Detect profile changes over time

**File:** `internal/tools/internet_archive.go`

```go
func (ia *InternetArchive) GetSnapshots(url string) ([]Snapshot, error) {
    // https://archive.org/wayback/available?url={url}
    // Returns list of archived versions
}
```

#### 6.2 Domain Intelligence
- [ ] WHOIS lookups (free: whois CLI)
- [ ] DNS records (free: dig CLI)
- [ ] Certificate transparency logs (free API)
- [ ] Website age and history

**File:** `internal/tools/domain_intel.go`

#### 6.3 Username Enumeration (Sherlock)
- [ ] Check username across 300+ platforms
- [ ] Link social accounts
- [ ] Detect pattern of account creation

**File:** `internal/tools/sherlock.go`

```go
func (s *Sherlock) SearchUsername(username string) ([]SocialAccount, error) {
    // Check: Twitter, Instagram, TikTok, LinkedIn, GitHub, etc.
    // Return found accounts with links
}
```

#### 6.4 Email/Domain Tracking (theHarvester)
- [ ] Find associated emails
- [ ] Find subdomains
- [ ] Find related breaches
- [ ] Social media presence

**File:** `internal/tools/the_harvester.go`

#### 6.5 SpiderFoot Integration
- [ ] Run OSINT scan on email/domain
- [ ] Correlate findings across sources
- [ ] Detect linked profiles

**File:** `internal/tools/spiderfoot.go`

---

### **Sprint 7: Investigation API & Audit Logging (Week 12)**

#### 7.1 Investigation API Endpoints

```
POST   /v1/investigations/case           → Create case
GET    /v1/investigations/case/{id}      → View case
PUT    /v1/investigations/case/{id}      → Update case
GET    /v1/investigations/cases          → List cases
DELETE /v1/investigations/case/{id}      → Close case

POST   /v1/investigations/{id}/archive   → Get Internet Archive data
POST   /v1/investigations/{id}/domain    → Get domain intel
POST   /v1/investigations/{id}/username  → Search username
POST   /v1/investigations/{id}/email     → Search email
POST   /v1/investigations/{id}/spiderfoot → Run OSINT scan
POST   /v1/investigations/{id}/evidence  → Attach evidence

POST   /v1/investigations/{id}/approve   → Approve user
POST   /v1/investigations/{id}/flag      → Flag for review
POST   /v1/investigations/{id}/suspend   → Suspend account
POST   /v1/investigations/{id}/report    → Report to authorities
```

**File:** `internal/api/investigation_handler.go`

#### 7.2 Enhanced Audit Logging
- [ ] Log all investigator queries
- [ ] Log what data was accessed
- [ ] Log what decisions were made
- [ ] Support audit report generation

**File:** `internal/audit/investigation_audit.go`

```go
type InvestigationAuditEvent struct {
    AuditID       uuid.UUID
    InvestigatorID uuid.UUID
    CaseID        uuid.UUID
    Action        string // "viewed_internet_archive", "searched_email", etc.
    QueryData     string
    Results       int
    Timestamp     time.Time
}
```

#### 7.3 Break-Glass Alerting
- [ ] Alert on self-lookups (investigator searching own data)
- [ ] Alert on sensitive access patterns
- [ ] Alert on data export
- [ ] Require justification for sensitive queries

**File:** `internal/audit/break_glass.go`

```go
func (bg *BreakGlass) CheckAlert(event *InvestigationAuditEvent) error {
    // Alert if: self-lookup, bulk export, late-night access, etc.
}
```

---

## Implementation Strategy

### Phase B: Priority Matrix

```
        ┌─────────────────────────────────┐
        │ LinkedIn (High Impact)          │ ← START HERE
        │ + Education Validator (Done ✅) │
        │ + Employment Signals             │
        │ Cost: $0 | Value: High          │
        └─────────────────────────────────┘
              ↓
        ┌─────────────────────────────────┐
        │ Government ID (High Value)      │
        │ + Age Gate (Critical)            │
        │ Cost: $1-3/user | Value: Critical│
        └─────────────────────────────────┘
              ↓
        ┌─────────────────────────────────┐
        │ Liveness + Image Verification   │
        │ Cost: $0-2 | Value: Medium      │
        └─────────────────────────────────┘
              ↓
        ┌─────────────────────────────────┐
        │ Consent Management & APIs       │
        │ Cost: $0 | Value: Medium        │
        └─────────────────────────────────┘
```

### Phase C: Priority Matrix

```
        ┌─────────────────────────────────┐
        │ Investigator Auth + RBAC        │ ← START HERE
        │ + Case Management                │
        │ Cost: $0 | Value: Foundation    │
        └─────────────────────────────────┘
              ↓
        ┌─────────────────────────────────┐
        │ Free OSINT Tools                │
        │ (Internet Archive, Sherlock)     │
        │ Cost: $0 | Value: High          │
        └─────────────────────────────────┘
              ↓
        ┌─────────────────────────────────┐
        │ Paid Tools (SpiderFoot, etc.)   │
        │ Cost: $50-500/month | Value: Med│
        └─────────────────────────────────┘
              ↓
        ┌─────────────────────────────────┐
        │ Investigation APIs + Audit      │
        │ Cost: $0 | Value: Critical      │
        └─────────────────────────────────┘
```

---

## Resource Estimates

### Plane B Effort

| Component | Engineer-Days | Complexity |
|-----------|---------------|-----------|
| LinkedIn OAuth | 3-4 | Medium |
| Education Validator | 1 | Low (Done ✅) |
| Employment Validator | 2 | Low |
| Government ID (Persona) | 4-5 | Medium |
| Age Gate | 1 | Low |
| Liveness Check | 3-4 | Medium |
| Image Verification | 3-4 | Medium |
| Consent Management | 2-3 | Low |
| API Endpoints | 2-3 | Low |
| Testing & Integration | 5-7 | Medium |
| **Total** | **26-33 days** | **~6-7 weeks** |

### Plane C Effort

| Component | Engineer-Days | Complexity |
|-----------|---------------|-----------|
| Investigator Auth | 2-3 | Low |
| Case Management | 3-4 | Low |
| Internet Archive | 1-2 | Low |
| Sherlock Integration | 2 | Low |
| Email/Domain Tools | 2-3 | Low |
| SpiderFoot Integration | 3-4 | Medium |
| Investigation APIs | 3-4 | Medium |
| Audit Logging | 2-3 | Low |
| Break-Glass Alerts | 2 | Medium |
| Testing & Integration | 4-5 | Medium |
| **Total** | **24-30 days** | **~5-6 weeks** |

**Grand Total: 50-63 days (~2-3 engineers, 8-12 weeks)**

---

## Database Schema Summary

### Plane B Tables (New)

```sql
-- Already done ✅
subject_education

-- Need to add
subject_employment
subject_linkedin_profile
government_id_verification
liveness_verification
image_verification
verification_consent

-- Update existing
subject (add verified_at, has_government_id, has_liveness)
```

### Plane C Tables (New)

```sql
investigation_case
investigation_tool_query
investigation_audit_event
investigation_evidence
investigator_role
investigator_permission
break_glass_alert
```

---

## Go-Live Criteria

### Plane B Launch Checklist

- [ ] All 5 verification types implemented
- [ ] All 3 trust tiers updated with verification signals
- [ ] Age gate enforced (no users under 18)
- [ ] Badges render on profile
- [ ] Privacy compliance (GDPR/CCPA)
- [ ] Cost tracking (vendor charges)
- [ ] Load testing (10k profiles)
- [ ] 99% vendor uptime SLA met
- [ ] Support playbook (help users with failed verifications)
- [ ] Analytics dashboard (verification rates)

### Plane C Launch Checklist

- [ ] Investigator auth working (2FA + strong password)
- [ ] Case management end-to-end
- [ ] All tools tested and integrated
- [ ] Audit logging comprehensive
- [ ] Break-glass alerts triggering correctly
- [ ] Investigator training completed
- [ ] Investigation playbook documented
- [ ] Data retention policies defined
- [ ] Right-to-deletion implemented
- [ ] Investigator role permissions locked down

---

## Success Metrics

### Plane B

- Target: 40% of users get at least one verification badge within 30 days
- LinkedIn: 25% connect LinkedIn OAuth
- Government ID: 15% verify official identity (premium feature)
- Liveness: 10% complete liveness check
- Result: 50% increase in trust signals, 30% increase in match safety

### Plane C

- Target: 5-10% of assessments trigger investigation
- Investigation resolution time: < 24 hours
- Investigator productivity: 20-30 cases/day
- False positive rate: < 5% (cases incorrectly flagged)
- User impact: Zero false suspensions per million assessments

---

## Risk & Mitigation

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Vendor outage (ID verification) | High | Multi-vendor support, fallback flow |
| Privacy scandal | Critical | Strong data retention policies, encryption |
| Investigator abuse | Critical | Audit logs, break-glass alerts, 2FA |
| False accusations | High | Appeals process, human review required |
| Cost overrun | Medium | Cap vendor spending, monitor usage |
| Adoption too slow | Medium | Incentivize verification (matching boost) |

---

## Next: Detailed Sprint Plans

Ready to dive into:
- [ ] Sprint 1: LinkedIn OAuth (Week 1-2)
- [ ] Sprint 2: Government ID (Week 3-4)
- [ ] Sprint 3: Liveness & Images (Week 5-6)
- [ ] Sprint 4: Plane B APIs & Consent (Week 7)
- [ ] Sprint 5: Investigator Auth (Week 8-9)
- [ ] Sprint 6: Investigation Tools (Week 10-11)
- [ ] Sprint 7: Investigation APIs (Week 12)

Which sprint should we start with?
