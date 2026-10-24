package handler

import (
	"hackton-treino/internal/repository/aimetrics"
	"hackton-treino/internal/repository/usermetrics"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type MetricsHandler struct {
	Logger  *zap.Logger
	AICalls *aimetrics.Repository
	Users   *usermetrics.Repository
}

func NewMetricsHandler(logger *zap.Logger, ai *aimetrics.Repository, users *usermetrics.Repository) *MetricsHandler {
	return &MetricsHandler{Logger: logger, AICalls: ai, Users: users}
}

// UserStats: total, novos, ativos (24h/7d/30d) e quantos já geraram currículo.
func (h *MetricsHandler) UserStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.Users.Stats(r.Context())
	if err != nil {
		h.Logger.Error("metrics: erro ao agregar usuários", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// Janelas aceitas e a granularidade da linha do tempo de cada uma.
var aiMetricsRanges = map[string]struct{ window, bucket time.Duration }{
	"24h": {24 * time.Hour, time.Hour},
	"7d":  {7 * 24 * time.Hour, 6 * time.Hour},
	"30d": {30 * 24 * time.Hour, 24 * time.Hour},
}

// AI agrega as chamadas aos providers de IA: GET /admin/metrics/ai?range=24h|7d|30d.
func (h *MetricsHandler) AI(w http.ResponseWriter, r *http.Request) {
	rg, ok := aiMetricsRanges[r.URL.Query().Get("range")]
	if !ok {
		rg = aiMetricsRanges["24h"]
	}
	report, err := h.AICalls.Report(r.Context(), time.Now().Add(-rg.window), rg.bucket)
	if err != nil {
		h.Logger.Error("metrics: erro ao agregar chamadas de IA", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}
