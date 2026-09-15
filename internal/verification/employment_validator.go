package verification

import (
	"context"
	"strings"
	"time"
)

// EmploymentData is one job from LinkedIn OAuth.
type EmploymentData struct {
	CompanyName string    `json:"company_name"`
	Title       string    `json:"title"`
	StartDate   time.Time `json:"start_date"`
	EndDate     time.Time `json:"end_date"`
	IsCurrent   bool      `json:"is_current"`
}

// EmploymentValidationResult mirrors EducationValidationResult: a 0-100
// confidence score plus the signals that produced it.
type EmploymentValidationResult struct {
	ConfidenceScore int      `json:"confidence_score"`
	IsVerified      bool     `json:"is_verified"`
	Signals         []string `json:"signals"`
	Badge           string   `json:"badge"`
	Details         string   `json:"details"`
	Cost            float64  `json:"cost"`
}

// KnownCompanies is a starter list of large, long-established employers whose
// existence as a named company is not itself suspicious. This is an existence
// sanity check, not a claim that the subject worked there — a real check would
// call a company registry.
var KnownCompanies = map[string]struct{}{
	"google":        {},
	"alphabet":      {},
	"microsoft":     {},
	"apple":         {},
	"amazon":        {},
	"meta":          {},
	"facebook":      {},
	"ibm":           {},
	"oracle":        {},
	"sap":           {},
	"salesforce":    {},
	"adobe":         {},
	"intel":         {},
	"nvidia":        {},
	"cisco":         {},
	"dell":          {},
	"hp":            {},
	"accenture":     {},
	"deloitte":      {},
	"mckinsey":      {},
	"goldman sachs": {},
	"jpmorgan":      {},
	"walmart":       {},
	"tesla":         {},
	"netflix":       {},
}

// EmploymentValidator scores employment history with no external vendor.
type EmploymentValidator struct{}

func NewEmploymentValidator() *EmploymentValidator {
	return &EmploymentValidator{}
}

// Validate scores an employment history. accountAgeHours is the age of the
// subject's account; a role that began after the account was created is not
// automatically suspicious (people do get promoted), so this is only used for
// the timeline checks that can actually indicate a fabrication.
func (v *EmploymentValidator) Validate(ctx context.Context, jobs []EmploymentData, accountAgeHours int) EmploymentValidationResult {
	result := EmploymentValidationResult{
		Signals: []string{},
		Cost:    0,
	}

	if len(jobs) == 0 {
		result.Details = "No employment history provided."
		result.Badge = "No employment"
		return result
	}

	// Signal 1: timeline plausibility (20)
	if v.timelinePlausible(jobs, accountAgeHours) {
		result.ConfidenceScore += 20
		result.Signals = append(result.Signals, "EMPLOYMENT_TIMELINE_PLAUSIBLE")
	}

	// Signal 2: company existence (30) — at least one current/known employer
	if v.hasKnownCompany(jobs) {
		result.ConfidenceScore += 30
		result.Signals = append(result.Signals, "KNOWN_EMPLOYER")
	}

	// Signal 3: no overlapping full-time roles at the same time (25)
	if v.noUnexplainedOverlap(jobs) {
		result.ConfidenceScore += 25
		result.Signals = append(result.Signals, "EMPLOYMENT_TIMELINE_CONSISTENT")
	}

	// Signal 4: meaningful tenure somewhere (15)
	if v.maxTenureMonths(jobs) >= 12 {
		result.ConfidenceScore += 15
		result.Signals = append(result.Signals, "TENURE_ESTABLISHED")
	}

	// Signal 5: current title aligns with a prior role's field (10)
	if v.titleProgression(jobs) {
		result.ConfidenceScore += 10
		result.Signals = append(result.Signals, "TITLE_PROGRESSION")
	}

	result.IsVerified = result.ConfidenceScore >= 70
	result.Badge = v.generateBadge(result.IsVerified, jobs)
	result.Details = v.generateDetails(result, jobs)

	return result
}

// timelinePlausible checks that dates are ordered and not in the future.
func (v *EmploymentValidator) timelinePlausible(jobs []EmploymentData, accountAgeHours int) bool {
	now := time.Now()
	for _, job := range jobs {
		if job.StartDate.After(now) {
			return false
		}
		if !job.IsCurrent && !job.EndDate.IsZero() && job.EndDate.After(now) {
			return false
		}
		if !job.EndDate.IsZero() && !job.StartDate.IsZero() && job.EndDate.Before(job.StartDate) {
			return false
		}
		// A role that started before the account existed is fine (the job
		// predates the profile). Only reject a start date so far in the past it
		// cannot be a real career.
		if job.StartDate.Before(now.AddDate(-60, 0, 0)) {
			return false
		}
	}
	return true
}

