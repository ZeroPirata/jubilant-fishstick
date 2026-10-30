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
}

func setupFilterRoutes(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool) {
	h := handler.NewFilterHandler(logger, db)

	mux.HandleFunc("GET /filters", h.ListFilters)
	mux.HandleFunc("POST /filters", h.InsertFilter)
	mux.HandleFunc("DELETE /filters/{id}", h.DeleteFilter)
}
