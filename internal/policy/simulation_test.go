package policy

import (
	"testing"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
)

func TestSimulateThreshold_AllAbuse(t *testing.T) {
	inputs := []SimulationInput{
		{RiskScore: 90, Outcome: models.ReviewOutcomeConfirmedAbuse},
		{RiskScore: 75, Outcome: models.ReviewOutcomeConfirmedAbuse},
		{RiskScore: 50, Outcome: models.ReviewOutcomeConfirmedAbuse},
	}
	r := SimulateThreshold(inputs, 80)
	if r.TruePositives != 1 {
		t.Errorf("expected 1 TP, got %d", r.TruePositives)
	}
	if r.FalseNegatives != 2 {
		t.Errorf("expected 2 FN, got %d", r.FalseNegatives)
	}
	// FP rate = 0 (no legit accounts)
	if r.FalsePositiveRate != 0 {
		t.Errorf("expected 0 FP rate, got %f", r.FalsePositiveRate)
	}
}

func TestSimulateThreshold_AllLegit(t *testing.T) {
	inputs := []SimulationInput{
		{RiskScore: 90, Outcome: models.ReviewOutcomeLegitimate},
		{RiskScore: 30, Outcome: models.ReviewOutcomeLegitimate},
	}
	r := SimulateThreshold(inputs, 80)
	if r.FalsePositives != 1 {
		t.Errorf("expected 1 FP, got %d", r.FalsePositives)
	}
	// FP rate = 1 FP / 2 legit = 0.5
	if r.FalsePositiveRate != 0.5 {
		t.Errorf("expected 0.5 FP rate, got %f", r.FalsePositiveRate)
	}
	if r.TrueNegatives != 1 {
		t.Errorf("expected 1 TN, got %d", r.TrueNegatives)
	}
}

func TestSimulateThreshold_Mixed(t *testing.T) {
	inputs := []SimulationInput{
		{RiskScore: 85, Outcome: models.ReviewOutcomeConfirmedAbuse}, // TP
		{RiskScore: 60, Outcome: models.ReviewOutcomeConfirmedAbuse}, // FN
		{RiskScore: 90, Outcome: models.ReviewOutcomeLegitimate},     // FP
		{RiskScore: 20, Outcome: models.ReviewOutcomeLegitimate},     // TN
	}
	r := SimulateThreshold(inputs, 80)

	if r.TruePositives != 1 || r.FalseNegatives != 1 || r.FalsePositives != 1 || r.TrueNegatives != 1 {
		t.Errorf("unexpected confusion matrix: TP=%d FN=%d FP=%d TN=%d",
			r.TruePositives, r.FalseNegatives, r.FalsePositives, r.TrueNegatives)
	}
	// FP rate = 1/2 = 0.5
	if r.FalsePositiveRate != 0.5 {
		t.Errorf("expected FP rate 0.5, got %f", r.FalsePositiveRate)
	}
	// FN rate = 1/2 = 0.5
	if r.FalseNegativeRate != 0.5 {
		t.Errorf("expected FN rate 0.5, got %f", r.FalseNegativeRate)
	}
}

func TestSimulatePolicy_MultipleThresholds(t *testing.T) {
	inputs := []SimulationInput{
		{RiskScore: 85, Outcome: models.ReviewOutcomeConfirmedAbuse},
		{RiskScore: 55, Outcome: models.ReviewOutcomeConfirmedAbuse},
		{RiskScore: 30, Outcome: models.ReviewOutcomeLegitimate},
		{RiskScore: 75, Outcome: models.ReviewOutcomeLegitimate},
	}
	results := SimulatePolicy(inputs, []int{50, 80})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Threshold != 50 || results[1].Threshold != 80 {
		t.Errorf("unexpected threshold order: %d %d", results[0].Threshold, results[1].Threshold)
	}
	// At threshold 50: both abuse flagged (2 TP), 1 legit flagged (1 FP)
	if results[0].TruePositives != 2 {
		t.Errorf("threshold 50: expected 2 TP, got %d", results[0].TruePositives)
	}
}

func TestSimulateThreshold_Empty(t *testing.T) {
	r := SimulateThreshold(nil, 70)
	if r.TotalAssessments != 0 || r.FalsePositiveRate != 0 {
		t.Error("empty inputs should produce zero metrics")
	}
}
