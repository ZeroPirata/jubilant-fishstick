package handler

import (
	"net/http"
	"strconv"

	"hackton-treino/internal/repository/importlog"

	"go.uber.org/zap"
)

// ImportLogHandler: histórico das importações de currículo no painel admin.
type ImportLogHandler struct {
	Logger *zap.Logger
	Repo   *importlog.Repository
}

func NewImportLogHandler(logger *zap.Logger, repo *importlog.Repository) *ImportLogHandler {
	return &ImportLogHandler{Logger: logger, Repo: repo}
}

func (h *ImportLogHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	entries, err := h.Repo.List(r.Context(), limit)
	if err != nil {
		h.Logger.Error("importações: erro ao listar", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
