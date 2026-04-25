package handler

import (
	"hackton-treino/internal/repository/secevents"
	"net/http"
	"strconv"

	"go.uber.org/zap"
)

type SecurityEventsHandler struct {
	*BaseHandler
	SecLog secevents.Repository
}

func NewSecurityEventsHandler(logger *zap.Logger, secLog secevents.Repository) *SecurityEventsHandler {
	return &SecurityEventsHandler{
		BaseHandler: NewBaseHandler(logger),
		SecLog:      secLog,
	}
}

func (h *SecurityEventsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 90 {
			days = n
		}
	}

	rows, errR := h.SecLog.List(r.Context(), days)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	if rows == nil {
		rows = nil
	}
	writeJSON(w, http.StatusOK, rows)
}
