# Plane B: Education Verification Implementation

## Overview

Plane B is the **consented verification** layer of TrustGraph. This document covers the **free education validation** component, which validates education data from LinkedIn OAuth using no external vendors.

**Phase:** Phase 1 Plane B (future - currently only Plane A is shipped)

---

## Architecture

```
User Links LinkedIn OAuth
         ↓
LinkedIn returns: {school_name, field_of_study, start_date, end_date, grade}
         ↓
Save to subject_education table
         ↓
EducationValidator (internal/verification/education_validator.go)
├─ Check 1: Timeline Plausibility (20 points)
├─ Check 2: Known University (30 points)
├─ Check 3: Career-Degree Alignment (25 points)
├─ Check 4: Recent Graduate (15 points)
└─ Check 5: GPA Disclosed (10 points)
         ↓
Confidence Score: 0-100
         ↓
IsVerified: true if score >= 70
         ↓
Badge: ✅ Stanford (Verified) OR 📚 Stanford (Self-Reported)
```

---

## Free Validation Signals

### **Signal 1: Timeline Plausibility (20 points)**

**What it checks:**
- Education ended before account creation (+ 3 month leniency)
- School duration is 1-8 years
- Start date not more than 40 years ago

**Why it matters:**
- Catches catfishers who claim to still be in school
- Ensures enrollment dates are realistic

**Example:**
- ✅ "Started 2018, ended 2022, account created 2023" = PASS
- ❌ "Started 2018, ended 2025, account created 2024" = FAIL (still in school)

### **Signal 2: Known University (30 points)**

**What it checks:**
- School is in the KnownUniversities map (top 500 universities)
- OR contains generic keywords ("university", "college", "institute")

**Why it matters:**
- Provides evidence the school actually exists
- Filters out obviously fake schools ("Fake University of Dreams")

**Limitation:**
- Small list of known universities (~50 explicitly, generic keywords for rest)
- In production, use paid service for comprehensive validation

**Example:**
- ✅ "Stanford University" = Known
- ✅ "UC Berkeley" = Known (partial match)
- ✅ "Community College" = Generic match
- ❌ "Fake School XYZ" = Unknown

### **Signal 3: Degree-Career Alignment (25 points)**

**What it checks:**
- Field of study matches current job title
- Uses category mapping (engineering, business, law, medicine, science)

**Why it matters:**
- If someone has a CS degree and works as a Software Engineer, it's consistent
- Catches misalignment (Biology degree but works in Finance)

**Examples:**
- ✅ CS + Software Engineer = ALIGNED
- ✅ Mechanical Engineering + Tech Lead = ALIGNED (generic match)
- ✅ MBA + Finance Manager = ALIGNED
- ❌ Accounting + Software Engineer = NOT ALIGNED

### **Signal 4: Recent Graduate (15 points)**

**What it checks:**
- Graduation date within last 10 years

**Why it matters:**
- Recent graduates are more trustworthy (invested in education recently)
- Very old education (40+ years ago) has lower signal

**Example:**
- ✅ Graduated 2 years ago = PASS
- ❌ Graduated 15 years ago = FAIL

### **Signal 5: GPA Disclosed (10 points)**

**What it checks:**
- User provided a GPA (optional field)

**Why it matters:**
- Shows transparency and investment in profile
- Users who hide GPA might be embarrassed

**Example:**
- ✅ Grade: "3.8" = PASS
- ❌ Grade: "" (empty) = FAIL

---

## Confidence Score Interpretation

| Score | Result | Badge | Action |
|-------|--------|-------|--------|
| 90-100 | Verified (All signals pass) | ✅ School (Verified) | Show to profile |
| 70-89 | Verified (3+ signals pass) | ✅ School (Verified) | Show to profile |
| 50-69 | Partial (2 signals pass) | ⚠️ School (Partial) | Flag for review |
| 0-49 | Unverified (0-1 signals pass) | 📚 School (Self-Reported) | Don't show badge |

---

## Integration Points

### 1. Assessment Handler (Future Plane B)

```go
// In internal/api/assessment_handler.go

// Add education data to assessment
func (h *AssessmentHandler) CreateAssessmentWithEducation(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...
    
    // Get education from LinkedIn OAuth
    eduRepo := store.NewEducationRepository(h.db)
    linkedInEdu, _ := eduRepo.GetEducationBySubject(ctx, subjectID)
    
    if linkedInEdu != nil {
        // Create education signal provider
        eduProvider := signals.NewEducationProvider(eduRepo)
        
        // Evaluate education
        eduSignal := eduProvider.EvaluateEducation(
            ctx,
            subjectID.String(),
            verification.EducationData{
                SchoolName:   linkedInEdu.SchoolName,
                FieldOfStudy: linkedInEdu.FieldOfStudy,
                StartDate:    linkedInEdu.StartDate,
                EndDate:      linkedInEdu.EndDate,
                Grade:        linkedInEdu.Grade,
            },
            req.Subject.CurrentJobTitle,  // From LinkedIn OAuth
            24,  // Account age in hours
        )
        
        // Add to policy scoring
        policySignals = append(policySignals, policy.SignalResult{
            Provider:    eduSignal.Provider,
            ReasonCodes: eduSignal.ReasonCodes,
            Score:       eduSignal.Score,
            Confidence:  eduSignal.Confidence,
        })
    }
}
```

### 2. Policy Engine Integration

The education signal will be weighted similarly to other Plane A signals:

```go
// In internal/policy/engine.go

// Add education weight (optional, low weight since it's free validation)
educationWeight := 0.15  // 15% of total scoring
```

### 3. Dashboard/UI Badge Display

