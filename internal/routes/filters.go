package routes

import (
	"hackton-treino/internal/discovery"
	"hackton-treino/internal/handler"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func setupDiscoveryRoutes(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool, svc *discovery.Service) {
	h := handler.NewDiscoveryHandler(logger, db, svc)

	mux.HandleFunc("GET /discovery/jobs", h.ListJobs)
	mux.HandleFunc("GET /discovery/status", h.Status)
	mux.HandleFunc("POST /discovery/refresh", h.Refresh)
	mux.HandleFunc("GET /discovery/preferences", h.GetPreferences)
	mux.HandleFunc("PUT /discovery/preferences", h.SavePreferences)
	mux.HandleFunc("GET /discovery/technologies", h.Technologies)
	mux.HandleFunc("GET /discovery/alerts", h.GetAlerts)
	mux.HandleFunc("PUT /discovery/alerts", h.SaveAlerts)
	mux.HandleFunc("POST /discovery/alerts/seen", h.MarkAlertsSeen)
	mux.HandleFunc("POST /discovery/alerts/feedback", h.AlertFeedback)
	mux.HandleFunc("POST /discovery/jobs/{id}/dismiss", h.DismissJob)
	mux.HandleFunc("DELETE /discovery/jobs/{id}/dismiss", h.RestoreJob)
}

func setupFilterRoutes(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool) {
	h := handler.NewFilterHandler(logger, db)

	mux.HandleFunc("GET /filters", h.ListFilters)
	mux.HandleFunc("POST /filters", h.InsertFilter)
	mux.HandleFunc("DELETE /filters/{id}", h.DeleteFilter)
}
