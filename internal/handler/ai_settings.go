package handler

import (
	"encoding/json"
	"errors"
	"hackton-treino/config"
	"hackton-treino/internal/aiconfig"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository/admin"
	ucache "hackton-treino/internal/repository/cache"
	"hackton-treino/internal/services"
	"net/http"

	"go.uber.org/zap"
)

// AISettingsHandler expõe as configurações de IA para usuários master.
type AISettingsHandler struct {
	Logger *zap.Logger
	Store  *aiconfig.Store
	Cache  ucache.Cache
	Admin  admin.Repository
	Config config.Config
}

func NewAISettingsHandler(logger *zap.Logger, store *aiconfig.Store, cache ucache.Cache, adminRepo admin.Repository, cfg config.Config) *AISettingsHandler {
	return &AISettingsHandler{Logger: logger, Store: store, Cache: cache, Admin: adminRepo, Config: cfg}
}

func (h *AISettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	mainChain, err := h.Store.List(r.Context(), services.PurposeMain)
	if err != nil {
		h.Logger.Error("ai-settings: erro ao listar cadeia main", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	scrapeChain, err := h.Store.List(r.Context(), services.PurposeScrape)
	if err != nil {
		h.Logger.Error("ai-settings: erro ao listar cadeia scrape", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}

	// O que é usado quando a cadeia do painel está vazia (sem keys).
	envMain := services.Provider{Name: h.Config.Ai.Provider, Model: h.Config.Ai.Model}.Label()
	envScrape := "mesma cadeia da IA principal"
	if h.Config.ScrapeAi.Activate {
		envScrape = services.Provider{Name: h.Config.ScrapeAi.Provider, Model: h.Config.ScrapeAi.Model}.Label()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"main":               mainChain,
		"scrape":             scrapeChain,
		"env_fallback":       map[string]string{"main": envMain, "scrape": envScrape},
		"encryption_enabled": h.Store.EncryptionEnabled(),
		"worker_paused_secs": int(h.Cache.RateLimitRemaining(r.Context()).Seconds()),
	})
}

func (h *AISettingsHandler) Save(w http.ResponseWriter, r *http.Request) {
	purpose, ok := aiconfig.ParsePurpose(r.PathValue("purpose"))
	if !ok {
		writeError(w, http.StatusBadRequest, "purpose deve ser main ou scrape")
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}

	var body struct {
		Providers []aiconfig.Input `json:"providers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}

	if err := h.Store.Save(r.Context(), purpose, body.Providers, userID.String()); err != nil {
		var verr *aiconfig.ValidationError
		if errors.As(err, &verr) {
			writeError(w, http.StatusBadRequest, verr.Msg)
			return
		}
		h.Logger.Error("ai-settings: erro ao salvar", zap.String("purpose", string(purpose)), zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}

	// Config nova pode resolver o rate limit: retoma o worker em vez de esperar a pausa acabar.
	if err := h.Cache.ClearRateLimit(r.Context()); err != nil {
		h.Logger.Warn("ai-settings: não foi possível limpar a pausa do worker", zap.Error(err))
	}

	h.Logger.Info("ai-settings: cadeia atualizada",
		zap.String("purpose", string(purpose)),
		zap.Int("providers", len(body.Providers)),
		zap.String("user_id", userID.String()),
	)

	chain, err := h.Store.List(r.Context(), purpose)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, chain)
}

// ResumeWorker tira o worker da pausa de rate limit sem mudar a configuração
// (ex.: a cota free do provider já voltou).
func (h *AISettingsHandler) ResumeWorker(w http.ResponseWriter, r *http.Request) {
	if err := h.Cache.ClearRateLimit(r.Context()); err != nil {
		h.Logger.Error("ai-settings: erro ao retomar worker", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *AISettingsHandler) SetMaster(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}

	var body struct {
		IsMaster bool `json:"is_master"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}

	// Evita ficar sem nenhum master por engano.
	if !body.IsMaster && targetID == userID.String() {
		writeError(w, http.StatusConflict, "você não pode remover o seu próprio master")
		return
	}

	if err := h.Admin.SetMaster(r.Context(), targetID, body.IsMaster); err != nil {
		h.Logger.Error("admin: erro ao atualizar is_master", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	h.Logger.Info("admin: is_master alterado",
		zap.String("target_id", targetID), zap.Bool("is_master", body.IsMaster), zap.String("by", userID.String()))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