```json
{
  "education": {
    "schoolName": "Stanford University",
    "badge": "✅ Stanford (Verified)",
    "confidence": 85,
    "signals": [
      "EDUCATION_KNOWN_UNIVERSITY",
      "EDUCATION_CAREER_ALIGNED",
      "EDUCATION_TIMELINE_PLAUSIBLE"
    ]
  }
}
```

---

## Testing

### Run Unit Tests

```bash
cd internal/verification
go test -v education_validator_test.go education_validator.go
```

### Test Scenarios

**Scenario 1: Perfect Profile**
```
Input: Stanford CS degree, 2020, GPA 3.8, works as Software Engineer, account 48h old
Expected: Confidence 90+, Verified=true
```

**Scenario 2: Decent Profile**
```
Input: UC Berkeley Engineering, 2018, GPA 3.5, works in Tech, account 72h old
Expected: Confidence 70-80, Verified=true
```

**Scenario 3: Weak Profile**
```
Input: Old school (1995), no GPA, misaligned job, account 24h old
Expected: Confidence <50, Verified=false
```

### Manual Testing

```go
package main

import (
    "context"
    "time"
    "trustgraph/internal/verification"
)

func main() {
    v := verification.NewEducationValidator()
    
    result := v.Validate(
        context.Background(),
        verification.EducationData{
            SchoolName:   "Stanford University",
            FieldOfStudy: "Computer Science",
            StartDate:    time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC),
            EndDate:      time.Date(2022, 5, 1, 0, 0, 0, 0, time.UTC),
            Grade:        "3.8",
        },
        "user@example.com",
        "Software Engineer at Google",
        48,  // 48 hours old account
    )
    
    println("Confidence:", result.ConfidenceScore)
    println("Verified:", result.IsVerified)
    println("Badge:", result.Badge)
}
```

---

## Database Schema

### subject_education
- `education_id` (UUID) - Primary key
- `subject_id` (UUID) - Foreign key to subject
- `school_name` (VARCHAR) - University name
- `field_of_study` (VARCHAR) - Major/field
- `start_date` (TIMESTAMPTZ) - Start date
- `end_date` (TIMESTAMPTZ) - Graduation date
- `grade` (VARCHAR) - GPA (optional)
- `confidence_score` (INT) - 0-100 validation score
- `is_verified` (BOOL) - true if confidence >= 70
- `validation_signals` (TEXT[]) - ['TIMELINE_PLAUSIBLE', 'KNOWN_UNIVERSITY', ...]
- `validation_details` (TEXT) - Human-readable summary
- `validation_risk_score` (INT) - Inverted risk (0 = high confidence)
- `source` (VARCHAR) - 'linkedin_oauth', 'manual_input', etc.
- `source_data` (JSONB) - Raw OAuth response
- `validated_at` (TIMESTAMPTZ) - When validation ran
- `created_at`, `updated_at`, `expires_at`

---

## Future Enhancements

### Phase 2: Paid Verification

```go
// Optional: Link to paid verification vendors

// Persona API
client := persona.NewClient(apiKey)
result := client.VerifyEducation(schoolName, degree, graduationYear)

// Truework API
result := truework.CheckDegree(schoolName, subjectName, graduationYear)

// Cost: $0.50-$3 per verification
```

### Phase 3: LinkedIn Deep Linking

```json
{
  "verification_type": "linkedin_profile_link",
  "linkedin_url": "https://linkedin.com/in/user",
  "publicly_verified": true
}
```

---

## Cost Analysis

| Component | Cost per User | When |
|-----------|---------------|------|
| Free validation | $0 | Every signup with LinkedIn |
| Optional paid verify | $1-3 | When user upgrades to premium |
| Storage | <$0.01 | All users, all time |
| **Total cost** | **$0 baseline** | **Free tier users** |

---

## Known Limitations

1. **Limited University Database**
   - Only ~50 universities explicitly listed
   - Generic keywords handle others but not perfect
   - Solution: Use paid service for comprehensive validation

2. **No Official Verification**
   - LinkedIn data is self-reported
   - Not actually verified with university registrar
   - Solution: Optional paid verification layer

3. **Career Alignment Heuristic**
   - Uses keywords, not official job classification
   - May have false positives/negatives
   - Solution: Machine learning model (future)

4. **No International Support**
   - University names primarily US-centric
   - Could be extended with international databases

5. **No Degree Revocation Tracking**
   - If diploma is revoked (fraud), we won't know
   - Solution: Paid vendor integration

---

## Decision Tree for Badge Display

```
User has education data?
├─ No → Skip (no badge)
└─ Yes → Run validation
    ├─ Confidence >= 70?
    │   ├─ Yes → Show ✅ School (Verified)
    │   └─ No → Show 📚 School (Self-Reported)
    ├─ User verified officially?
    │   ├─ Yes → Show ✅ School (Government Verified)
    │   └─ No → (as above)
    └─ User wants to hide?
        ├─ Yes → Don't show (respect privacy)
        └─ No → Show badge
```

---

## Next Steps to Ship Plane B

1. **LinkedIn OAuth Integration** (external, not in TrustGraph)
   - Add LinkedIn scopes to request education data
   - Handle OAuth callback and store data

2. **Integrate Education Validator**
   - Add education signal to assessment pipeline
   - Test with real LinkedIn data

3. **UI/UX**
   - Render education badges in profile
   - Show verification status to other users
   - Allow users to hide/delete education

4. **Testing**
   - Load test with 10k profiles
   - Verify accuracy of validation logic
   - Collect false positive/negative rates

5. **Optional: Paid Verification**
   - Integrate Persona or Truework APIs
   - Charge $2.99 for verified badge
   - Track vendor responses and costs
