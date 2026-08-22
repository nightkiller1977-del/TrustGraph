package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/config"
	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

type MetricsHandler struct {
	logger      *zap.Logger
	cfg         *config.Config
	calibration *store.CalibrationRepository
}

func NewMetricsHandler(db *store.PostgresDB, logger *zap.Logger, cfg *config.Config) *MetricsHandler {
	return &MetricsHandler{
		logger:      logger,
		cfg:         cfg,
		calibration: store.NewCalibrationRepository(db),
	}
}

// GetCalibration returns accuracy metrics computed from reviewed assessments.
func (h *MetricsHandler) GetCalibration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	metrics, err := h.calibration.GetCalibrationMetrics(ctx, h.cfg.EnforcementThreshold)
	if err != nil {
		h.logger.Error("calibration metrics failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to compute calibration metrics")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}
