package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/jobfit"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/aliases"
	discoveryRepo "hackton-treino/internal/repository/discovery"
	"hackton-treino/internal/repository/feedbacks"
	"hackton-treino/internal/repository/jobs"
	"hackton-treino/internal/repository/secevents"
	"hackton-treino/internal/repository/users"
	"hackton-treino/internal/resume"
	"hackton-treino/internal/sse"
	"hackton-treino/internal/util"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type JobHandler struct {
	*BaseHandler
	Jobs      jobs.Repository
	Feedbacks feedbacks.Repository
	Users     users.Repository
	Aliases   aliases.Repository
	Catalog   discoveryRepo.Repository
	PDF       *PDFService
	Bus       *sse.Bus
	SecLog    secevents.Repository
}

func NewJobHandler(logger *zap.Logger, conn *pgxpool.Pool, pdf *PDFService, bus *sse.Bus, secLog secevents.Repository) *JobHandler {
	return &JobHandler{
		BaseHandler: NewBaseHandler(logger),
		Jobs:        jobs.New(conn),
		Feedbacks:   feedbacks.New(conn),
		Users:       users.New(conn),
		Aliases:     aliases.New(conn),
		Catalog:     discoveryRepo.New(conn),
		PDF:         pdf,
		Bus:         bus,
		SecLog:      secLog,
	}
}

func (h *JobHandler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.Bus.Subscribe(userID.String())
	defer h.Bus.Unsubscribe(userID.String(), ch)

	// Envia os headers na hora: sem isso o navegador (e proxies como o cloudflared)
	// só considera a conexão aberta no primeiro evento ou keepalive, até 20s depois.
	fmt.Fprintf(w, ": conectado\n\n")
	flusher.Flush()

	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case event := <-ch:
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// Job
func (h *JobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {

	userId, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusMethodNotAllowed, ErrUserNotAllowed.Error())
		return
	}

	var req Job
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}

	if err := Validate(Required("url", req.Url)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := util.ValidatePublicURL(req.Url); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Language != nil && (*req.Language == "" || !validJobLanguage(*req.Language)) {
		writeError(w, http.StatusBadRequest, "idioma inválido: use pt-br ou en")
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = "full"
	}

	hasExtraData := req.CompanyName != nil ||
		req.JobTitle != nil ||
		req.Description != nil ||
		req.Language != nil ||
		len(req.Stacks) > 0 ||
		len(req.Requirements) > 0

	var jobId string
	var errR *repository.RepositoryError
	if hasExtraData {
		jobFull := db.QueryInsertFullJobParams{
			ExternalUrl:  req.Url,
			UserID:       userId,
			CompanyName:  util.ConvertToPgTextPtr(req.CompanyName),
			JobTitle:     util.ConvertToPgTextPtr(req.JobTitle),
			Description:  util.ConvertToPgTextPtr(req.Description),
			TechStack:    util.ConvertToPgTextArray(req.Stacks),
			Requirements: util.ConvertToPgTextArray(req.Requirements),
			Language:     util.ConvertToPgTextPtr(req.Language),
			Mode:         mode,
		}
		jobId, errR = h.Jobs.QueryInsertFullJob(r.Context(), jobFull)
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}

	} else {
		jobWithUrl := db.QueryInsertUrlJobParams{
			ExternalUrl: req.Url,
			UserID:      userId,
			Mode:        mode,
		}
		jobId, errR = h.Jobs.QueryInsertUrlJob(r.Context(), jobWithUrl)
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}
	}

	warnings := checkProfileWeakness(r.Context(), h.Users, userId.String())
	writeJSON(w, http.StatusCreated, map[string]any{"id": jobId, "warnings": warnings})
}