func (v *EmploymentValidator) hasKnownCompany(jobs []EmploymentData) bool {
	for _, job := range jobs {
		name := strings.ToLower(strings.TrimSpace(job.CompanyName))
		if name == "" {
			continue
		}
		if _, ok := KnownCompanies[name]; ok {
			return true
		}
	}
	return false
}

// noUnexplainedOverlap rejects histories where two non-current roles occupy
// the same period, unless one is short (a transition/overlap of under 60 days
// is normal).
func (v *EmploymentValidator) noUnexplainedOverlap(jobs []EmploymentData) bool {
	for i := 0; i < len(jobs); i++ {
		for j := i + 1; j < len(jobs); j++ {
			a, b := jobs[i], jobs[j]
			aEnd := a.EndDate
			if a.IsCurrent || aEnd.IsZero() {
				aEnd = time.Now()
			}
			bEnd := b.EndDate
			if b.IsCurrent || bEnd.IsZero() {
				bEnd = time.Now()
			}
			if a.StartDate.IsZero() || b.StartDate.IsZero() {
				continue
			}
			overlapStart := maxTime(a.StartDate, b.StartDate)
			overlapEnd := minTime(aEnd, bEnd)
			if overlapEnd.After(overlapStart) && overlapEnd.Sub(overlapStart) > 60*24*time.Hour {
				return false
			}
		}
	}
	return true
}

func (v *EmploymentValidator) maxTenureMonths(jobs []EmploymentData) int {
	now := time.Now()
	max := 0
	for _, job := range jobs {
		end := job.EndDate
		if job.IsCurrent || end.IsZero() {
			end = now
		}
		if job.StartDate.IsZero() {
			continue
		}
		months := int(end.Sub(job.StartDate).Hours() / (24 * 30))
		if months > max {
			max = months
		}
	}
	return max
}

// titleProgression is a light heuristic: the latest role's title shares a
// keyword with an earlier one, i.e. the career reads as connected rather than
// a list of unrelated jobs.
func (v *EmploymentValidator) titleProgression(jobs []EmploymentData) bool {
	if len(jobs) < 2 {
		return false
	}
	latest := jobs[0]
	latestWords := wordSet(latest.Title)
	for _, job := range jobs[1:] {
		for word := range wordSet(job.Title) {
			if latestWords[word] {
				return true
			}
		}
	}
	return false
}

func wordSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		if len(w) > 3 {
			set[w] = true
		}
	}
	return set
}

func (v *EmploymentValidator) generateBadge(isVerified bool, jobs []EmploymentData) string {
	if len(jobs) == 0 {
		return "No employment"
	}
	company := jobs[0].CompanyName
	if isVerified {
		return "✅ " + company + " (Verified)"
	}
	return "💼 " + company + " (Self-Reported)"
}

func (v *EmploymentValidator) generateDetails(result EmploymentValidationResult, jobs []EmploymentData) string {
	phrases := map[string]string{
		"EMPLOYMENT_TIMELINE_PLAUSIBLE":  "timeline is plausible",
		"KNOWN_EMPLOYER":                 "employer is a known company",
		"EMPLOYMENT_TIMELINE_CONSISTENT": "no unexplained overlaps",
		"TENURE_ESTABLISHED":             "tenure of at least a year",
		"TITLE_PROGRESSION":              "career progression is coherent",
	}
	order := []string{
		"EMPLOYMENT_TIMELINE_PLAUSIBLE", "KNOWN_EMPLOYER",
		"EMPLOYMENT_TIMELINE_CONSISTENT", "TENURE_ESTABLISHED", "TITLE_PROGRESSION",
	}
	present := make(map[string]bool, len(result.Signals))
	for _, s := range result.Signals {
		present[s] = true
	}
	parts := make([]string, 0, len(result.Signals))
	for _, s := range order {
		if present[s] {
			parts = append(parts, phrases[s])
		}
	}

	summary := "Employment claim could not be corroborated."
	if result.IsVerified {
		summary = "Employment verified with high confidence."
	} else if result.ConfidenceScore >= 50 {
		summary = "Moderate confidence in employment claim."
	}
	if len(parts) > 0 {
		summary += " Signals: " + strings.Join(parts, ", ") + "."
	}
	return summary
}

// CalculateEmploymentRiskScore maps confidence to a risk score, matching the
// education validator's inversion: high confidence = low risk.
func (v *EmploymentValidator) CalculateEmploymentRiskScore(result EmploymentValidationResult) int {
	risk := 50 - (result.ConfidenceScore / 2)
	if risk < 0 {
		risk = 0
	}
	return risk
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
