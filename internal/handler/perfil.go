package handler

import (
	"context"
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/util"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

type PerfilHandler struct {
	Logger   *zap.Logger
	DataBase repository.Repository
}

func parsePgDate(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{Valid: false}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{Valid: false}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// ─── Informações Básicas ───────────────────────────────────────────────────

func (h *PerfilHandler) GetInformacoesBasicas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	info, appErr := h.DataBase.QuerySelectBasicInfo(ctx)
	if appErr != nil {
		h.Logger.Error("erro ao buscar informações básicas", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	if err := json.NewEncoder(w).Encode(info); err != nil {
		h.Logger.Error("erro ao serializar informações básicas", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

type upsertInfoBasicasRequest struct {
	Nome      string `json:"nome"`
	Email     string `json:"email"`
	Telefone  string `json:"telefone"`
	Linkedin  string `json:"linkedin"`
	Github    string `json:"github"`
	Portfolio string `json:"portfolio"`
	Resumo    string `json:"resumo"`
}

func (h *PerfilHandler) UpsertInformacoesBasicas(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	var req upsertInfoBasicasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "body inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	params := db.QueryUpsertInformacoesBasicasParams{
		ID:        id,
		Nome:      req.Nome,
		Email:     req.Email,
		Telefone:  util.ConvertToPgText(req.Telefone),
		Linkedin:  util.ConvertToPgText(req.Linkedin),
		Github:    util.ConvertToPgText(req.Github),
		Portfolio: util.ConvertToPgText(req.Portfolio),
		Resumo:    util.ConvertToPgText(req.Resumo),
	}

	info, appErr := h.DataBase.QueryUpsertInformacoesBasicas(ctx, params)
	if appErr != nil {
		h.Logger.Error("erro ao upsert informações básicas", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(info); err != nil {
		h.Logger.Error("erro ao serializar informações básicas", zap.Error(err))
	}
}

// ─── Experiências ──────────────────────────────────────────────────────────

func (h *PerfilHandler) ListExperiencias(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, appErr := h.DataBase.QuerySelectAllExperiencias(ctx)
	if appErr != nil {
		h.Logger.Error("erro ao listar experiências", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	if err := json.NewEncoder(w).Encode(rows); err != nil {
		h.Logger.Error("erro ao serializar experiências", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

type insertExperienciaRequest struct {
	Empresa    string   `json:"empresa"`
	Cargo      string   `json:"cargo"`
	Descricao  string   `json:"descricao"`
	Atual      bool     `json:"atual"`
	DataInicio string   `json:"data_inicio"`
	DataFim    string   `json:"data_fim"`
	Stack      []string `json:"stack"`
	Conquistas []string `json:"conquistas"`
	Tags       []string `json:"tags"`
}

func (h *PerfilHandler) InsertExperiencia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req insertExperienciaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "body inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	row, appErr := h.DataBase.QueryInsertExperiencia(ctx, db.QueryInsertExperienciaParams{
		Empresa:    req.Empresa,
		Cargo:      req.Cargo,
		Descricao:  util.ConvertToPgText(req.Descricao),
		Atual:      req.Atual,
		DataInicio: parsePgDate(req.DataInicio),
		DataFim:    parsePgDate(req.DataFim),
		Stack:      req.Stack,
		Conquistas: req.Conquistas,
		Tags:       req.Tags,
	})
	if appErr != nil {
		h.Logger.Error("erro ao inserir experiência", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(row); err != nil {
		h.Logger.Error("erro ao serializar experiência", zap.Error(err))
	}
}

func (h *PerfilHandler) DeleteExperiencia(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if appErr := h.DataBase.QueryDeleteExperiencia(ctx, id); appErr != nil {
		h.Logger.Error("erro ao deletar experiência", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Habilidades ───────────────────────────────────────────────────────────

func (h *PerfilHandler) ListHabilidades(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, appErr := h.DataBase.QuerySelectAllHabilidades(ctx)
	if appErr != nil {
		h.Logger.Error("erro ao listar habilidades", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	if err := json.NewEncoder(w).Encode(rows); err != nil {
		h.Logger.Error("erro ao serializar habilidades", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

type insertHabilidadeRequest struct {
	Nome  string   `json:"nome"`
	Nivel string   `json:"nivel"`
	Tags  []string `json:"tags"`
}

func (h *PerfilHandler) InsertHabilidade(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req insertHabilidadeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "body inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	row, appErr := h.DataBase.QueryInsertHabilidade(ctx, db.QueryInsertHabilidadeParams{
		Nome:    req.Nome,
		Nivel:   db.NivelHabilidade(req.Nivel),
		Column3: req.Tags,
	})
	if appErr != nil {
		h.Logger.Error("erro ao inserir habilidade", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(row); err != nil {
		h.Logger.Error("erro ao serializar habilidade", zap.Error(err))
	}
}

func (h *PerfilHandler) DeleteHabilidade(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if appErr := h.DataBase.QueryDeleteHabilidade(ctx, id); appErr != nil {
		h.Logger.Error("erro ao deletar habilidade", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Projetos ──────────────────────────────────────────────────────────────

func (h *PerfilHandler) ListProjetos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, appErr := h.DataBase.QuerySelectAllProjetos(ctx)
	if appErr != nil {
		h.Logger.Error("erro ao listar projetos", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	if err := json.NewEncoder(w).Encode(rows); err != nil {
		h.Logger.Error("erro ao serializar projetos", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

type insertProjetoRequest struct {
	Nome        string   `json:"nome"`
	Descricao   string   `json:"descricao"`
	Link        string   `json:"link"`
	Tags        []string `json:"tags"`
	DataInicio  string   `json:"data_inicio"`
	DataFim     string   `json:"data_fim"`
	Facultativo bool     `json:"facultativo"`
}

func (h *PerfilHandler) InsertProjeto(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req insertProjetoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "body inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	row, appErr := h.DataBase.QueryInsertProjeto(ctx, db.QueryInsertProjetoParams{
		Nome:        req.Nome,
		Descricao:   util.ConvertToPgText(req.Descricao),
		Link:        util.ConvertToPgText(req.Link),
		Column4:     req.Tags,
		DataInicio:  parsePgDate(req.DataInicio),
		DataFim:     parsePgDate(req.DataFim),
		Facultativo: req.Facultativo,
	})
	if appErr != nil {
		h.Logger.Error("erro ao inserir projeto", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(row); err != nil {
		h.Logger.Error("erro ao serializar projeto", zap.Error(err))
	}
}

func (h *PerfilHandler) DeleteProjeto(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if appErr := h.DataBase.QueryDeleteProjeto(ctx, id); appErr != nil {
		h.Logger.Error("erro ao deletar projeto", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Filtros ───────────────────────────────────────────────────────────────

func (h *PerfilHandler) ListFiltros(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, appErr := h.DataBase.QuerySelectAllFiltrosWithID(ctx)
	if appErr != nil {
		h.Logger.Error("erro ao listar filtros", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	if err := json.NewEncoder(w).Encode(rows); err != nil {
		h.Logger.Error("erro ao serializar filtros", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

type insertFiltroRequest struct {
	Keyword string `json:"keyword"`
}

func (h *PerfilHandler) InsertFiltro(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req insertFiltroRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "body inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	row, appErr := h.DataBase.QueryInsertFiltro(ctx, req.Keyword)
	if appErr != nil {
		h.Logger.Error("erro ao inserir filtro", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(row); err != nil {
		h.Logger.Error("erro ao serializar filtro", zap.Error(err))
	}
}

func (h *PerfilHandler) DeleteFiltro(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if appErr := h.DataBase.QueryDeleteFiltro(ctx, id); appErr != nil {
		h.Logger.Error("erro ao deletar filtro", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Certificações ─────────────────────────────────────────────────────────────

func (h *PerfilHandler) ListCertificacoes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, appErr := h.DataBase.QuerySelectAllCertificacoes(ctx)
	if appErr != nil {
		h.Logger.Error("erro ao listar certificações", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	if err := json.NewEncoder(w).Encode(rows); err != nil {
		h.Logger.Error("erro ao serializar certificações", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

type insertCertificacaoRequest struct {
	Nome      string   `json:"nome"`
	Emissor   string   `json:"emissor"`
	EmitidoEm string   `json:"emitido_em"`
	Codigo    string   `json:"codigo"`
	Link      string   `json:"link"`
	Tags      []string `json:"tags"`
}

func (h *PerfilHandler) InsertCertificacao(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req insertCertificacaoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "body inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	row, appErr := h.DataBase.QueryInsertCertificacao(ctx, db.QueryInsertCertificacaoParams{
		Nome:      req.Nome,
		Emissor:   req.Emissor,
		EmitidoEm: parsePgDate(req.EmitidoEm),
		Codigo:    util.ConvertToPgText(req.Codigo),
		Link:      util.ConvertToPgText(req.Link),
		Column6:   req.Tags,
	})
	if appErr != nil {
		h.Logger.Error("erro ao inserir certificação", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(row); err != nil {
		h.Logger.Error("erro ao serializar certificação", zap.Error(err))
	}
}

func (h *PerfilHandler) DeleteCertificacao(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if appErr := h.DataBase.QueryDeleteCertificacao(ctx, id); appErr != nil {
		h.Logger.Error("erro ao deletar certificação", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
