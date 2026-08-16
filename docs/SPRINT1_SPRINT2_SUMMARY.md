# Sprint 1 & Sprint 2 - Complete JIRA Task Package

## 📦 What's Included

### **1. Epic: Plane B - Consented Verification**
- **Key:** TG-PLANE-B
- **Timeline:** 7 weeks (Weeks 9-15)
- **Team:** 2 engineers
- **Priority:** High

### **2. Sprint 1: LinkedIn OAuth Integration (2 weeks)**

**Feature: TG-LI-FEAT-1**

| Task | Points | Assignee | Due Date | Status |
|------|--------|----------|----------|--------|
| TG-LI-001: LinkedIn OAuth Flow Setup | 8 | Engineer A | 2025-03-15 | Ready |
| TG-LI-002: LinkedIn Data Extraction & Storage | 5 | Engineer A | 2025-03-13 | Ready |
| TG-LI-003: Employment Validator (Free) | 5 | Engineer A | 2025-03-15 | Ready |
| TG-LI-004: Employment Signal Provider | 3 | Engineer A | 2025-03-15 | Ready |
| TG-LI-005: LinkedIn Integration Testing | 5 | Engineer B | 2025-03-16 | Ready |
| TG-LI-006: LinkedIn Badges & UI | 5 | Engineer B | 2025-03-16 | Ready |
| **Sprint 1 Total** | **31 points** | **2 engineers** | **2025-03-16** | **Ready** |

### **3. Sprint 2: Government ID Verification + Age Gate (2 weeks)**

**Feature: TG-ID-FEAT-2**

| Task | Points | Assignee | Due Date | Priority |
|------|--------|----------|----------|----------|
| TG-ID-001: Persona API Integration | 8 | Engineer B | 2025-03-29 | Critical |
| TG-ID-002: Age Gate Implementation | 3 | Engineer A | 2025-03-27 | Critical |
| TG-ID-003: Government ID Testing | 5 | Engineer B | 2025-03-30 | High |
| **Sprint 2 Total** | **16 points** | **2 engineers** | **2025-03-30** | **Critical** |

---

## 📋 Task Details

### Sprint 1 Breakdown

#### TG-LI-001: LinkedIn OAuth Flow Setup (8 pts)
**Assignee:** Engineer A | **Due:** 2025-03-15

Implement OAuth 2.0 flow for LinkedIn sign-in.

**Acceptance Criteria:**
- ✓ LinkedIn app created and configured
- ✓ OAuth callback handler implemented
- ✓ Access token stored securely
- ✓ Consent screen shows requested data
- ✓ Error handling for rejected permissions
- ✓ Token refresh capability

**Files to Create:**
- `internal/api/oauth_handler.go`
- `migrations/003_linkedin_integration.sql`

---

#### TG-LI-002: LinkedIn Data Extraction & Storage (5 pts)
**Assignee:** Engineer A | **Due:** 2025-03-13

Extract and store employment and education data.

**Acceptance Criteria:**
- ✓ Parse LinkedIn profile API response
- ✓ Extract job history (title, company, dates)
- ✓ Extract education (school, degree, years)
- ✓ Store in subject_employment table
- ✓ Store in subject_education table
- ✓ Handle missing data gracefully

**Files to Create:**
- `internal/models/linkedin.go`
- `internal/store/linkedin_repo.go`

---

#### TG-LI-003: Employment Validator (5 pts)
**Assignee:** Engineer A | **Due:** 2025-03-15

Build free employment validation signals.

**Signals (Total 100 points):**
1. Timeline Plausibility (20 pts)
2. Company Existence (30 pts)
3. Job-Career Alignment (25 pts)
4. Employment Tenure (15 pts)
5. Verification Status (10 pts)

**Acceptance Criteria:**
- ✓ All 5 signals implemented
- ✓ Confidence score (0-100)
- ✓ 20+ unit tests
- ✓ <1ms per profile performance

**Files to Create:**
- `internal/verification/employment_validator.go`
- `internal/verification/employment_validator_test.go`

---

#### TG-LI-004: Employment Signal Provider (3 pts)
**Assignee:** Engineer A | **Due:** 2025-03-15

