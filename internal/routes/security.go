package routes

import (
	"hackton-treino/internal/handler"
	"hackton-treino/internal/repository/secevents"
	"net/http"

	"go.uber.org/zap"
)

func setupSecurityRoutes(mux *http.ServeMux, logger *zap.Logger, secLog secevents.Repository) {
	h := handler.NewSecurityEventsHandler(logger, secLog)
	mux.HandleFunc("GET /security/events", h.ListEvents)
}
