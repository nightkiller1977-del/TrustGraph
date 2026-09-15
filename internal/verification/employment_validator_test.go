package verification

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func daysAgo(n int) time.Time { return time.Now().AddDate(0, 0, -n) }

func TestEmploymentValidator_EmptyHistory(t *testing.T) {
	v := NewEmploymentValidator()
	result := v.Validate(context.Background(), nil, 1000)

	assert.Equal(t, 0, result.ConfidenceScore)
	assert.False(t, result.IsVerified)
	assert.Equal(t, "No employment", result.Badge)
}

func TestEmploymentValidator_StrongHistory(t *testing.T) {
	v := NewEmploymentValidator()
	jobs := []EmploymentData{
		{CompanyName: "Google", Title: "Senior Software Engineer", StartDate: daysAgo(365 * 4), IsCurrent: true},
		{CompanyName: "Acme Corp", Title: "Software Engineer", StartDate: daysAgo(365 * 7), EndDate: daysAgo(365 * 4)},
	}

	result := v.Validate(context.Background(), jobs, 8760)

	assert.GreaterOrEqual(t, result.ConfidenceScore, 70)
	assert.True(t, result.IsVerified)
	assert.Contains(t, result.Signals, "KNOWN_EMPLOYER")
	assert.Contains(t, result.Signals, "TENURE_ESTABLISHED")
	assert.Contains(t, result.Signals, "TITLE_PROGRESSION")
}

func TestEmploymentValidator_FutureStartDateFailsTimeline(t *testing.T) {
	v := NewEmploymentValidator()
	jobs := []EmploymentData{
		{CompanyName: "Google", Title: "Engineer", StartDate: time.Now().AddDate(0, 0, 30), IsCurrent: true},
	}

	result := v.Validate(context.Background(), jobs, 100)

	assert.NotContains(t, result.Signals, "EMPLOYMENT_TIMELINE_PLAUSIBLE")
	assert.False(t, result.IsVerified)
}

func TestEmploymentValidator_OverlappingRolesRejected(t *testing.T) {
	v := NewEmploymentValidator()
	jobs := []EmploymentData{
		{CompanyName: "A", Title: "Engineer", StartDate: daysAgo(365), EndDate: daysAgo(100)},
		{CompanyName: "B", Title: "Engineer", StartDate: daysAgo(200), EndDate: daysAgo(50)},
	}

	result := v.Validate(context.Background(), jobs, 1000)
	assert.NotContains(t, result.Signals, "EMPLOYMENT_TIMELINE_CONSISTENT")
}

func TestEmploymentValidator_ShortOverlapAllowed(t *testing.T) {
	v := NewEmploymentValidator()
	// A two-week overlap around a job change is normal and must not be flagged.
	jobs := []EmploymentData{
		{CompanyName: "A", Title: "Engineer", StartDate: daysAgo(400), EndDate: daysAgo(200)},
		{CompanyName: "B", Title: "Engineer", StartDate: daysAgo(214), EndDate: daysAgo(50)},
	}

	result := v.Validate(context.Background(), jobs, 1000)
	assert.Contains(t, result.Signals, "EMPLOYMENT_TIMELINE_CONSISTENT")
}

func TestEmploymentValidator_RiskScoreInverted(t *testing.T) {
	v := NewEmploymentValidator()
	high := EmploymentValidationResult{ConfidenceScore: 90}
	low := EmploymentValidationResult{ConfidenceScore: 10}

	assert.Less(t, v.CalculateEmploymentRiskScore(high), v.CalculateEmploymentRiskScore(low))
	assert.Equal(t, 0, v.CalculateEmploymentRiskScore(EmploymentValidationResult{ConfidenceScore: 200}))
}