Create signal for assessment pipeline.

**Acceptance Criteria:**
- ✓ EmploymentProvider signal implemented
- ✓ Score range: 0-30 points
- ✓ Reason codes generated
- ✓ Integrated with policy engine

**Files to Create:**
- `internal/signals/employment_plane_b.go`

---

#### TG-LI-005: LinkedIn Integration Testing (5 pts)
**Assignee:** Engineer B | **Due:** 2025-03-16

End-to-end testing with real profiles.

**Test Scenarios:**
1. OAuth callback with real account
2. Complete data extraction
3. Incomplete profiles (missing fields)
4. Old job history (10+ years)
5. Permission denial flow
6. Token refresh
7. Load test (1000 profiles)

**Acceptance Criteria:**
- ✓ All scenarios passing
- ✓ <500ms per callback
- ✓ Database integrity verified

---

#### TG-LI-006: LinkedIn Badges & UI (5 pts)
**Assignee:** Engineer B | **Due:** 2025-03-16

Render verification badges on profile.

**Acceptance Criteria:**
- ✓ "✅ Google" badge if verified
- ✓ "📼 Google" badge if self-reported
- ✓ Job title and tenure displayed
- ✓ Mobile and web responsive
- ✓ Visible to other users

**Components:**
- Employment Badge Component
- Profile Employment Section
- Privacy Controls

---

### Sprint 2 Breakdown

#### TG-ID-001: Persona API Integration (8 pts)
**Assignee:** Engineer B | **Due:** 2025-03-29 | **CRITICAL**

Integrate Persona API for government ID verification.

**Vendor:** Persona ($2.99/verification)

**Acceptance Criteria:**
- ✓ Persona account configured
- ✓ Document upload flow implemented
- ✓ Selfie/liveness verification works
- ✓ Age extracted from ID (DOB)
- ✓ Verified name stored
- ✓ Vendor response logged (PII stripped)
- ✓ Error handling implemented
- ✓ Cost tracking per user

**Files to Create:**
- `internal/verification/id_verifier.go`
- `internal/models/government_id.go`
- `migrations/004_id_verification.sql`

---

#### TG-ID-002: Age Gate Implementation (3 pts)
**Assignee:** Engineer A | **Due:** 2025-03-27 | **CRITICAL**

Enforce age gate. Block users under 18.

