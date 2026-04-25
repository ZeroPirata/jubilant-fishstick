package handler

import (
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/security"
	"net/http"

	"go.uber.org/zap"
)

type LogoutHandler struct {
	*BaseHandler
	Revoker security.TokenRevoker
}

func NewLogoutHandler(logger *zap.Logger, revoker security.TokenRevoker) *LogoutHandler {
	return &LogoutHandler{BaseHandler: NewBaseHandler(logger), Revoker: revoker}
}

func (h *LogoutHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetAuthClaims(r.Context())
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	h.Revoker.Revoke(r.Context(), claims.JTI, claims.Exp)
	w.WriteHeader(http.StatusNoContent)
}
