package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type SensorHandler struct {
	Logger   *zap.Logger
	DataBase *pgxpool.Pool
}

func (h *SensorHandler) CreateTelemetry(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Creating telemetry data")
	// Lógica para criar dados de telemetria
	w.WriteHeader(http.StatusAccepted)
}

func (h *SensorHandler) Health(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Health check")

	_, err := w.Write([]byte("OK"))
	if err != nil {
		h.Logger.Error("failed to write response", zap.Error(err))
	}
}
