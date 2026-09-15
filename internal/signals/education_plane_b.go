package signals

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
	"github.com/nightkiller1977-del/trustgraph/internal/verification"
)

// ErrNoPlaneBData is returned (via SignalResult.Error) when a Plane B provider
// has nothing to evaluate. It is not a failure — the evaluator reports the
// provider as skipped rather than inventing a signal.
var ErrNoPlaneBData = errors.New("no plane B data available for subject")

// EducationProvider turns consented education data (LinkedIn OAuth or manual
// entry) into an assessment signal using the free EducationValidator.
//
// It is intentionally NOT part of the registration-time evaluator: at
// registration a subject cannot yet have consented Plane B data, so running it
// there would only add a query that always comes back empty. Re-assessment
// attaches it instead.
type EducationProvider struct {
	validator *verification.EducationValidator
	eduRepo   *store.EducationRepository
}

// NewEducationProvider builds an education provider backed by the education
// repository. A nil repo yields a provider that always reports no data.
func NewEducationProvider(eduRepo *store.EducationRepository) *EducationProvider {
	return &EducationProvider{
		validator: verification.NewEducationValidator(),
		eduRepo:   eduRepo,
	}
}

func (p *EducationProvider) Name() string {
	return "education"
}

// Evaluate satisfies the Provider interface. It loads the subject's education
// record and scores it; when there is no record (or no subject ID) it reports
// ErrNoPlaneBData so the evaluator marks the signal as skipped instead of
// folding an absent signal into the risk score.
func (p *EducationProvider) Evaluate(ctx context.Context, evalCtx *EvalContext, _ *store.PostgresDB) SignalResult {
	result := SignalResult{Provider: p.Name()}

	if p.eduRepo == nil || evalCtx == nil || evalCtx.SubjectID == "" {
		result.Error = ErrNoPlaneBData
		return result
	}

	subjectID, err := uuid.Parse(evalCtx.SubjectID)
	if err != nil {
		result.Error = err
		return result
	}

	edu, err := p.eduRepo.GetEducationBySubject(ctx, subjectID)
	if err != nil {
		result.Error = err
		return result
	}
	if edu == nil {
		result.Error = ErrNoPlaneBData
		return result
	}

	validation := p.validator.Validate(ctx, verification.EducationData{
		SchoolName:   edu.SchoolName,
		FieldOfStudy: edu.FieldOfStudy,
		StartDate:    edu.StartDate,
		EndDate:      edu.EndDate,
		Grade:        edu.Grade,
	}, evalCtx.Email, evalCtx.CurrentJobTitle, evalCtx.AccountAgeHours)

	return signalResultFromEducation(p.validator, validation)
}

// EvaluateEducation scores an in-memory education record. Verification
// endpoints use it immediately after writing a record so the caller gets a
// result without a second database round-trip.
func (p *EducationProvider) EvaluateEducation(
	ctx context.Context,
	edu verification.EducationData,
	currentJobTitle string,
	accountAgeHours int,
) SignalResult {
	validation := p.validator.Validate(ctx, edu, "", currentJobTitle, accountAgeHours)
	return signalResultFromEducation(p.validator, validation)
}

func signalResultFromEducation(v *verification.EducationValidator, validation verification.EducationValidationResult) SignalResult {
	result := SignalResult{
		Provider:   "education",
		Score:      v.CalculateEducationRiskScore(validation),
		Confidence: float64(validation.ConfidenceScore) / 100.0,
	}

	result.ReasonCodes = mapEducationSignals(validation.Signals)
	if validation.IsVerified {
		result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeEducationVerified)
	} else {
		result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeEducationSelfReported)
	}

	return result
}

var educationSignalToReasonCode = map[string]string{
	"TIMELINE_PLAUSIBLE":    models.ReasonCodeEducationTimelinePlausible,
	"KNOWN_UNIVERSITY":      models.ReasonCodeEducationKnownUniversity,
	"DEGREE_CAREER_ALIGNED": models.ReasonCodeEducationCareerAligned,
	"RECENT_GRADUATE":       models.ReasonCodeEducationRecentGraduate,
	"GPA_DISCLOSED":         models.ReasonCodeEducationGPADisclosed,
}

func mapEducationSignals(signals []string) []string {
	reasonCodes := make([]string, 0, len(signals))
	for _, signal := range signals {
		if code, ok := educationSignalToReasonCode[signal]; ok {
			reasonCodes = append(reasonCodes, code)
		}
	}
	return reasonCodes
}