func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}

	cursor, size := getPaginationParams(r)

	rows, errR := h.Jobs.QuerySelectJobsForUser(r.Context(), db.QuerySelectJobsForUserParams{
		UserID:  userID,
		Cursor:  cursor,
		Size:    size,
		Status:  r.URL.Query().Get("status"),
		Quality: r.URL.Query().Get("quality"),
	})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	if len(rows) == 0 {
		writeList(w, http.StatusOK, []Job{}, size, cursor, 0)
		return
	}

	// Compatibilidade recalculada com o perfil de agora: quem cadastra uma skill vê a
	// nota mudar sem reprocessar a vaga. Sem skills ou aliases, a vaga sai sem fit.
	skills, errS := h.Users.QuerySelectAllSkills(r.Context(), userID.String())
	aliasMap, errA := h.Aliases.QuerySelectAllStackAliases(r.Context())
	canFit := errS == nil && errA == nil

	response := make([]Job, len(rows))
	for i, j := range rows {
		response[i] = Job{
			Base: Base{
				ID:        j.ID.String(),
				CreatedAt: j.CreatedAt.Time,
				UpdatedAt: util.PgTimeToPtr(j.UpdatedAt.Time),
				DeletedAt: util.PgTimeToPtr(j.DeletedAt.Time),
			},
			Url:          j.ExternalUrl,
			CompanyName:  util.PgTextoToNullString(j.CompanyName.String),
			JobTitle:     util.PgTextoToNullString(j.JobTitle.String),
			Description:  util.PgTextoToNullString(j.Description.String),
			Stacks:       j.TechStack,
			Requirements: j.Requirements,
			Language:     util.PgTextoToNullString(j.Language.String),
			Status:       string(j.Status),
			Quality:      util.PgTextoToNullString(string(j.Quality.JobQuality)),
			Analysis:     decodeAnalysis(j.FitAnalysis),

			ApplicationStage:   j.ApplicationStage.String,
			ApplicationStageAt: pgTimePtr(j.ApplicationStageAt),
		}
		if canFit {
			h.attachFit(r.Context(), userID, &response[i], j.ID, j.TechStack, j.Status, j.Quality, skills, aliasMap)
		}
	}

	writeList(w, http.StatusOK, response, size, cursor, int32(rows[0].TotalCount))
}

// attachFit calcula a compatibilidade com o perfil atual e, se a qualidade gravada
// mudou, grava a nova (o filtro de qualidade continua batendo com a tela).
func (h *JobHandler) attachFit(ctx context.Context, userID pgtype.UUID, out *Job, id pgtype.UUID, stack []string,
	status db.JobStatus, stored db.NullJobQuality, skills []db.UserSkill, aliasMap map[string]string) {
	if len(stack) == 0 {
		return
	}
	fit := jobfit.Evaluate(stack, skills, aliasMap)
	out.Fit = &fit
	if status != db.JobStatusCompleted || !stored.Valid || stored.JobQuality == fit.Quality {
		return
	}
	q := string(fit.Quality)
	out.Quality = &q
	if errU := h.Jobs.QueryUpdateJobQuality(ctx, db.QueryUpdateJobQualityParams{
		Quality: db.NullJobQuality{JobQuality: fit.Quality, Valid: true},
		ID:      id,
		UserID:  userID,
	}); errU != nil {
		h.Logger.Warn("Erro ao atualizar qualidade recalculada", zap.String("job_id", id.String()), zap.String("erro", errU.Message))
	}
}

// GetJob é a tela da vaga: tudo de uma vaga (descrições, stack, compatibilidade,
// análise). Os currículos vêm de /jobs/{id}/resumes.
func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	jobID, err := util.ParseUUID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	j, errR := h.Jobs.QuerySelectJobForUser(r.Context(), db.QuerySelectJobForUserParams{ID: jobID, UserID: userID})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	out := Job{
		Base: Base{
			ID:        j.ID.String(),
			CreatedAt: j.CreatedAt.Time,
			UpdatedAt: util.PgTimeToPtr(j.UpdatedAt.Time),
		},
		Url:               j.ExternalUrl,
		CompanyName:       util.PgTextoToNullString(j.CompanyName.String),
		JobTitle:          util.PgTextoToNullString(j.JobTitle.String),
		Description:       util.PgTextoToNullString(j.Description.String),
		Stacks:            j.TechStack,
		Requirements:      j.Requirements,
		Language:          util.PgTextoToNullString(j.Language.String),
		Status:            string(j.Status),
		Quality:           util.PgTextoToNullString(string(j.Quality.JobQuality)),
		Mode:              j.Mode,
		Analysis:          decodeAnalysis(j.FitAnalysis),
		SourceDescription: util.PgTextoToNullString(j.SourceDescription.String),

		ApplicationStage:   j.ApplicationStage.String,
		ApplicationStageAt: pgTimePtr(j.ApplicationStageAt),
	}
	// Texto completo: o lido no processamento; para vagas processadas antes de ele ser
	// guardado, o do catálogo do Descobrir, quando a vaga veio de lá.
	switch {
	case strings.TrimSpace(j.ScrapedDescription.String) != "":
		out.FullDescription, out.FullDescriptionFrom = &j.ScrapedDescription.String, "processamento"
	default:
		if row, found, errC := h.Catalog.SelectByURL(r.Context(), j.ExternalUrl); errC == nil && found && row.Description != "" {
			out.FullDescription, out.FullDescriptionFrom = &row.Description, "catalogo"
		}
	}

	skills, errS := h.Users.QuerySelectAllSkills(r.Context(), userID.String())
	aliasMap, errA := h.Aliases.QuerySelectAllStackAliases(r.Context())
	if errS == nil && errA == nil {
		h.attachFit(r.Context(), userID, &out, j.ID, j.TechStack, j.Status, j.Quality, skills, aliasMap)
		desc := j.SourceDescription.String
		if out.FullDescription != nil {
			desc = *out.FullDescription
		}
		kw := jobfit.Keywords(j.TechStack, desc, h.latestResumeText(r.Context(), userID, j.ID), skills, aliasMap)
		out.Keywords = &kw
	}
	writeJSON(w, http.StatusOK, out)
}

