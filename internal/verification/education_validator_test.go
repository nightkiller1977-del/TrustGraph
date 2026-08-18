package verification

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestEducationValidator_TimelinePlausibility(t *testing.T) {
	v := NewEducationValidator()

	tests := []struct {
		name        string
		edu         EducationData
		accountHours int
		want        bool
	}{
		{
			name: "valid timeline - graduated 2 years ago",
			edu: EducationData{
				StartDate: time.Now().AddDate(-4, 0, 0),
				EndDate:   time.Now().AddDate(-2, 0, 0),
			},
			accountHours: 24,
			want:         true,
		},
		{
			name: "invalid - school duration too short (< 1 year)",
			edu: EducationData{
				StartDate: time.Now().AddDate(-1, 0, 0),
				EndDate:   time.Now().AddDate(-1, 0, 6*30), // 6 months later
			},
			accountHours: 24,
			want:         false,
		},
		{
			name: "invalid - school duration too long (> 8 years)",
			edu: EducationData{
				StartDate: time.Now().AddDate(-10, 0, 0),
				EndDate:   time.Now().AddDate(-2, 0, 0),
			},
			accountHours: 24,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.validateTimeline(tt.edu, tt.accountHours)
			if got != tt.want {
				t.Errorf("validateTimeline() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEducationValidator_RealUniversity(t *testing.T) {
	v := NewEducationValidator()

	tests := []struct {
		name       string
		schoolName string
		want       bool
	}{
		{"Stanford", "Stanford University", true},
		{"MIT", "MIT", true},
		{"Berkeley", "University of California, Berkeley", true},
		{"Harvard", "Harvard University", true},
		{"Fake School", "Fake School of Dreams", false},
		{"Generic University", "University", true}, // Generic match
		{"Generic College", "College", true},
		{"Generic Institute", "Institute", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.isRealUniversity(tt.schoolName)
			if got != tt.want {
				t.Errorf("isRealUniversity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEducationValidator_DegreeCareerAlignment(t *testing.T) {
	v := NewEducationValidator()

	tests := []struct {
		name           string
		fieldOfStudy   string
		currentJobTitle string
		want           bool
	}{
		{
			name:            "CS degree + Software Engineer",
			fieldOfStudy:    "Computer Science",
			currentJobTitle: "Software Engineer",
			want:            true,
		},
		{
			name:            "MBA + Finance Manager",
			fieldOfStudy:    "Business Administration",
			currentJobTitle: "Finance Manager",
			want:            true,
		},
		{
			name:            "Engineering + Tech role",
			fieldOfStudy:    "Mechanical Engineering",
			currentJobTitle: "Tech Lead",
			want:            true,
		},
		{
			name:            "Misaligned - Accounting degree + Software Engineer",
			fieldOfStudy:    "Accounting",
			currentJobTitle: "Software Engineer",
			want:            false,
		},
		{
			name:            "Misaligned - Biology + Finance",
			fieldOfStudy:    "Biology",
			currentJobTitle: "Finance Analyst",
			want:            false,
		},
		{
			name:            "Generic - both mention tech",
			fieldOfStudy:    "Technology",
			currentJobTitle: "Tech Lead",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.degreeMatchesCareer(tt.fieldOfStudy, tt.currentJobTitle)
			if got != tt.want {
				t.Errorf("degreeMatchesCareer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEducationValidator_FullValidation(t *testing.T) {
	v := NewEducationValidator()
	ctx := context.Background()

	tests := []struct {
		name              string
		edu               EducationData
		currentJobTitle   string
		accountAgeHours   int
		minConfidence     int
		shouldBeVerified  bool
	}{
		{
			name: "perfect profile - Stanford CS -> Software Engineer",
			edu: EducationData{
				SchoolName:   "Stanford University",
				FieldOfStudy: "Computer Science",
				StartDate:    time.Now().AddDate(-4, 0, 0),
				EndDate:      time.Now().AddDate(-1, 0, 0),
				Grade:        "3.8",
			},
			currentJobTitle:  "Software Engineer at Google",
			accountAgeHours:  48,
			minConfidence:    70,  // Realistic: will get 4-5 signals
			shouldBeVerified: true,
		},
		{
			name: "good profile - Berkeley Engineering -> Tech",
			edu: EducationData{
				SchoolName:   "UC Berkeley",
				FieldOfStudy: "Electrical Engineering",
				StartDate:    time.Now().AddDate(-5, 0, 0),
				EndDate:      time.Now().AddDate(-3, 0, 0),
				Grade:        "3.5",
			},
			currentJobTitle:  "Tech Lead",
			accountAgeHours:  72,
			minConfidence:    70,
			shouldBeVerified: true,
		},
		{
			name: "weak profile - no GPA, older education",
			edu: EducationData{
				SchoolName:   "Harvard University",
				FieldOfStudy: "Business",
				StartDate:    time.Now().AddDate(-20, 0, 0),
				EndDate:      time.Now().AddDate(-12, 0, 0),
				Grade:        "",
			},
			currentJobTitle:  "Manager",
			accountAgeHours:  24,
			minConfidence:    30,  // Will get timeline + known university = 50, minus not recent = 30-40
			shouldBeVerified: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(ctx, tt.edu, "user@example.com", tt.currentJobTitle, tt.accountAgeHours)

			if result.ConfidenceScore < tt.minConfidence {
				t.Errorf("confidence score %d < min %d", result.ConfidenceScore, tt.minConfidence)
			}

			if result.IsVerified != tt.shouldBeVerified {
				t.Errorf("IsVerified = %v, want %v", result.IsVerified, tt.shouldBeVerified)
			}

			// Verify badge is set
			if result.Badge == "" {
				t.Error("Badge should not be empty")
			}

			// Verify details are set
			if result.Details == "" {
				t.Error("Details should not be empty")
			}

			// Verify signals are populated
			if len(result.Signals) == 0 {
				t.Error("Signals should be populated")
			}
		})
	}
}

// A profile can cross the verified threshold (70) without every signal
// passing — known school (30) + career aligned (25) + recent grad (15) +
// GPA (10) = 80, with timeline plausibility never checked at all (a
// too-short 6-month program). generateDetails must not claim "timeline is
// plausible" here, and must only mention signals that actually fired.
func TestEducationValidator_DetailsOnlyClaimSignalsThatFired(t *testing.T) {
	v := NewEducationValidator()
	ctx := context.Background()

	edu := EducationData{
		SchoolName:   "Stanford University",
		FieldOfStudy: "Computer Science",
		StartDate:    time.Now().AddDate(0, -6, 0),
		EndDate:      time.Now().AddDate(0, -1, 0), // 5-month duration: fails the 1-8 year timeline check
		Grade:        "3.8",
	}

	result := v.Validate(ctx, edu, "user@example.com", "Software Engineer at Google", 24)

	for _, sig := range result.Signals {
		if sig == "TIMELINE_PLAUSIBLE" {
			t.Fatalf("expected TIMELINE_PLAUSIBLE to be absent for a 5-month program, got signals %v", result.Signals)
		}
	}
	if !result.IsVerified {
		t.Fatalf("expected verified (score %d) despite the missing timeline signal", result.ConfidenceScore)
	}
	if strings.Contains(strings.ToLower(result.Details), "timeline is plausible") {
		t.Errorf("Details claims timeline is plausible when TIMELINE_PLAUSIBLE never fired: %q", result.Details)
	}
	if !strings.Contains(result.Details, "school is known") {
		t.Errorf("Details should mention the KNOWN_UNIVERSITY signal that did fire: %q", result.Details)
	}
}

func TestEducationValidator_RiskScore(t *testing.T) {
	v := NewEducationValidator()

	result := EducationValidationResult{
		ConfidenceScore: 90,
		IsVerified:      true,
	}

	riskScore := v.CalculateEducationRiskScore(result)
	if riskScore > 10 {
		t.Errorf("High confidence should yield low risk score, got %d", riskScore)
	}

	result.ConfidenceScore = 0
	result.IsVerified = false
	riskScore = v.CalculateEducationRiskScore(result)
	if riskScore < 40 {
		t.Errorf("Low confidence should yield moderate risk score, got %d", riskScore)
	}
}

func TestEducationValidator_RecentGraduate(t *testing.T) {
	v := NewEducationValidator()

	recentGrad := time.Now().AddDate(-2, 0, 0)
	if !v.isRecentGraduate(recentGrad) {
		t.Error("Should identify recent graduate (2 years ago)")
	}

	oldGrad := time.Now().AddDate(-15, 0, 0)
	if v.isRecentGraduate(oldGrad) {
		t.Error("Should not identify old graduate (15 years ago)")
	}

	futureGrad := time.Now().AddDate(1, 0, 0)
	if v.isRecentGraduate(futureGrad) {
		t.Error("Should not identify a future graduation date as a recent graduate")
	}
}

func TestEducationValidator_DegreeCareerAlignment_RequiresTermInBothFields(t *testing.T) {
	v := NewEducationValidator()

	// "data" appears twice in fieldOfStudy and zero times in the job title —
	// the loose-match term must be required in EACH field separately, not
	// just twice across the concatenated pair.
	if v.degreeMatchesCareer("Data Science and Data Analytics", "Chef") {
		t.Error("expected no alignment: 'data' appears twice in the field but not at all in the job title")
	}

	if !v.degreeMatchesCareer("Data Science", "Data Analyst") {
		t.Error("expected alignment: 'data' appears in both fields")
	}
}
