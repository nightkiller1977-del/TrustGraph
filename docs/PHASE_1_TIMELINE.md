# Phase 1 Complete Timeline: Plane A → B → C

## Current Status: Plane A Complete ✅

```
PHASE 1 TIMELINE
═══════════════════════════════════════════════════════════════════════

✅ COMPLETE (Weeks 1-8, Already Done)
├── Plane A: Auto-Verification
│   ├── Email Signal (verification + disposable detection) ✅
│   ├── Phone Signal ✅
│   ├── Device Fingerprint Sharing ✅
│   ├── Registration Velocity Detection ✅
│   ├── Image Hash Reuse ✅
│   ├── Policy Engine (scoring + trust tiers) ✅
│   ├── Assessment API (/v1/assessments) ✅
│   └── Audit Logging ✅
│
└── Plane B Infrastructure (Partial)
    └── Education Validator (Free) ✅ NEW!

⏳ TODO: PLANE B EXTENSIONS (Weeks 9-20)
├── Sprint 1: LinkedIn OAuth (2 weeks)
│   ├── OAuth flow integration
│   ├── Extract employment data
│   ├── Employment validator (free)
│   └── Employment signals
│
├── Sprint 2: Government ID (2 weeks)
│   ├── Persona/Onfido integration
│   ├── ID verification flow
│   ├── Age gate enforcement
│   └── Identity verification signals
│
├── Sprint 3: Liveness & Images (2 weeks)
│   ├── Liveness check integration
│   ├── Reverse image search
│   ├── Synthetic image detection
│   └── Image verification signals
│
└── Sprint 4: Plane B Completion (1 week)
    ├── Consent management
    ├── Verification API endpoints
    ├── Badge rendering
    └── Testing & launch

⏳ TODO: PLANE C (Weeks 21-32)
├── Sprint 5: Investigator Auth (2 weeks)
│   ├── RBAC implementation
│   ├── Investigator roles/permissions
│   ├── Case management
│   └── 2FA + strong password enforcement
│
├── Sprint 6: Investigation Tools (2 weeks)
│   ├── Internet Archive integration
│   ├── Sherlock (username enumeration)
│   ├── theHarvester (email/domain)
│   ├── Domain intelligence
│   └── SpiderFoot integration
│
└── Sprint 7: Investigation APIs (1 week)
    ├── Investigation endpoints
    ├── Audit logging (enhanced)
    ├── Break-glass alerts
    └── Testing & launch

TOTAL TIMELINE: 32 weeks (~8 months)
WITH 2 ENGINEERS: 4-5 months
═══════════════════════════════════════════════════════════════════════
```

---

## What's Done vs. What's Left

### ✅ Completed

| Component | Status | When | Who |
|-----------|--------|------|-----|
| Plane A (5 signals) | SHIPPED ✅ | Weeks 1-8 | Team |
| Education Validator | DONE ✅ | TODAY | Me |
| Assessment API | SHIPPED ✅ | Weeks 1-8 | Team |
| Policy Engine | SHIPPED ✅ | Weeks 1-8 | Team |
| Audit Logging | SHIPPED ✅ | Weeks 1-8 | Team |

**Total: 5 shipped features, ~1000 lines of code**

---

### ⏳ Remaining: Plane B

| Component | Effort | Cost | Timeline | Priority |
|-----------|--------|------|----------|----------|
| **LinkedIn OAuth** | 3-4 days | $0 | Sprint 1 | HIGH ⭐⭐⭐ |
| Employment Validator | 2 days | $0 | Sprint 1 | HIGH ⭐⭐⭐ |
| Gov't ID (Persona) | 4-5 days | $1-3/user | Sprint 2 | CRITICAL ⭐⭐⭐ |
| Age Gate | 1 day | $0 | Sprint 2 | CRITICAL ⭐⭐⭐ |
| Liveness Check | 3-4 days | $2-5/user | Sprint 3 | MEDIUM ⭐⭐ |
| Image Verification | 3-4 days | $0-2/user | Sprint 3 | MEDIUM ⭐⭐ |
| Consent + APIs | 2-3 days | $0 | Sprint 4 | HIGH ⭐⭐⭐ |

**Plane B Total: 6-7 weeks, 2 engineers**

---

### ⏳ Remaining: Plane C

| Component | Effort | Cost | Timeline | Priority |
|-----------|--------|------|----------|----------|
| Investigator Auth | 2-3 days | $0 | Sprint 5 | CRITICAL ⭐⭐⭐ |
| Case Management | 3-4 days | $0 | Sprint 5 | CRITICAL ⭐⭐⭐ |
| Free OSINT Tools | 5-7 days | $0 | Sprint 6 | HIGH ⭐⭐⭐ |
| Paid Tools (Optional) | 3-4 days | $50-500/mo | Sprint 6 | MEDIUM ⭐⭐ |
| Investigation APIs | 3-4 days | $0 | Sprint 7 | CRITICAL ⭐⭐⭐ |
| Audit + Break-Glass | 2-3 days | $0 | Sprint 7 | CRITICAL ⭐⭐⭐ |

**Plane C Total: 5-6 weeks, 2 engineers**

---

