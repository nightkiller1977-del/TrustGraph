package verification

import (
	"context"
	"strings"
	"time"
)

// EducationData represents education info from LinkedIn OAuth
type EducationData struct {
	SchoolName   string    `json:"school_name"`
	FieldOfStudy string    `json:"field_of_study"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	Grade        string    `json:"grade"`
}

// EducationValidationResult contains the validation score and details
type EducationValidationResult struct {
	ConfidenceScore int       `json:"confidence_score"` // 0-100
	IsVerified      bool      `json:"is_verified"`      // true if > 70
	Signals         []string  `json:"signals"`          // Evidence signals
	Badge           string    `json:"badge"`            // Display badge
	Details         string    `json:"details"`          // Human-readable summary
	Cost            float64   `json:"cost"`             // $0 for free validation
}

// KnownUniversities is a curated list of real universities (top 500 by enrollment)
// In production, this would be loaded from a database or external service
var KnownUniversities = map[string]struct{}{
	// Ivy League
	"harvard university":       {},
	"yale university":          {},
	"princeton university":     {},
	"columbia university":      {},
	"university of pennsylvania": {},
	"dartmouth college":        {},
	"brown university":         {},
	"cornell university":       {},

	// Top State Schools
	"stanford university":      {},
	"mit":                      {},
	"massachusetts institute of technology": {},
	"university of california, berkeley": {},
	"uc berkeley":              {},
	"university of michigan":   {},
	"university of virginia":   {},
	"university of texas at austin": {},
	"university of washington": {},
	"university of california, los angeles": {},
	"ucla":                     {},
	"northwestern university":  {},
	"duke university":          {},
	"carnegie mellon university": {},
	"caltech":                  {},

	// Other notable schools
	"chicago university":       {},
	"university of chicago":    {},
	"johns hopkins university": {},
	"rice university":          {},
	"emory university":         {},
	"vanderbilt university":    {},
	"washington university in st. louis": {},
	"georgia institute of technology": {},
	"georgia tech":             {},

	// Community colleges (partial list, real impl would have all)
	"community college":        {},
	"college":                  {},
}

// DegreeKeywords maps degrees to career fields
var DegreeKeywords = map[string][]string{
	"engineering": {
		"engineer", "software", "mechanical", "electrical", "civil", "chemical",
		"computer", "science", "cs", "programming", "developer", "tech", "it",
	},
	"business": {
		"mba", "business", "finance", "accounting", "economics", "management",
		"consultant", "finance", "banker", "trader", "cfo", "ceo",
	},
	"medicine": {
		"medical", "md", "doctor", "physician", "surgeon", "nurse", "healthcare",
		"hospital", "clinic", "pharma", "healthcare",
	},
	"law": {
		"jd", "law", "attorney", "lawyer", "legal", "paralegal", "counsel",
	},
	"science": {
		"physics", "chemistry", "biology", "phd", "researcher", "scientist",
		"lab", "research", "academic",
	},
}

// EducationValidator validates education data from LinkedIn OAuth
type EducationValidator struct {
	// In production: inject repository for user data, logger, etc.
}

// NewEducationValidator creates a new validator
func NewEducationValidator() *EducationValidator {
	return &EducationValidator{}
}

// Validate performs free education validation and returns a confidence score
func (v *EducationValidator) Validate(
	ctx context.Context,
	edu EducationData,
	userEmail string,
	currentJobTitle string,
	accountAgeHours int,
) EducationValidationResult {

	result := EducationValidationResult{
		ConfidenceScore: 0,
		Signals:         []string{},
		Cost:            0,
	}

	// Signal 1: Timeline plausibility
	if v.validateTimeline(edu, accountAgeHours) {
		result.ConfidenceScore += 20
		result.Signals = append(result.Signals, "TIMELINE_PLAUSIBLE")
	}

	// Signal 2: Real university check
	if v.isRealUniversity(edu.SchoolName) {
		result.ConfidenceScore += 30
		result.Signals = append(result.Signals, "KNOWN_UNIVERSITY")
	}

	// Signal 3: Degree-career alignment
	if v.degreeMatchesCareer(edu.FieldOfStudy, currentJobTitle) {
		result.ConfidenceScore += 25
		result.Signals = append(result.Signals, "DEGREE_CAREER_ALIGNED")
	}

	// Signal 4: Graduation year recency (fresh grads more trustworthy)
	if v.isRecentGraduate(edu.EndDate) {
		result.ConfidenceScore += 15
		result.Signals = append(result.Signals, "RECENT_GRADUATE")
	}

	// Signal 5: GPA listed (shows transparency)
	if edu.Grade != "" && edu.Grade != "0" {
		result.ConfidenceScore += 10
		result.Signals = append(result.Signals, "GPA_DISCLOSED")
	}

	// Set verified flag at 70+ confidence
	result.IsVerified = result.ConfidenceScore >= 70

	// Generate badge text
	result.Badge = v.generateBadge(result.IsVerified, edu.SchoolName)

	// Generate human-readable details
	result.Details = v.generateDetails(result, edu)

	return result
}

// validateTimeline checks if education timeline is plausible
// (ended before account creation, started at reasonable age)
func (v *EducationValidator) validateTimeline(edu EducationData, accountAgeHours int) bool {
	now := time.Now()
	accountCreatedAt := now.Add(time.Duration(-accountAgeHours) * time.Hour)

	// Check 1: Education ended before account was created
	// (no leniency: they couldn't have enrolled after signup)
	if edu.EndDate.After(accountCreatedAt) {
		return false // Went to school after account creation = suspicious
	}

	// Check 2: Didn't start school too early (before age 10)
	// Assume age 18 at start of college
	minStartAge := now.AddDate(-40, 0, 0) // Couldn't have started before 40 years ago
	if edu.StartDate.Before(minStartAge) {
		return false
	}

	// Check 3: Education duration is reasonable (1-8 years)
	duration := edu.EndDate.Sub(edu.StartDate)
	if duration < time.Duration(1*365*24)*time.Hour || duration > time.Duration(8*365*24)*time.Hour {
		return false
	}

	return true
}

// isRealUniversity checks against known universities list
func (v *EducationValidator) isRealUniversity(schoolName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(schoolName))

	// Direct match
	if _, exists := KnownUniversities[normalized]; exists {
		return true
	}

	// Partial match (handles "UC Berkeley" vs "University of California, Berkeley")
	for known := range KnownUniversities {
		if strings.Contains(normalized, known) || strings.Contains(known, normalized) {
			return true
		}
	}

	// Generic check: if it STARTS with known keywords, assume real
	// (avoid false positives like "Fake School of Dreams")
	validKeywords := []string{"university", "college", "institute", "academy"}
	for _, keyword := range validKeywords {
		if strings.Contains(normalized, keyword) && len(normalized) > 10 {
			// Must contain keyword AND be reasonably long name
			return true
		}
	}

	return false
}

// degreeMatchesCareer checks if field of study aligns with current job
func (v *EducationValidator) degreeMatchesCareer(fieldOfStudy, currentJobTitle string) bool {
	fieldLower := strings.ToLower(fieldOfStudy)
	jobLower := strings.ToLower(currentJobTitle)

	// For each degree category, check if both degree and job keywords match
	for _, keywords := range DegreeKeywords {
		hasDegreeKeyword := false
		hasJobKeyword := false

		// Check if field of study contains any keywords from this category
		for _, keyword := range keywords {
			if strings.Contains(fieldLower, keyword) {
				hasDegreeKeyword = true
				break
			}
		}

		// Check if job title contains any keywords from this category
		for _, keyword := range keywords {
			if strings.Contains(jobLower, keyword) {
				hasJobKeyword = true
				break
			}
		}

		// If we found both degree and job keywords in same category, it's aligned
		if hasDegreeKeyword && hasJobKeyword {
			return true
		}
	}

	// Loose match: both mention common terms. Must check fieldLower and
	// jobLower separately — counting occurrences in the concatenated
	// string (fullText) passes a term that appears twice in ONE field and
	// zero times in the other, e.g. field "Data Science and Data
	// Analytics" + job "Chef" has "data" twice with no job-side match.
	commonTerms := []string{"tech", "finance", "business", "data"}
	for _, term := range commonTerms {
		if strings.Contains(fieldLower, term) && strings.Contains(jobLower, term) {
			return true
		}
	}

	return false
}

// isRecentGraduate checks if graduated within last 10 years
func (v *EducationValidator) isRecentGraduate(endDate time.Time) bool {
	now := time.Now()
	tenYearsAgo := now.AddDate(-10, 0, 0)
	// A future end date fails validateTimeline (never plausible relative to
	// account creation) but, without this upper bound, still passed here —
	// any future date is trivially "after ten years ago" — awarding the
	// 15-point signal for a graduation date that hasn't happened yet.
	return endDate.After(tenYearsAgo) && !endDate.After(now)
}

// generateBadge creates the display badge text
func (v *EducationValidator) generateBadge(isVerified bool, schoolName string) string {
	if isVerified {
		return "✅ " + schoolName + " (Verified)"
	}
	return "📚 " + schoolName + " (Self-Reported)"
}

// signalPhrases converts validation signal codes into natural-language
// clauses, in scoring order. A score can reach the verified threshold
// without every signal passing (e.g. known-school + career-aligned +
// recent-grad + GPA = 80 with no timeline check at all), so details text
// must be built from whichever signals actually fired rather than
// asserting a fixed set of checks all passed.
func (v *EducationValidator) signalPhrases(signals []string) []string {
	phraseBySignal := map[string]string{
		"TIMELINE_PLAUSIBLE":    "timeline is plausible",
		"KNOWN_UNIVERSITY":      "school is known",
		"DEGREE_CAREER_ALIGNED": "aligns with current career",
		"RECENT_GRADUATE":       "recent graduate",
		"GPA_DISCLOSED":         "GPA disclosed",
	}
	order := []string{"TIMELINE_PLAUSIBLE", "KNOWN_UNIVERSITY", "DEGREE_CAREER_ALIGNED", "RECENT_GRADUATE", "GPA_DISCLOSED"}
	present := make(map[string]bool, len(signals))
	for _, s := range signals {
		present[s] = true
	}
	phrases := make([]string, 0, len(signals))
	for _, s := range order {
		if present[s] {
			phrases = append(phrases, phraseBySignal[s])
		}
	}
	return phrases
}

// generateDetails creates human-readable validation details
func (v *EducationValidator) generateDetails(result EducationValidationResult, edu EducationData) string {
	if result.IsVerified {
		details := "Education verified with high confidence."
		if phrases := v.signalPhrases(result.Signals); len(phrases) > 0 {
			details += " Signals: " + strings.Join(phrases, ", ") + "."
		}
		if result.ConfidenceScore >= 90 {
			details += " Very strong signal."
		}
		return details
	}

	if result.ConfidenceScore >= 50 {
		details := "Moderate confidence in education claim. "
		switch {
		case len(result.Signals) >= 2:
			details += "Multiple signals support this education."
		default:
			details += "Consider verifying with official credential."
		}
		return details
	}

	return "Unable to verify education claim. Consider linking official credentials."
}

// CalculateEducationRiskScore returns a risk score (0-100) for trust tier assignment
// Lower score = lower risk = better
func (v *EducationValidator) CalculateEducationRiskScore(result EducationValidationResult) int {
	// Inverted: high confidence = low risk
	// 100 confidence → risk score 0
	// 0 confidence → risk score 50 (moderate risk)
	// No education provided → risk score 30 (neutral)

	riskScore := 50 - (result.ConfidenceScore / 2)
	if riskScore < 0 {
		riskScore = 0
	}
	return riskScore
}