// latestResumeText devolve o texto do currículo gerado mais recente da vaga ("" sem
// currículo ou com erro: o relatório de palavras-chave sai sem a coluna do currículo).
func (h *JobHandler) latestResumeText(ctx context.Context, userID, jobID pgtype.UUID) string {
	rows, err := h.Jobs.QueryListResumesForUser(ctx, db.QueryListResumesForUserParams{UserID: userID, JobID: jobID, Size: 1})
	if err != nil || len(rows) == 0 {
		return ""
	}
	return resumeText(rows[0].ContentJson)
}

// resumeText tira o texto do currículo do content_json: "curriculo" (os dois formatos
// gravam) ou, se vier vazio, o texto montado a partir do doc estruturado.
func resumeText(contentJSON []byte) string {
	var c struct {
		Doc       *resume.Doc `json:"doc"`
		Curriculo string      `json:"curriculo"`
	}
	if json.Unmarshal(contentJSON, &c) != nil {
		return ""
	}
	if strings.TrimSpace(c.Curriculo) == "" && c.Doc != nil {
		return resume.Text(*c.Doc)
	}
	return c.Curriculo
}

func decodeAnalysis(raw []byte) *resume.FitAnalysis {
	if len(raw) == 0 {
		return nil
	}
	var a resume.FitAnalysis
	if json.Unmarshal(raw, &a) != nil {
		return nil
	}
	return a.Clean()
}

func (h *JobHandler) DeleteJob(w http.ResponseWriter, r *http.Request) {
	GenericDelete(func(userID pgtype.UUID, r *http.Request) (db.QueryDeleteJobParams, *repository.RepositoryError) {
		id, err := util.ParseUUID(r.PathValue("id"))
		return db.QueryDeleteJobParams{JobID: id, UserID: userID}, err
	},
		h.Jobs.QueryDeleteJob,
	)(w, r)
}

func (h *JobHandler) UpdateFullJob(w http.ResponseWriter, r *http.Request) {
	GenericUpdate(
		func(userId pgtype.UUID, r *http.Request, body Job) (db.QueryUpdateJobParams, *repository.RepositoryError) {
			jobId, err := util.ParseUUID(r.PathValue("id"))
			return db.QueryUpdateJobParams{
				UserID:       userId,
				CompanyName:  util.ConvertToPgTextPtr(body.CompanyName),
				JobTitle:     util.ConvertToPgTextPtr(body.JobTitle),
				Description:  util.ConvertToPgTextPtr(body.Description),
				TechStack:    util.ConvertToPgTextArray(body.Stacks),
				Requirements: util.ConvertToPgTextArray(body.Requirements),
				Language:     util.ConvertToPgTextPtr(body.Language),
				ID:           jobId,
			}, err
		},
		h.Jobs.QueryUpdateJob,
		nil,
	)(w, r)
}

