package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/profileimport"
	"hackton-treino/internal/repository"
	ucache "hackton-treino/internal/repository/cache"

	"go.uber.org/zap"
)

// ProfileImportHandler: enviar o currículo → consultar o rascunho → aplicar o que foi conferido.
type ProfileImportHandler struct {
	*BaseHandler
	Service *profileimport.Service
	Tx      repository.Base
	Cache   ucache.Cache
}

func NewProfileImportHandler(logger *zap.Logger, svc *profileimport.Service, tx repository.Base, cache ucache.Cache) *ProfileImportHandler {
	return &ProfileImportHandler{BaseHandler: NewBaseHandler(logger), Service: svc, Tx: tx, Cache: cache}
}

// Upload recebe o arquivo cru no body (PDF, DOCX ou TXT). O arquivo não é guardado.
func (h *ProfileImportHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, profileimport.MaxFileSize+1))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) || len(data) > profileimport.MaxFileSize {
		writeError(w, http.StatusRequestEntityTooLarge, "arquivo maior que 5 MB")
		return
	}
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusBadRequest, "envie o arquivo do currículo")
		return
	}

	id, err := h.Service.Start(r.Context(), userID.String(), data)
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, map[string]string{"id": id, "status": profileimport.StatusProcessing})
	case errors.Is(err, profileimport.ErrUnsupported), errors.Is(err, profileimport.ErrNoText):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, profileimport.ErrBusy):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, profileimport.ErrQuota):
		writeError(w, http.StatusTooManyRequests, err.Error())
	default:
		h.Logger.Warn("importação de currículo: falha ao iniciar", zap.Error(err))
		writeError(w, http.StatusUnprocessableEntity, "não consegui ler esse arquivo; confira se não está protegido ou corrompido")
	}
}

func (h *ProfileImportHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}
	st, err := h.Service.Store.Get(r.Context(), userID.String(), r.PathValue("id"))
	if errors.Is(err, profileimport.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		h.Logger.Error("importação de currículo: falha ao ler o estado", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao consultar a importação")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// Cancel interrompe a leitura em andamento (ou descarta o rascunho pronto).
func (h *ProfileImportHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}
	h.Service.Cancel(r.Context(), userID.String(), r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileImportHandler) Apply(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}
	var req profileimport.ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}

	aliasMap := h.Service.LoadAliases(r.Context())
	var sum profileimport.Summary
	var vErr *profileimport.ValidationError
	repoErr := h.Tx.ExecTx(r.Context(), func(q *db.Queries) error {
		var err error
		sum, err = profileimport.Apply(r.Context(), q, userID, req, aliasMap)
		errors.As(err, &vErr)
		return err
	})
	if vErr != nil {
		writeError(w, http.StatusBadRequest, vErr.Msg)
		return
	}
	if repoErr != nil {
		writeRepositoryError(w, repoErr)
		return
	}

	uid := userID.String()
	for _, topic := range []string{ucache.TopicProfile, ucache.TopicExperiences, ucache.TopicAcademic,
		ucache.TopicSkills, ucache.TopicProjects, ucache.TopicCertificates} {
		if err := h.Cache.Delete(r.Context(), uid, topic); err != nil {
			h.Logger.Warn("cache: falha ao invalidar", zap.String("topic", topic), zap.Error(err))
		}
	}
	if req.ImportID != "" {
		h.Service.Store.Delete(r.Context(), uid, req.ImportID)
	}
	writeJSON(w, http.StatusOK, sum)
}
