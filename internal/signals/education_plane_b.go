package signals

import (
	"context"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
	"github.com/nightkiller1977-del/trustgraph/internal/verification"
)

// EducationProvider evaluates education data from Plane B (LinkedIn OAuth)
// This is a Plane B signal - requires user consent to collect education data
type EducationProvider struct {
	validator    *verification.EducationValidator
	eduRepo      *store.EducationRepository
}

func NewEducationProvider(eduRepo *store.EducationRepository) *EducationProvider {
	return &EducationProvider{
		validator:    verification.NewEducationValidator(),
		eduRepo:      eduRepo,
	}
}

func (p *EducationProvider) Name() string {
	return "education"
}

// Evaluate performs free education validation
// Returns a signal result that can be incorporated into policy scoring
func (p *EducationProvider) EvaluateEducation(
	ctx context.Context,
	subjectID string,
	edu verification.EducationData,
	currentJobTitle string,
	accountAgeHours int,
) SignalResult {

	result := SignalResult{
		Provider: p.Name(),
	}

	// Run free validation
	validationResult := p.validator.Validate(ctx, edu, "", currentJobTitle, accountAgeHours)

	// Convert validation result to signal result
	// Higher confidence score = lower risk = lower signal score
	result.Score = p.validator.CalculateEducationRiskScore(validationResult)
	result.Confidence = float64(validationResult.ConfidenceScore) / 100.0

	// Map validation signals to reason codes
	result.ReasonCodes = p.mapValidationSignals(validationResult)

	// Additional context
	if validationResult.IsVerified {
		result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeEducationVerified)
	} else {
		result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeEducationSelfReported)
	}

	return result
}

// mapValidationSignals converts validation signals to reason codes
func (p *EducationProvider) mapValidationSignals(validationResult verification.EducationValidationResult) []string {
	var reasonCodes []string

	signalMap := map[string]string{
		"TIMELINE_PLAUSIBLE":      models.ReasonCodeEducationTimelinePlausible,
		"KNOWN_UNIVERSITY":        models.ReasonCodeEducationKnownUniversity,
		"DEGREE_CAREER_ALIGNED":   models.ReasonCodeEducationCareerAligned,
		"RECENT_GRADUATE":         models.ReasonCodeEducationRecentGraduate,
		"GPA_DISCLOSED":           models.ReasonCodeEducationGPADisclosed,
	}

	for _, signal := range validationResult.Signals {
		if reasonCode, exists := signalMap[signal]; exists {
			reasonCodes = append(reasonCodes, reasonCode)
		}
	}

	return reasonCodes
}