**Compliance:**
- ✓ COPPA (Children's Online Privacy Protection)
- ✓ Dating app industry standard
- ✓ Audit trail of all denials

**Acceptance Criteria:**
- ✓ DOB from ID verification
- ✓ Calculate age at assessment
- ✓ Block if age < 18
- ✓ Error code: UNDERAGE_USER
- ✓ Admin alert on underage attempts

**Files to Create:**
- `internal/policy/age_gate.go`

---

#### TG-ID-003: Government ID Testing (5 pts)
**Assignee:** Engineer B | **Due:** 2025-03-30

Test ID verification with real documents.

**Test Scenarios:**
1. Valid driver's license
2. Valid passport
3. Expired ID (should fail)
4. Fake ID (should fail)
5. Age boundary tests
6. Multiple attempts
7. Load test (500 concurrent)

**Acceptance Criteria:**
- ✓ All scenarios passing
- ✓ Age extraction 100% accurate
- ✓ Cost tracking verified
- ✓ <60s for 500 IDs

---

## 🎯 Key Metrics

### Effort Estimate
- **Total Story Points:** 47 points
- **Timeline:** 4 weeks (2 weeks per sprint)
- **Team:** 2 full-time engineers
- **Velocity:** 23.5 points/week

### Success Criteria
- ✓ All acceptance criteria met
- ✓ >80% code coverage
- ✓ Zero production bugs in first 2 weeks
- ✓ <500ms API response time
- ✓ AI Commander integration active

### Deliverables
- ✓ Plane B OAuth integration
- ✓ Employment verification
- ✓ Government ID verification
- ✓ Age gate enforcement
- ✓ Badges on user profiles
- ✓ Full test coverage

---

## 🚀 How to Use

### 1. Import into JIRA

**Option A: CSV Import (Recommended)**
```bash
# Go to JIRA Settings → Tools → Bulk Import
# Upload: docs/JIRA_SPRINT1_SPRINT2_IMPORT.csv
# Click Import
```

**Option B: JSON Import (API)**
```bash
# Use JIRA CLI or API
curl -X POST \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d @docs/JIRA_SPRINT1_SPRINT2_IMPORT.json \
  https://YOUR-DOMAIN.atlassian.net/rest/api/2/issues/bulk
```

### 2. Verify AI Commander Integration

```bash
# Wait 2-5 minutes for Aicc-Coordinator to poll JIRA
# Then verify:
curl http://localhost:8080/operations?source_tenant=TG \
  -H "Authorization: Bearer TOKEN"
# Should return 12 operations
```

### 3. Assign Engineers

Replace placeholders in JIRA:
- `[Engineer A]` → Actual engineer name
- `[Engineer B]` → Actual engineer name

### 4. Start Sprint 1

In JIRA Sprint Planning:
1. Click "Start Sprint" for Sprint 1
2. Set sprint goal: "Implement LinkedIn OAuth integration"
3. Add stories to sprint
4. Engineers can start claiming tasks

---

## 📁 Files Provided

```
docs/
├── JIRA_SPRINT1_SPRINT2_IMPORT.csv      ← CSV for bulk import
├── JIRA_SPRINT1_SPRINT2_IMPORT.json     ← JSON for API import
├── JIRA_IMPORT_INSTRUCTIONS.md          ← Step-by-step guide
├── SPRINT1_SPRINT2_SUMMARY.md           ← This file
├── PHASE_1_PLANE_B_C_ROADMAP.md         ← Full 12-week roadmap
├── PHASE_1_TIMELINE.md                  ← Timeline + decision matrix
├── AICOMMANDER_TASKS.md                 ← AI Commander format
└── PLANE_B_EDUCATION.md                 ← Education validator docs
```

---

## ✅ Pre-Import Checklist

- [ ] JIRA project `TG` (TrustGraph) created
- [ ] Issue types exist: Epic, Feature, Story
- [ ] Custom field "Story Points" configured
- [ ] Custom field "Sprint" configured
- [ ] Sprints created: "Sprint 1" and "Sprint 2"
- [ ] Engineers' JIRA accounts created
- [ ] Aicc-Coordinator configured for JIRA ingestion
- [ ] Environment variables set:
  - `AICC_JIRA_ENABLED=true`
  - `AICC_JIRA_PROJECTS=TG`
  - `AICC_JIRA_READY_STATUS=Ready for AI Commander`

---

## 🎓 Next Steps

1. **Import tasks** (choose CSV or JSON method)
2. **Assign engineers** (replace placeholders)
3. **Create sprints** (if not already done)
4. **Start Sprint 1** (in JIRA Sprint Planning)
5. **Wait 2-5 minutes** (for Aicc-Coordinator ingestion)
6. **Verify AI Commander** (tasks should appear as operations)
7. **Begin Sprint 1** (engineers can claim tasks)

---

## 💡 Tips

- **Monday Start:** Schedule Sprint 1 to start on Monday 2025-03-10
- **Daily Standup:** 10am daily (auto-report: TG-LI-001, TG-LI-002, etc.)
- **Sprint Review:** Friday 2025-03-16 (Sprint 1 completion)
- **Sprint Planning:** Monday 2025-03-24 (Sprint 2 kickoff)

---

## 📞 Support

**Issues with import?**
- See `JIRA_IMPORT_INSTRUCTIONS.md` → Troubleshooting section
- Check JIRA settings for issue types and custom fields
- Verify Aicc-Coordinator configuration

**Questions about tasks?**
- Each story includes detailed acceptance criteria
- See `PHASE_1_PLANE_B_C_ROADMAP.md` for full context
- Refer to `PLANE_B_EDUCATION.md` for background on validation

---

## 📊 Summary

✅ **12 JIRA tasks ready for import**
- 1 Epic: Plane B - Consented Verification
- 2 Features: Sprint 1 & 2
- 9 Stories: Implementation tasks
- 47 Story Points
- 4-week timeline
- AI Commander compatible

**Status:** Ready for immediate import and execution.

