package policy

import "github.com/nightkiller1977-del/trustgraph/internal/models"

// SimulationInput pairs a past assessment with its ground-truth reviewer outcome.
type SimulationInput struct {
	RiskScore int
	Outcome   models.ReviewOutcome
}

// SimulationResult is the accuracy at a single risk-score threshold.
type SimulationResult struct {
	Threshold         int     `json:"threshold"`
	TotalAssessments  int     `json:"totalAssessments"`
	TruePositives     int     `json:"truePositives"`
	FalsePositives    int     `json:"falsePositives"`
	TrueNegatives     int     `json:"trueNegatives"`
	FalseNegatives    int     `json:"falseNegatives"`
	FalsePositiveRate float64 `json:"falsePositiveRate"`
	FalseNegativeRate float64 `json:"falseNegativeRate"`
	Precision         float64 `json:"precision"`
	Recall            float64 `json:"recall"`
}

// SimulateThreshold calculates accuracy metrics if the risk-score threshold were
// set to threshold — i.e., risk_score >= threshold → flag (deny/review).
func SimulateThreshold(inputs []SimulationInput, threshold int) SimulationResult {
	r := SimulationResult{
		Threshold:        threshold,
		TotalAssessments: len(inputs),
	}
	totalAbuse := 0
	totalLegit := 0

	for _, in := range inputs {
		isAbuse := in.Outcome == models.ReviewOutcomeConfirmedAbuse
		isFlagged := in.RiskScore >= threshold

		if isAbuse {
			totalAbuse++
		} else if in.Outcome == models.ReviewOutcomeLegitimate {
			totalLegit++
		}

		switch {
		case isFlagged && isAbuse:
			r.TruePositives++
		case isFlagged && !isAbuse:
			r.FalsePositives++
		case !isFlagged && isAbuse:
			r.FalseNegatives++
		case !isFlagged && !isAbuse:
			r.TrueNegatives++
		}
	}

	if totalLegit > 0 {
		r.FalsePositiveRate = float64(r.FalsePositives) / float64(totalLegit)
	}
	if totalAbuse > 0 {
		r.FalseNegativeRate = float64(r.FalseNegatives) / float64(totalAbuse)
		r.Recall = float64(r.TruePositives) / float64(totalAbuse)
	}
	flagged := r.TruePositives + r.FalsePositives
	if flagged > 0 {
		r.Precision = float64(r.TruePositives) / float64(flagged)
	}

	return r
}

// SimulatePolicy runs SimulateThreshold over all supplied thresholds.
func SimulatePolicy(inputs []SimulationInput, thresholds []int) []SimulationResult {
	results := make([]SimulationResult, len(thresholds))
	for i, t := range thresholds {
		results[i] = SimulateThreshold(inputs, t)
	}
	return results
}
