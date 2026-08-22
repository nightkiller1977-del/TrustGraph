package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/nightkiller1977-del/trustgraph/internal/store"
)

type MetricsHandler struct {
	logger      *zap.Logger
	calibration *store.CalibrationRepository
}

func NewMetricsHandler(db *store.PostgresDB, logger *zap.Logger) *MetricsHandler {
	return &MetricsHandler{
		logger:      logger,
		calibration: store.NewCalibrationRepository(db),
	}
}

// GetCalibration returns accuracy metrics computed from reviewed assessments.
func (h *MetricsHandler) GetCalibration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	metrics, err := h.calibration.GetCalibrationMetrics(ctx)
	if err != nil {
		h.logger.Error("calibration metrics failed", zap.Error(err))
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Failed to compute calibration metrics")
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}
