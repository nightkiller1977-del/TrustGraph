package signals

import (
	"context"

	"github.com/google/uuid"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
	"github.com/nightkiller1977-del/trustgraph/internal/verification"
)

// EmploymentProvider turns consented employment history (LinkedIn OAuth) into
// an assessment signal using the free EmploymentValidator.
type EmploymentProvider struct {
	validator *verification.EmploymentValidator
	repo      *store.EmploymentRepository
}

func NewEmploymentProvider(repo *store.EmploymentRepository) *EmploymentProvider {
	return &EmploymentProvider{
		validator: verification.NewEmploymentValidator(),
		repo:      repo,
	}
}

func (p *EmploymentProvider) Name() string {
	return "employment"
}

// Evaluate loads and scores the subject's employment history. Like the
// education provider it reports ErrNoPlaneBData rather than inventing a score
// when nothing is stored.
func (p *EmploymentProvider) Evaluate(ctx context.Context, evalCtx *EvalContext, _ *store.PostgresDB) SignalResult {
	if p.repo == nil || evalCtx == nil || evalCtx.SubjectID == "" {
		return SignalResult{Provider: p.Name(), Error: ErrNoPlaneBData}
	}

	subjectID, err := uuid.Parse(evalCtx.SubjectID)
	if err != nil {
		return SignalResult{Provider: p.Name(), Error: err}
	}

	jobs, err := p.repo.ListBySubject(ctx, subjectID)
	if err != nil {
		return SignalResult{Provider: p.Name(), Error: err}
	}
	if len(jobs) == 0 {
		return SignalResult{Provider: p.Name(), Error: ErrNoPlaneBData}
	}

	data := make([]verification.EmploymentData, 0, len(jobs))
	for _, job := range jobs {
		data = append(data, verification.EmploymentData{
			CompanyName: job.CompanyName,
			Title:       job.Title,
			StartDate:   job.StartDate,
			EndDate:     job.EndDate,
			IsCurrent:   job.IsCurrent,
		})
	}

	validation := p.validator.Validate(ctx, data, evalCtx.AccountAgeHours)
	return signalResultFromEmployment(p.validator, validation)
}

// EvaluateEmployment scores an in-memory history (used by the verification
// endpoint right after a LinkedIn sync).
func (p *EmploymentProvider) EvaluateEmployment(ctx context.Context, jobs []verification.EmploymentData, accountAgeHours int) SignalResult {
	validation := p.validator.Validate(ctx, jobs, accountAgeHours)
	return signalResultFromEmployment(p.validator, validation)
}

func signalResultFromEmployment(v *verification.EmploymentValidator, validation verification.EmploymentValidationResult) SignalResult {
	result := SignalResult{
		Provider:   "employment",
		Score:      v.CalculateEmploymentRiskScore(validation),
		Confidence: float64(validation.ConfidenceScore) / 100.0,
	}
	result.ReasonCodes = mapEmploymentSignals(validation.Signals)
	if validation.IsVerified {
		result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeEmploymentVerified)
	} else {
		result.ReasonCodes = append(result.ReasonCodes, models.ReasonCodeEmploymentSelfReported)
	}
	return result
}

var employmentSignalToReasonCode = map[string]string{
	"EMPLOYMENT_TIMELINE_PLAUSIBLE":  models.ReasonCodeEmploymentTimelinePlausible,
	"KNOWN_EMPLOYER":                 models.ReasonCodeEmploymentKnownEmployer,
	"EMPLOYMENT_TIMELINE_CONSISTENT": models.ReasonCodeEmploymentTimelineConsistent,
	"TENURE_ESTABLISHED":             models.ReasonCodeEmploymentTenureEstablished,
	"TITLE_PROGRESSION":              models.ReasonCodeEmploymentTitleProgression,
}

func mapEmploymentSignals(signals []string) []string {
	codes := make([]string, 0, len(signals))
	for _, s := range signals {
		if code, ok := employmentSignalToReasonCode[s]; ok {
			codes = append(codes, code)
		}
	}
	return codes
}