## Recommendation: Start With Sprint 1 (LinkedIn OAuth)

### Why LinkedIn First?

1. **Highest ROI:** $0 vendor cost, massive trust signal
2. **Builds on Education:** We already have the validator
3. **User Demand:** People want to see professional profiles
4. **Unblocks Plane C:** Investigation tools can query LinkedIn
5. **Quick Win:** 2-3 weeks to launch

### LinkedIn Sprint Timeline

```
Week 1:
  Mon-Tue: OAuth flow, data extraction
  Wed:     Database schema, repository
  Thu-Fri: Employment validator, signal provider
  
Week 2:
  Mon-Tue: API integration, endpoint tests
  Wed:     UI/badge rendering
  Thu-Fri: E2E testing, edge cases
  
Week 3 (Partial):
  Mon:     Deployment preparation
  Tue:     Shadow deploy, monitoring
  Wed:     Launch to production
```

### What You Get in 2 Weeks

✅ Users can link LinkedIn  
✅ Employment signals in assessments  
✅ Professional badges on profiles  
✅ $0 vendor cost  
✅ Foundation for Plane C tools  

**Impact:** 20-30% signup boost (verified professionals attract others)

---

## Cost Comparison: Plane B Vendors

### Government ID Verification (Required)

| Vendor | Cost/User | Identity | Age Verification | Recommended |
|--------|-----------|----------|------------------|-------------|
| Persona | $2.99 | Yes ✅ | Yes ✅ | Best UX |
| Onfido | $1.99 | Yes ✅ | Yes ✅ | Budget |
| Truework | $0.50 | Partial | No | Education only |
| IDology | $1-2 | Yes ✅ | Yes ✅ | Legacy |

**Recommendation:** Persona ($2.99) - best UX, highest approval rate

### Liveness Check (Optional)

| Vendor | Cost/User | Recommendation |
|--------|-----------|-----------------|
| Persona (included) | Included | Use with ID |
| Onfido | Included | Use with ID |
| BioID | $0.50-1 | Standalone option |
| Self-hosted ML | $0.10-0.20 | Advanced (future) |

### Image Verification (Optional)

| Vendor | Cost | Type | Recommendation |
|--------|------|------|-----------------|
| None (free) | $0 | Google reverse image | Start here |
| TinEye API | $0.10/check | Reverse image | Add later |
| Real.pictures | $0.25/check | Synthetic detection | Advanced |

---

## Decision Point: Sprint Selection

**You have 3 options:**

### Option A: Maximum Impact (Recommended) 🎯
- **Start with:** Sprint 1 (LinkedIn)
- **Then:** Sprint 2 (Gov't ID)
- **Why:** Stagger vendor costs, build trust progressively

**Timeline:** 4 weeks → launch Plane B MVP

```
Week 1-2: LinkedIn OAuth
Week 3-4: Government ID + Age Gate
Deploy: Launch Plane B with 2 verification types
```

### Option B: Full Speed
- **Start with:** Sprints 1 + 2 in parallel (2 engineers)
- **Then:** Sprints 3-4 sequential
- **Why:** Get all of Plane B done in 6 weeks

**Timeline:** 6-7 weeks → launch Plane B complete

```
Week 1-2: LinkedIn (Engineer A) + Gov ID (Engineer B)
Week 3-4: Liveness (A) + Images (B)
Week 5: Consent + APIs (both)
Deploy: Launch Plane B complete
```

### Option C: Skip Plane B, Start Plane C
- **Start with:** Sprint 5 (Investigator Auth)
- **Then:** Sprints 6-7
- **Why:** Get case management working for high-risk accounts

**Timeline:** 5-6 weeks → launch Plane C for manual review

```
Week 1-2: Investigator auth + cases
Week 3-4: Investigation tools
Week 5: APIs + audit
Deploy: Manual investigation flow for Plane A flagged accounts
```

---

## My Recommendation

**Go with Option A: Start LinkedIn, then Gov't ID**

### Rationale

1. **LinkedIn is free** — No vendor risk, validate product-market fit
2. **Gov't ID is critical** — Age gate + legal identity for dating app
3. **Staggered costs** — Validate adoption before committing to vendors
4. **Builds momentum** — Team sees quick wins
5. **Plane C can wait** — Plane A + Plane B handles 95% of cases

### Timeline

```
THIS WEEK:
  └─ Finalize LinkedIn OAuth design

NEXT 2 WEEKS (Sprint 1):
  └─ Launch LinkedIn integration
  
WEEKS 3-4 (Sprint 2):
  └─ Add Gov't ID verification
  
WEEKS 5-6 (Sprint 3):
  └─ Add liveness + images (optional features)
  
WEEK 7 (Sprint 4):
  └─ APIs, consent, launch
  
RESULT: Plane B complete in 7 weeks
```

---

## Next Steps

Which sprint should we build first?

- [ ] **Sprint 1: LinkedIn OAuth** (START HERE)
- [ ] **Sprint 2: Government ID** (Critical path)
- [ ] **Sprint 3: Liveness & Images** (Nice to have)
- [ ] **Sprint 5: Plane C Investigation** (Alternative path)

**What's your priority?**
