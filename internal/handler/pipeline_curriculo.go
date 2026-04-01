package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/util"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func ServeUI(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/index.html")
}

type PipelineCurriculo struct {
	Logger   *zap.Logger
	DataBase repository.Repository
}

type VagaRequest struct {
	Url          string    `json:"url"`
	Title        *string   `json:"title,omitempty"`
	Description  *string   `json:"description,omitempty"`
	Enterprise   *string   `json:"enterprise,omitempty"`
	Stack        *[]string `json:"stack,omitempty"`
	Requirements *[]string `json:"requirements,omitempty"`
	Langage      string    `json:"idioma"`
}

func (h *PipelineCurriculo) CreateVaga(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Creating job posting")
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	body, errIO := io.ReadAll(r.Body)
	if errIO != nil {
		h.Logger.Error("Error reading request body", zap.Error(errIO))
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	var req VagaRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.Logger.Error("Error decoding request body", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	id, err := h.DataBase.QueryFindVagaByUrl(ctx, req.Url)
	if err != nil {
		if err.StatusCode != http.StatusNotFound {
			h.Logger.Error("Error checking existing job posting", zap.String("err", err.Message))
			handlerAppError(w, err)
			return
		}
	}
	if id != 0 {
		h.Logger.Warn("Job posting already exists", zap.String("url", req.Url))
		http.Error(w, "Job posting already exists", http.StatusConflict)
		return
	}

	h.Logger.Info("Received job posting request", zap.String("url", req.Url))

	dbQuery := db.QueryInsertVagaParams{
		Url:        req.Url,
		Titulo:     util.ConvertToPgText(util.SafeString(req.Title)),
		Descricao:  util.ConvertToPgText(util.SafeString(req.Description)),
		Empresa:    util.ConvertToPgText(util.SafeString(req.Enterprise)),
		Stack:      util.ConvertToPgTextArray(util.SafeStringSlice(req.Stack)),
		Requisitos: util.ConvertToPgTextArray(util.SafeStringSlice(req.Requirements)),
		Idioma:     util.ConvertToPgText(req.Langage),
	}

	err = h.DataBase.QueryInsertVagas(ctx, dbQuery)
	if err != nil {
		h.Logger.Error("Error executing transaction", zap.Error(err))
		handlerAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

type listarVagasResponse struct {
	Id         int64     `json:"id"`
	Url        string    `json:"url"`
	Title      *string   `json:"titulo"`
	Enterprise *string   `json:"empresa"`
	CriadoEm   time.Time `json:"criado_em"`
	Status     string    `json:"status"`
}

func (h *PipelineCurriculo) DeleteVaga(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if appErr := h.DataBase.QueryDeleteVaga(ctx, id); appErr != nil {
		h.Logger.Error("erro ao deletar vaga", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PipelineCurriculo) ListarVagas(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Listando vagas")

	query := r.URL.Query()

	limit := 10
	offset := 0

	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil {
			offset = parsed
		}
	}

	var status db.StatusVaga
	if s := query.Get("status"); s != "" {
		status = db.StatusVaga(s)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	args := db.QueryListarVagasParams{
		Tamanho: int32(limit),
		Pagina:  int32(offset),
		Status:  string(status),
	}

	rows, err := h.DataBase.QueryListarVagas(ctx, args)
	if err != nil {
		h.Logger.Error("Error executing transaction", zap.Error(err))
		handlerAppError(w, err)
		return
	}

	result := make([]listarVagasResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, listarVagasResponse{
			Id:         row.ID,
			Url:        row.Url,
			Title:      &row.Titulo.String,
			Enterprise: &row.Empresa.String,
			CriadoEm:   row.CriadoEm.Time,
			Status:     string(row.Status),
		})
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.Logger.Error("erro ao serializar vagas", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

type curriculoResponse struct {
	ID       int64           `json:"id"`
	VagaID   int64           `json:"vaga_id"`
	Conteudo json.RawMessage `json:"conteudo"`
	CriadoEm string          `json:"criado_em"`
	Empresa  string          `json:"empresa"`
	Titulo   string          `json:"titulo"`
	Url      string          `json:"url"`
}

func (h *PipelineCurriculo) ListarCurriculos(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if p, err := strconv.Atoi(l); err == nil {
			limit = p
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if p, err := strconv.Atoi(o); err == nil {
			offset = p
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := h.DataBase.QueryListarCurriculos(ctx, db.QueryListarCurriculosParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		h.Logger.Error("erro ao listar curriculos", zap.Error(err))
		handlerAppError(w, err)
		return
	}

	result := make([]curriculoResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, curriculoResponse{
			ID:       row.ID,
			VagaID:   row.VagaID,
			Conteudo: json.RawMessage(row.ConteudoJson),
			CriadoEm: row.CriadoEm.Time.Format(time.RFC3339),
			Empresa:  row.Empresa.String,
			Titulo:   row.Titulo.String,
			Url:      row.Url,
		})
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.Logger.Error("erro ao serializar curriculos", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
	}
}

type feedbackRequest struct {
	CurriculoID int64  `json:"curriculo_id"`
	VagaID      int64  `json:"vaga_id"`
	Status      string `json:"status"`
	Comentario  string `json:"comentario"`
}

func (h *PipelineCurriculo) DeleteCurriculoGerado(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if appErr := h.DataBase.QueryDeleteCurriculoGerado(ctx, id); appErr != nil {
		h.Logger.Error("erro ao deletar currículo gerado", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type updateVagaStatusRequest struct {
	Status string `json:"status"`
}

func (h *PipelineCurriculo) UpdateVagaStatus(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, convErr := strconv.ParseInt(idStr, 10, 64)
	if convErr != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	var req updateVagaStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "body inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if appErr := h.DataBase.QueryUpdateVagaStatusOnly(ctx, db.QueryUpdateVagaStatusOnlyParams{
		ID:     id,
		Status: db.StatusVaga(req.Status),
	}); appErr != nil {
		h.Logger.Error("erro ao atualizar status da vaga", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func normalizarNome(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

type gerarPDFResponse struct {
	ResumePath      string `json:"resume_path"`
	CoverLetterPath string `json:"cover_letter_path"`
}

type pythonInput struct {
	Curriculo   string `json:"curriculo"`
	CoverLetter string `json:"cover_letter"`
	OutputDir   string `json:"output_dir"`
}

type pythonOutput struct {
	ResumePath      string `json:"resume_path"`
	CoverLetterPath string `json:"cover_letter_path"`
	Error           string `json:"error,omitempty"`
}

func (h *PipelineCurriculo) GerarPDF(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	idStr := r.PathValue("id")
	id, parseErr := strconv.ParseInt(idStr, 10, 64)
	if parseErr != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	row, appErr := h.DataBase.QueryGetCurriculoComVaga(ctx, id)
	if appErr != nil {
		h.Logger.Error("erro ao buscar currículo", zap.Error(appErr))
		handlerAppError(w, appErr)
		return
	}

	var conteudo map[string]string
	if err := json.Unmarshal(row.ConteudoJson, &conteudo); err != nil {
		h.Logger.Error("erro ao parsear conteudo_json", zap.Error(err))
		http.Error(w, "conteudo_json inválido", http.StatusInternalServerError)
		return
	}

	curriculo, ok1 := conteudo["curriculo"]
	coverLetter, ok2 := conteudo["cover_letter"]
	if !ok1 || !ok2 {
		http.Error(w, "campos curriculo/cover_letter ausentes no conteudo_json", http.StatusUnprocessableEntity)
		return
	}

	empresa := normalizarNome(row.Empresa.String)
	titulo := normalizarNome(row.Titulo.String)
	if empresa == "" {
		empresa = "empresa"
	}
	if titulo == "" {
		titulo = "vaga"
	}
	data := time.Now().Format("2006-01-02")
	outputDir := filepath.Join("output", fmt.Sprintf("%s_%s_%s", empresa, titulo, data))

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		h.Logger.Error("erro ao criar diretório de output", zap.Error(err))
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}

	input := pythonInput{
		Curriculo:   curriculo,
		CoverLetter: coverLetter,
		OutputDir:   outputDir,
	}
	inputJSON, _ := json.Marshal(input)

	cmd := exec.CommandContext(ctx, "python3", "scripts/gerar_pdf.py")
	cmd.Stdin = bytes.NewReader(inputJSON)
	out, err := cmd.Output()
	if err != nil {
		h.Logger.Error("erro ao executar script Python", zap.Error(err))
		if exitErr, ok := err.(*exec.ExitError); ok {
			h.Logger.Error("stderr do Python", zap.String("stderr", string(exitErr.Stderr)))
		}
		http.Error(w, "erro ao gerar PDF", http.StatusInternalServerError)
		return
	}

	var pyOut pythonOutput
	if err := json.Unmarshal(out, &pyOut); err != nil {
		h.Logger.Error("erro ao parsear saída do Python", zap.Error(err))
		http.Error(w, "erro ao processar resultado do PDF", http.StatusInternalServerError)
		return
	}
	if pyOut.Error != "" {
		h.Logger.Error("erro reportado pelo script Python", zap.String("error", pyOut.Error))
		http.Error(w, pyOut.Error, http.StatusInternalServerError)
		return
	}

	updateErr := h.DataBase.QueryUpdateCurriculoPaths(ctx, db.QueryUpdateCurriculoPathsParams{
		ID:              id,
		ResumePath:      pgtype.Text{String: pyOut.ResumePath, Valid: true},
		CoverLetterPath: pgtype.Text{String: pyOut.CoverLetterPath, Valid: true},
	})
	if updateErr != nil {
		h.Logger.Error("erro ao salvar caminhos dos PDFs", zap.Error(updateErr))
		handlerAppError(w, updateErr)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(gerarPDFResponse{
		ResumePath:      pyOut.ResumePath,
		CoverLetterPath: pyOut.CoverLetterPath,
	})
}

func (h *PipelineCurriculo) InserirFeedback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req feedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "body inválido", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := h.DataBase.QueryInsertFeedback(ctx, db.QueryInsertFeedbackParams{
		CurriculoID: req.CurriculoID,
		VagaID:      req.VagaID,
		Status:      db.StatusFeedback(req.Status),
		Comentario:  util.ConvertToPgText(req.Comentario),
	})
	if err != nil {
		h.Logger.Error("erro ao inserir feedback", zap.Error(err))
		handlerAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
