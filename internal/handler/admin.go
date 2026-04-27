package handler

import (
	"encoding/json"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository/admin"
	"net/http"
	"strconv"

	"go.uber.org/zap"
)

type AdminHandler struct {
	Logger *zap.Logger
	Admin  admin.Repository
}

func NewAdminHandler(logger *zap.Logger, repo admin.Repository) *AdminHandler {
	return &AdminHandler{Logger: logger, Admin: repo}
}

func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	isAdmin, err := h.Admin.IsAdmin(r.Context(), userID.String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"is_admin": isAdmin})
}

func (h *AdminHandler) ListErrorLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	logs, total, err := h.Admin.ListErrorLogs(r.Context(), limit, offset)
	if err != nil {
		h.Logger.Error("admin: erro ao listar error_logs", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": logs, "total": total})
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Admin.ListUsers(r.Context())
	if err != nil {
		h.Logger.Error("admin: erro ao listar usuários", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": users})
}

func (h *AdminHandler) SetAdmin(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "id obrigatório")
		return
	}

	var body struct {
		IsAdmin bool `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}

	if err := h.Admin.SetAdmin(r.Context(), userID, body.IsAdmin); err != nil {
		h.Logger.Error("admin: erro ao atualizar is_admin", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
