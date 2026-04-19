package handler

import (
	"context"
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/feedbacks"
	"hackton-treino/internal/repository/jobs"
	"hackton-treino/internal/util"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type JobHandler struct {
	*BaseHandler
	Jobs      jobs.Repository
	Feedbacks feedbacks.Repository
	PDF       *PDFService
}

func NewJobHandler(logger *zap.Logger, conn *pgxpool.Pool, pdf *PDFService) *JobHandler {
	return &JobHandler{
		BaseHandler: NewBaseHandler(logger),
		Jobs:        jobs.New(conn),
		Feedbacks:   feedbacks.New(conn),
		PDF:         pdf,
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
		}
		jobId, errR = h.Jobs.QueryInsertUrlJob(r.Context(), jobWithUrl)
		if errR != nil {
			writeRepositoryError(w, errR)
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": jobId})
}

func (h *JobHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	GenericList(
		func(userID pgtype.UUID, query PaginationParams) db.QuerySelectJobsForUserParams {
			return db.QuerySelectJobsForUserParams{
				UserID: userID,
				Cursor: query.Cursor,
				Size:   query.Size,
			}
		},
		h.Jobs.QuerySelectJobsForUser,
		func(j db.QuerySelectJobsForUserRow) Job {
			return Job{
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
			}
		},
		func(j db.QuerySelectJobsForUserRow) int32 { return int32(j.TotalCount) },
	)(w, r)
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
		id, err := util.ParseUUID(r.PathValue("id"))
		return db.QueryDeleteResumeParams{ID: id, UserID: userID}, err
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

	var conteudo map[string]string
	if err := json.Unmarshal(row.ContentJson, &conteudo); err != nil {
		http.Error(w, "content_json inválido", http.StatusInternalServerError)
		return
	}

	outputDir, err := buildOutputDir(row.CompanyName.String, row.JobTitle.String)
	if err != nil {
		h.Logger.Error("erro ao criar diretório", zap.Error(err))
		http.Error(w, "erro ao preparar diretório", http.StatusInternalServerError)
		return
	}

	pyOut, err := h.PDF.Generate(ctx, pythonInput{
		Curriculo:   conteudo["curriculo"],
		CoverLetter: conteudo["cover_letter"],
		OutputDir:   outputDir,
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

	appErr = h.Jobs.QueryUpdateResumePaths(ctx, db.QueryUpdateResumePathsParams{
		ResumePdfPath:   pgtype.Text{String: pyOut.ResumePath, Valid: true},
		CoverLetterPath: pgtype.Text{String: pyOut.CoverLetterPath, Valid: true},
		ID:              resumeID,
		UserID:          userID,
	})
	if appErr != nil {
		h.Logger.Error("erro ao salvar caminhos dos PDFs", zap.Error(appErr))
		handlerRepositoryError(w, appErr)
		return
	}

	w.WriteHeader(http.StatusOK)
	errE := json.NewEncoder(w).Encode(map[string]string{
		"resume_path":       pyOut.ResumePath,
		"cover_letter_path": pyOut.CoverLetterPath,
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

	errR = h.Jobs.QueryReprocessJob(r.Context(), db.QueryReprocessJobParams{
		ID:     pgJobId,
		UserID: userId,
	})

	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

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
