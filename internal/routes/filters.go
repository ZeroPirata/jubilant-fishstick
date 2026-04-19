package routes

import (
	"hackton-treino/internal/handler"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func setupFilterRoutes(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool) {
	h := handler.NewFilterHandler(logger, db)

	mux.HandleFunc("GET /filters", h.ListFilters)
	mux.HandleFunc("POST /filters", h.InsertFilter)
	mux.HandleFunc("DELETE /filters/{id}", h.DeleteFilter)
}
