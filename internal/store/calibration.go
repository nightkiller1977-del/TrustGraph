package store

import (
	"context"
	"fmt"
)

// CalibrationRepository computes accuracy metrics from reviewed assessments.
type CalibrationRepository struct {
	db *PostgresDB
}

func NewCalibrationRepository(db *PostgresDB) *CalibrationRepository {
	return &CalibrationRepository{db: db}
}

// RiskBandStats holds per-band accuracy numbers.
type RiskBandStats struct {
	Reviews   int     `json:"reviews"`
	AbuseRate float64 `json:"abuseRate"`
}

// ReasonCodeStats holds precision/recall for a single reason code.
type ReasonCodeStats struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
}

// CalibrationMetrics is the full accuracy snapshot returned by /v1/metrics/calibration.
type CalibrationMetrics struct {
	TotalReviews       int                        `json:"totalReviews"`
	FalsePositiveRate  float64                    `json:"falsePositiveRate"`
	FalseNegativeRate  float64                    `json:"falseNegativeRate"`
	AppealOverturnRate float64                    `json:"appealOverturnRate"`
	ByRiskBand         map[string]RiskBandStats   `json:"byRiskBand"`
	ByReasonCode       map[string]ReasonCodeStats `json:"byReasonCode"`
}

// GetCalibrationMetrics computes the full accuracy snapshot. threshold is the
// risk-score cutoff (0-100) a flagged/not-flagged decision is evaluated
// against — pass the same value used by cmd/trustgraph-simulate so the two
// tools report FP/FN rates for the same cutoff instead of incompatible
// definitions (see config.EnforcementThreshold).
func (r *CalibrationRepository) GetCalibrationMetrics(ctx context.Context, threshold int) (*CalibrationMetrics, error) {
	m := &CalibrationMetrics{
		ByRiskBand:   make(map[string]RiskBandStats),
		ByReasonCode: make(map[string]ReasonCodeStats),
	}

	// Total reviews + FP/FN rates, both measured against the same numeric
	// risk_score threshold used by policy.SimulateThreshold.
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*)                                                                            AS total,
			COALESCE(
				COUNT(*) FILTER (WHERE r.outcome = 'legitimate' AND a.risk_score >= $1)
				* 1.0 / NULLIF(COUNT(*) FILTER (WHERE r.outcome = 'legitimate'), 0),
			0)                                                                                  AS fp_rate,
			COALESCE(
				COUNT(*) FILTER (WHERE r.outcome = 'confirmed_abuse' AND a.risk_score < $1)
				* 1.0 / NULLIF(COUNT(*) FILTER (WHERE r.outcome = 'confirmed_abuse'), 0),
			0)                                                                                  AS fn_rate
		FROM assessment a
		JOIN assessment_review r ON a.assessment_id = r.assessment_id
	`, threshold).Scan(&m.TotalReviews, &m.FalsePositiveRate, &m.FalseNegativeRate)
	if err != nil {
		return nil, fmt.Errorf("fp/fn query: %w", err)
	}

	// Appeal overturn rate (appeals where outcome = approved / all resolved appeals)
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(
			COUNT(*) FILTER (WHERE outcome = 'approved') * 1.0 / NULLIF(COUNT(*) FILTER (WHERE outcome != 'pending'), 0),
		0)
		FROM assessment_appeal
	`).Scan(&m.AppealOverturnRate)
	if err != nil {
		return nil, fmt.Errorf("appeal overturn query: %w", err)
	}

	// Per-risk-band stats
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			a.risk_band,
			COUNT(*)                                                                                 AS reviews,
			COALESCE(COUNT(*) FILTER (WHERE r.outcome = 'confirmed_abuse') * 1.0 / NULLIF(COUNT(*), 0), 0) AS abuse_rate
		FROM assessment a
		JOIN assessment_review r ON a.assessment_id = r.assessment_id
		GROUP BY a.risk_band
	`)
	if err != nil {
		return nil, fmt.Errorf("risk band query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var band string
		var stats RiskBandStats
		if err := rows.Scan(&band, &stats.Reviews, &stats.AbuseRate); err != nil {
			return nil, fmt.Errorf("scan risk band: %w", err)
		}
		m.ByRiskBand[band] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("risk band rows: %w", err)
	}

	// Per-reason-code precision/recall
	codeRows, err := r.db.QueryContext(ctx, `
		WITH abuse_total AS (
			SELECT COUNT(*) AS n FROM assessment_review WHERE outcome = 'confirmed_abuse'
		),
		code_stats AS (
			SELECT
				rc                                                                       AS reason_code,
				COUNT(*) FILTER (WHERE r.outcome = 'confirmed_abuse')                   AS tp,
				COUNT(*)                                                                 AS total_with_code
			FROM assessment a
			JOIN assessment_review r ON a.assessment_id = r.assessment_id,
			LATERAL unnest(a.reason_codes) AS rc
			GROUP BY rc
		)
		SELECT
			cs.reason_code,
			COALESCE(cs.tp * 1.0 / NULLIF(cs.total_with_code, 0), 0) AS precision,
			COALESCE(cs.tp * 1.0 / NULLIF(at.n, 0), 0)               AS recall
		FROM code_stats cs, abuse_total at
		ORDER BY cs.tp DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("reason code query: %w", err)
	}
	defer codeRows.Close()
	for codeRows.Next() {
		var code string
		var stats ReasonCodeStats
		if err := codeRows.Scan(&code, &stats.Precision, &stats.Recall); err != nil {
			return nil, fmt.Errorf("scan reason code: %w", err)
		}
		m.ByReasonCode[code] = stats
	}
	if err := codeRows.Err(); err != nil {
		return nil, fmt.Errorf("reason code rows: %w", err)
	}

	return m, nil
}
