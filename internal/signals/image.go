package signals

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nightkiller1977-del/trustgraph/internal/models"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

// ImageProvider evaluates image hash (pHash stub) reuse across subjects.
type ImageProvider struct{}

func (p *ImageProvider) Name() string {
	return "image"
}

func (p *ImageProvider) Evaluate(ctx context.Context, evalCtx *EvalContext, db *store.PostgresDB) SignalResult {
	result := SignalResult{
		Provider: p.Name(),
	}

	// No image hash provided -- nothing meaningful to evaluate
	if evalCtx.ImageHash == "" {
		result.ReasonCodes = []string{models.ReasonCodeImageHashNew}
		result.Score = 0
		result.Confidence = 0.3
		return result
	}

	// Check if this image hash appears in observations for a different subject.
	query := `
		SELECT o.subject_id
		FROM observation o
		WHERE o.observation_type = 'image_hash'
		  AND o.source_data->>'hash' = $1
		  AND o.subject_id != $2::uuid
		LIMIT 1
	`

	var otherSubjectID string
	err := db.QueryRowContext(ctx, query, evalCtx.ImageHash, evalCtx.SubjectID).Scan(&otherSubjectID)

	if err != nil && err != sql.ErrNoRows {
		result.Error = fmt.Errorf("image hash lookup: %w", err)
		result.ReasonCodes = []string{models.ReasonCodeImageHashNew}
		result.Score = 0
		result.Confidence = 0.3
		return result
	}

	if err == sql.ErrNoRows {
		// Hash is new -- not seen before
		result.ReasonCodes = []string{models.ReasonCodeImageHashNew}
		result.Score = 0
		result.Confidence = 0.8
		return result
	}

	// Hash reused by another subject
	result.ReasonCodes = []string{models.ReasonCodeImageHashReused}
	result.Score = 20
	result.Confidence = 0.75
	return result
}
