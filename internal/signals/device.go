package signals

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// DeviceProvider evaluates device fingerprint sharing across accounts.
type DeviceProvider struct{}

func (p *DeviceProvider) Name() string {
	return "device"
}

func (p *DeviceProvider) Evaluate(ctx context.Context, evalCtx *EvalContext, db *store.PostgresDB) SignalResult {
	result := SignalResult{
		Provider: p.Name(),
	}

	// No fingerprint provided -- treat as first-seen with low confidence
	if evalCtx.DeviceFingerprint == "" {
		result.ReasonCodes = []string{models.ReasonCodeDeviceFirstSeen}
		result.Score = 5
		result.Confidence = 0.5
		return result
	}

	// Query observation table for other subjects that share this device fingerprint.
	// We look for observations whose source_data contains the same fingerprint
	// and belong to a different subject, then join with the most recent assessment
	// to determine the trust tier of the other account.
	query := `
		SELECT a.trust_tier
		FROM observation o
		JOIN assessment a ON a.subject_id = o.subject_id
		WHERE o.observation_type = 'device_fingerprint'
		  AND o.source_data->>'fingerprint' = $1
		  AND o.subject_id != $2::uuid
		ORDER BY a.created_at DESC
		LIMIT 1
	`

	var trustTier string
	err := db.QueryRowContext(ctx, query, evalCtx.DeviceFingerprint, evalCtx.SubjectID).Scan(&trustTier)

	if err != nil && err != sql.ErrNoRows {
		result.Error = fmt.Errorf("device fingerprint lookup: %w", err)
		result.ReasonCodes = []string{models.ReasonCodeDeviceFirstSeen}
		result.Score = 5
		result.Confidence = 0.5
		return result
	}

	if err == sql.ErrNoRows {
		// No prior subject with this fingerprint -- first-seen
		result.ReasonCodes = []string{models.ReasonCodeDeviceFirstSeen}
		result.Score = 0
		result.Confidence = 0.8
		return result
	}

	// Fingerprint found on another account -- check trust tier
	if trustTier == models.TrustTierLimited {
		result.ReasonCodes = []string{models.ReasonCodeDeviceSharedWithEnforced}
		result.Score = 30
		result.Confidence = 0.8
		return result
	}

	// Shared with standard or elevated tier account
	result.ReasonCodes = []string{models.ReasonCodeDeviceSharedWithStandard}
	result.Score = 10
	result.Confidence = 0.8
	return result
}