// Resume
func (h *JobHandler) ListResumes(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}

	jobID, errR := util.ParseUUID(r.PathValue("id"))
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	cursor, size := getPaginationParams(r)

	rows, errR := h.Jobs.QueryListResumesForUser(r.Context(), db.QueryListResumesForUserParams{
		UserID: userID,
		JobID:  jobID,
		Cursor: cursor,
		Size:   size,
	})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	if len(rows) == 0 {
		writeList(w, http.StatusOK, []Resume{}, size, cursor, 0)
		return
	}

	response := make([]Resume, len(rows))
	for i, row := range rows {
		response[i] = Resume{
			Base: Base{
				ID:        row.ID.String(),
				CreatedAt: row.CreatedAt.Time,
			},
			Job: &Job{
				Base: Base{
					ID: row.JobID.String(),
				},
				CompanyName: util.PgTextoToNullString(row.CompanyName.String),
				JobTitle:    util.PgTextoToNullString(row.JobTitle.String),
				Quality:     util.PgTextoToNullString(string(row.Quality.JobQuality)),
			},
			ContentJson:     string(row.ContentJson),
			ResumePdfPath:   util.PgTextoToNullString(row.ResumePdfPath.String),
			CoverLetterPath: util.PgTextoToNullString(row.CoverLetterPath.String),
		}
	}

	writeList(w, http.StatusOK, response, size, cursor, int32(rows[0].TotalCount))
}

func (h *JobHandler) GetResume(w http.ResponseWriter, r *http.Request) {
	GenericOne(
		func(userID pgtype.UUID, r *http.Request) (db.QuerySelectResumeJobParams, *repository.RepositoryError) {
			resumeId := r.PathValue("resume_id")
			pgResumeId, err := util.ParseUUID(resumeId)
			return db.QuerySelectResumeJobParams{ID: pgResumeId, UserID: userID}, err
		},
		h.Jobs.QuerySelectResumeJob,
		func(row db.QuerySelectResumeJobRow) Resume {
			return Resume{
				Base: Base{
					ID: row.ID.String(),
				},
				Job: &Job{
					CompanyName: util.PgTextoToNullString(row.CompanyName.String),
					JobTitle:    util.PgTextoToNullString(row.JobTitle.String),
					Language:    util.PgTextoToNullString(row.Language.String),
					Quality:     util.PgTextoToNullString(string(row.Quality.JobQuality)),
				},
				ContentJson:     string(row.ContentJson),
				ResumePdfPath:   util.PgTextoToNullString(row.ResumePdfPath.String),
				CoverLetterPath: util.PgTextoToNullString(row.CoverLetterPath.String),
			}
		},
	)(w, r)
}

func (h *JobHandler) DeleteResume(w http.ResponseWriter, r *http.Request) {
	GenericDelete(func(userID pgtype.UUID, r *http.Request) (db.QueryDeleteResumeParams, *repository.RepositoryError) {
		jobID, err := util.ParseUUID(r.PathValue("id"))
		if err != nil {
			return db.QueryDeleteResumeParams{}, err
		}
		resumeID, err := util.ParseUUID(r.PathValue("resume_id"))
		return db.QueryDeleteResumeParams{ID: resumeID, JobID: jobID, UserID: userID}, err
	},
		h.Jobs.QueryDeleteResume,
	)(w, r)
}

func (h *JobHandler) GeneratePDF(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}

	resumeIDStr := r.PathValue("resume_id")

	var resumeID pgtype.UUID
	if err := resumeID.Scan(resumeIDStr); err != nil {
		writeError(w, http.StatusBadRequest, "resume_id inválido")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	row, appErr := h.Jobs.QuerySelectResumeJob(ctx, db.QuerySelectResumeJobParams{
		ID:     resumeID,
		UserID: userID,
	})
	if appErr != nil {
		h.Logger.Error("erro ao buscar currículo", zap.Error(appErr))
		handlerRepositoryError(w, appErr)
		return
	}

	// Corpo opcional: {"template": "executivo"}. Sem corpo ou modelo desconhecido = clássico.
	var body struct {
		Template string `json:"template"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	template := body.Template
	if !pdfTemplates[template] {
		template = defaultPDFTemplate
	}

	var conteudo storedContent
	if err := json.Unmarshal(row.ContentJson, &conteudo); err != nil {
		http.Error(w, "content_json inválido", http.StatusInternalServerError)
		return
	}

	fsDir, urlPath, err := buildOutputDir(userID.String(), resumeIDStr, row.CompanyName.String, row.JobTitle.String)
	if err != nil {
		h.Logger.Error("erro ao criar diretório", zap.Error(err))
		http.Error(w, "erro ao preparar diretório", http.StatusInternalServerError)
		return
	}

	pyOut, err := h.PDF.Generate(ctx, pythonInput{
		Curriculo:   conteudo.Curriculo,
		CoverLetter: conteudo.CoverLetter,
		OutputDir:   fsDir,
		Doc:         conteudo.Doc,
		Template:    template,
	})
	if err != nil {
		h.Logger.Error("erro ao gerar PDF", zap.Error(err))
		http.Error(w, "erro ao gerar PDF", http.StatusInternalServerError)
		return
	}
	if pyOut.Error != "" {
		http.Error(w, pyOut.Error, http.StatusInternalServerError)
		return
	}

	resumeURL := filepath.Join(urlPath, "resume.pdf")
	coverURL := filepath.Join(urlPath, "cover_letter.pdf")

	appErr = h.Jobs.QueryUpdateResumePaths(ctx, db.QueryUpdateResumePathsParams{
		ResumePdfPath:   pgtype.Text{String: resumeURL, Valid: true},
		CoverLetterPath: pgtype.Text{String: coverURL, Valid: true},
		ID:              resumeID,
		UserID:          userID,
	})
	if appErr != nil {
		h.Logger.Error("erro ao salvar caminhos dos PDFs", zap.Error(appErr))
		handlerRepositoryError(w, appErr)
		return
	}

	h.SecLog.Insert(r.Context(), secevents.InsertParams{
		EventType: db.SecurityEventTypePdfGenerated,
		IP:        util.ClientIP(r),
		UserID:    userID.String(),
		Metadata:  []byte(`{"resume_id":"` + resumeIDStr + `"}`),
	})

	w.WriteHeader(http.StatusOK)
	errE := json.NewEncoder(w).Encode(map[string]string{
		"resume_path":       resumeURL,
		"cover_letter_path": coverURL,
	})

	if errE != nil {
		h.Logger.Error("erro ao codificar resposta", zap.Error(errE))
		http.Error(w, "erro ao codificar resposta", http.StatusInternalServerError)
		return
	}
}

func (h *JobHandler) RetryJobProcessing(w http.ResponseWriter, r *http.Request) {
	userId, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusMethodNotAllowed, ErrUserNotAllowed.Error())
		return
	}

	jobId := r.PathValue("id")
	pgJobId, errR := util.ParseUUID(jobId)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	var body struct {
		Mode string `json:"mode"`
		// Language: idioma do currículo e da cover letter ("pt-br" ou "en"); vazio mantém.
		Language string `json:"language"`
		// Fresh: pede uma resposta nova à IA em vez de reaproveitar a guardada para o
		// mesmo pedido (mesma vaga, perfil, idioma e modo).
		Fresh bool `json:"fresh"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !validJobLanguage(body.Language) {
		writeError(w, http.StatusBadRequest, "idioma inválido: use pt-br ou en")
		return
	}

	errR = h.Jobs.QueryReprocessJob(r.Context(), db.QueryReprocessJobParams{
		ID:              pgJobId,
		UserID:          userId,
		Mode:            body.Mode,
		Language:        body.Language,
		FreshGeneration: body.Fresh,
	})

	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	h.Bus.Publish(userId.String(), sse.JobEvent{ID: jobId, Status: string(db.JobStatusPending)})
	w.WriteHeader(http.StatusNoContent)
}

// Feedback
func (h *JobHandler) InsertFeedback(w http.ResponseWriter, r *http.Request) {
	GenericCreate(
		func(ctx context.Context, userId pgtype.UUID, body Feedback) *repository.RepositoryError {
			pgResumeId, err := util.ParseUUID(r.PathValue("resume_id"))
			if err != nil {
				return err
			}
			return h.Feedbacks.QueryInsertFeedback(ctx, db.QueryInsertFeedbackParams{
				ResumeID: pgResumeId,
				UserID:   userId,
				Status:   db.FeedbackStatus(body.Status),
				Comments: util.ConvertToPgText(body.Comments),
			})
		},
		TypedValidate(
			TypedRequired[Feedback]("status", "comments"),
		),
	)(w, r)
}

// validJobLanguage: idiomas em que o worker gera currículo (o prompt em inglês para
// "en", o em português para o resto). Vazio é "não mudar".
func validJobLanguage(lang string) bool {
	return lang == "" || lang == "pt-br" || lang == "ptbr" || lang == "en"
}
