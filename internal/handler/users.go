package handler

import (
	"context"
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	ucache "hackton-treino/internal/repository/cache"
	"hackton-treino/internal/repository/users"
	"hackton-treino/internal/util"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type UserHandler struct {
	*BaseHandler
	Users users.Repository
	Cache ucache.Cache
}

func NewUserHandler(logger *zap.Logger, conn *pgxpool.Pool, rds *redis.Client) *UserHandler {
	return &UserHandler{
		BaseHandler: NewBaseHandler(logger),
		Users:       users.New(conn),
		Cache:       ucache.New(rds),
	}
}

// invalidate deletes a cache topic for a user, logging but not failing on error.
func (h *UserHandler) invalidate(ctx context.Context, userID, topic string) {
	if err := h.Cache.Delete(ctx, userID, topic); err != nil {
		h.Logger.Warn("cache: falha ao invalidar", zap.String("topic", topic), zap.Error(err))
	}
}

// Profile

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}
	uid := userID.String()

	if cached, hit := ucache.GetTyped[profileResponse](r.Context(), h.Cache, uid, ucache.TopicProfile); hit {
		writeJSON(w, http.StatusOK, cached)
		return
	}

	row, errR := h.Users.QuerySelectProfile(r.Context(), uid)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	resp := profileResponse{
		Email:        row.Email,
		FullName:     row.FullName,
		Phone:        row.Phone,
		About:        row.About,
		ContactEmail: row.ContactEmail,
		LinkedinUrl:  row.LinkedinUrl,
		GithubUrl:    row.GithubUrl,
		PortfolioUrl: row.PortfolioUrl,
	}

	_ = ucache.SetTyped(r.Context(), h.Cache, uid, ucache.TopicProfile, resp)
	writeJSON(w, http.StatusOK, resp)
}

func (h *UserHandler) UpsertProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}

	var req upsertProfileReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}

	if err := Validate(Required("full_name", req.FullName)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	row, errR := h.Users.QueryUpsertProfile(r.Context(), db.QueryUpsertProfileParams{
		UserID:       userID,
		FullName:     req.FullName,
		Phone:        util.ConvertToPgTextPtr(req.Phone),
		About:        util.ConvertToPgTextPtr(req.About),
		ContactEmail: util.ConvertToPgTextPtr(req.ContactEmail),
	})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	h.invalidate(r.Context(), userID.String(), ucache.TopicProfile)
	writeJSON(w, http.StatusOK, row)
}

func (h *UserHandler) UpsertLinks(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrUserNotAllowed.Error())
		return
	}

	var req upsertLinksReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}

	row, errR := h.Users.QueryUpsertLinks(r.Context(), db.QueryUpsertLinksParams{
		UserID:       userID,
		LinkedinUrl:  util.ConvertToPgTextPtr(req.LinkedinUrl),
		GithubUrl:    util.ConvertToPgTextPtr(req.GithubUrl),
		PortfolioUrl: util.ConvertToPgTextPtr(req.PortfolioUrl),
		OtherLinks:   []byte("null"),
	})
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	// links are part of the profile response
	h.invalidate(r.Context(), userID.String(), ucache.TopicProfile)
	writeJSON(w, http.StatusOK, row)
}

// Experiences

func (h *UserHandler) ListExperiences(w http.ResponseWriter, r *http.Request) {
	GenericList(
		func(userID pgtype.UUID, q PaginationParams) db.QueryListExperiencesParams {
			search := pgtype.Text{}
			if q.Search != nil {
				search = pgtype.Text{String: *q.Search, Valid: true}
			}
			return db.QueryListExperiencesParams{UserID: userID, Search: search, Cursor: q.Cursor, Size: q.Size}
		},
		h.Users.QueryListExperiences,
		func(r db.QueryListExperiencesRow) db.QueryListExperiencesRow { return r },
		func(r db.QueryListExperiencesRow) int32 { return int32(r.TotalCount) },
	)(w, r)
}

func (h *UserHandler) InsertExperience(w http.ResponseWriter, r *http.Request) {
	GenericCreate(
		func(ctx context.Context, userID pgtype.UUID, req experienceReq) *repository.RepositoryError {
			_, errR := h.Users.QueryInsertExperience(ctx, db.QueryInsertExperienceParams{
				UserID:       userID,
				CompanyName:  req.CompanyName,
				JobRole:      req.JobRole,
				Description:  util.ConvertToPgText(req.Description),
				IsCurrentJob: req.IsCurrentJob,
				StartDate:    util.ParsePgDate(req.StartDate),
				EndDate:      util.ParsePgDate(req.EndDate),
				TechStack:    req.TechStack,
				Achievements: req.Achievements,
				Tags:         req.Tags,
			})
			if errR == nil {
				h.invalidate(ctx, userID.String(), ucache.TopicExperiences)
			}
			return errR
		},
		TypedValidate(TypedRequired[experienceReq]("company_name", "job_role", "start_date")),
	)(w, r)
}

func (h *UserHandler) UpdateExperience(w http.ResponseWriter, r *http.Request) {
	GenericUpdate(
		func(userID pgtype.UUID, r *http.Request, req experienceReq) (db.QueryUpdateExperienceParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryUpdateExperienceParams{
				ID:           id,
				UserID:       userID,
				CompanyName:  req.CompanyName,
				JobRole:      req.JobRole,
				Description:  util.ConvertToPgText(req.Description),
				IsCurrentJob: req.IsCurrentJob,
				StartDate:    util.ParsePgDate(req.StartDate),
				EndDate:      util.ParsePgDate(req.EndDate),
				TechStack:    req.TechStack,
				Achievements: req.Achievements,
				Tags:         req.Tags,
			}, appErr
		},
		func(ctx context.Context, p db.QueryUpdateExperienceParams) *repository.RepositoryError {
			errR := h.Users.QueryUpdateExperience(ctx, p)
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicExperiences)
			}
			return errR
		},
		TypedValidate(TypedRequired[experienceReq]("company_name", "job_role", "start_date")),
	)(w, r)
}

func (h *UserHandler) DeleteExperience(w http.ResponseWriter, r *http.Request) {
	GenericDelete(
		func(userID pgtype.UUID, r *http.Request) (db.QueryDeleteExperienceParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryDeleteExperienceParams{ID: id, UserID: userID}, appErr
		},
		func(ctx context.Context, p db.QueryDeleteExperienceParams) *repository.RepositoryError {
			errR := h.Users.QueryDeleteExperience(ctx, p.ID.String(), p.UserID.String())
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicExperiences)
			}
			return errR
		},
	)(w, r)
}

// Academic

func (h *UserHandler) ListAcademic(w http.ResponseWriter, r *http.Request) {
	GenericList(
		func(userID pgtype.UUID, q PaginationParams) db.QueryListAcademicHistoriesParams {
			search := pgtype.Text{}
			if q.Search != nil {
				search = pgtype.Text{String: *q.Search, Valid: true}
			}
			return db.QueryListAcademicHistoriesParams{UserID: userID, Search: search, Cursor: q.Cursor, Size: q.Size}
		},
		h.Users.QueryListAcademicHistories,
		func(r db.QueryListAcademicHistoriesRow) db.QueryListAcademicHistoriesRow { return r },
		func(r db.QueryListAcademicHistoriesRow) int32 { return int32(r.TotalCount) },
	)(w, r)
}

func (h *UserHandler) InsertAcademic(w http.ResponseWriter, r *http.Request) {
	GenericCreate(
		func(ctx context.Context, userID pgtype.UUID, req academicReq) *repository.RepositoryError {
			_, errR := h.Users.QueryInsertAcademicHistory(ctx, db.QueryInsertAcademicHistoryParams{
				UserID:          userID,
				InstitutionName: req.InstitutionName,
				CourseName:      req.CourseName,
				StartDate:       util.ParsePgDate(req.StartDate),
				EndDate:         util.ParsePgDate(req.EndDate),
				Description:     util.ConvertToPgText(req.Description),
			})
			if errR == nil {
				h.invalidate(ctx, userID.String(), ucache.TopicAcademic)
			}
			return errR
		},
		TypedValidate(TypedRequired[academicReq]("institution_name", "course_name", "start_date")),
	)(w, r)
}

func (h *UserHandler) UpdateAcademic(w http.ResponseWriter, r *http.Request) {
	GenericUpdate(
		func(userID pgtype.UUID, r *http.Request, req academicReq) (db.QueryUpdateAcademicHistoryParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryUpdateAcademicHistoryParams{
				ID:              id,
				UserID:          userID,
				InstitutionName: req.InstitutionName,
				CourseName:      req.CourseName,
				StartDate:       util.ParsePgDate(req.StartDate),
				EndDate:         util.ParsePgDate(req.EndDate),
				Description:     util.ConvertToPgText(req.Description),
			}, appErr
		},
		func(ctx context.Context, p db.QueryUpdateAcademicHistoryParams) *repository.RepositoryError {
			errR := h.Users.QueryUpdateAcademicHistory(ctx, p)
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicAcademic)
			}
			return errR
		},
		TypedValidate(TypedRequired[academicReq]("institution_name", "course_name", "start_date")),
	)(w, r)
}

func (h *UserHandler) DeleteAcademic(w http.ResponseWriter, r *http.Request) {
	GenericDelete(
		func(userID pgtype.UUID, r *http.Request) (db.QueryDeleteAcademicHistoryParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryDeleteAcademicHistoryParams{ID: id, UserID: userID}, appErr
		},
		func(ctx context.Context, p db.QueryDeleteAcademicHistoryParams) *repository.RepositoryError {
			errR := h.Users.QueryDeleteAcademicHistory(ctx, p.ID.String(), p.UserID.String())
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicAcademic)
			}
			return errR
		},
	)(w, r)
}

// Skills

func (h *UserHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
	GenericList(
		func(userID pgtype.UUID, q PaginationParams) db.QueryListSkillsParams {
			search := pgtype.Text{}
			if q.Search != nil {
				search = pgtype.Text{String: *q.Search, Valid: true}
			}
			return db.QueryListSkillsParams{UserID: userID, Search: search, Cursor: q.Cursor, Size: q.Size}
		},
		h.Users.QueryListSkills,
		func(r db.QueryListSkillsRow) db.QueryListSkillsRow { return r },
		func(r db.QueryListSkillsRow) int32 { return int32(r.TotalCount) },
	)(w, r)
}

func (h *UserHandler) InsertSkill(w http.ResponseWriter, r *http.Request) {
	GenericCreate(
		func(ctx context.Context, userID pgtype.UUID, req skillReq) *repository.RepositoryError {
			_, errR := h.Users.QueryInsertSkill(ctx, db.QueryInsertSkillParams{
				UserID:           userID,
				SkillName:        req.SkillName,
				ProficiencyLevel: db.SkillLevel(req.ProficiencyLevel),
				Tags:             req.Tags,
			})
			if errR == nil {
				h.invalidate(ctx, userID.String(), ucache.TopicSkills)
			}
			return errR
		},
		TypedValidate(TypedRequired[skillReq]("skill_name", "proficiency_level")),
	)(w, r)
}

func (h *UserHandler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	GenericUpdate(
		func(userID pgtype.UUID, r *http.Request, req skillReq) (db.QueryUpdateSkillParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryUpdateSkillParams{
				ID:               id,
				UserID:           userID,
				SkillName:        req.SkillName,
				ProficiencyLevel: db.SkillLevel(req.ProficiencyLevel),
				Tags:             req.Tags,
			}, appErr
		},
		func(ctx context.Context, p db.QueryUpdateSkillParams) *repository.RepositoryError {
			errR := h.Users.QueryUpdateSkill(ctx, p)
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicSkills)
			}
			return errR
		},
		TypedValidate(TypedRequired[skillReq]("skill_name", "proficiency_level")),
	)(w, r)
}

func (h *UserHandler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	GenericDelete(
		func(userID pgtype.UUID, r *http.Request) (db.QueryDeleteSkillParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryDeleteSkillParams{ID: id, UserID: userID}, appErr
		},
		func(ctx context.Context, p db.QueryDeleteSkillParams) *repository.RepositoryError {
			errR := h.Users.QueryDeleteSkill(ctx, p.ID.String(), p.UserID.String())
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicSkills)
			}
			return errR
		},
	)(w, r)
}

// Projects

func (h *UserHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	GenericList(
		func(userID pgtype.UUID, q PaginationParams) db.QueryListProjectsParams {
			search := pgtype.Text{}
			if q.Search != nil {
				search = pgtype.Text{String: *q.Search, Valid: true}
			}
			return db.QueryListProjectsParams{UserID: userID, Search: search, Cursor: q.Cursor, Size: q.Size}
		},
		h.Users.QueryListProjects,
		func(r db.QueryListProjectsRow) db.QueryListProjectsRow { return r },
		func(r db.QueryListProjectsRow) int32 { return int32(r.TotalCount) },
	)(w, r)
}

func (h *UserHandler) InsertProject(w http.ResponseWriter, r *http.Request) {
	GenericCreate(
		func(ctx context.Context, userID pgtype.UUID, req projectReq) *repository.RepositoryError {
			_, errR := h.Users.QueryInsertProject(ctx, db.QueryInsertProjectParams{
				UserID:      userID,
				ProjectName: req.ProjectName,
				Description: req.Description,
				ProjectUrl:  util.ConvertToPgText(req.ProjectUrl),
				Tags:        req.Tags,
				StartDate:   util.ParsePgDate(req.StartDate),
				EndDate:     util.ParsePgDate(req.EndDate),
				IsAcademic:  req.IsAcademic,
			})
			if errR == nil {
				h.invalidate(ctx, userID.String(), ucache.TopicProjects)
			}
			return errR
		},
		TypedValidate(TypedRequired[projectReq]("project_name", "description", "start_date")),
	)(w, r)
}

func (h *UserHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	GenericUpdate(
		func(userID pgtype.UUID, r *http.Request, req projectReq) (db.QueryUpdateProjectParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryUpdateProjectParams{
				ID:          id,
				UserID:      userID,
				ProjectName: req.ProjectName,
				Description: req.Description,
				ProjectUrl:  util.ConvertToPgText(req.ProjectUrl),
				Tags:        req.Tags,
				StartDate:   util.ParsePgDate(req.StartDate),
				EndDate:     util.ParsePgDate(req.EndDate),
				IsAcademic:  req.IsAcademic,
			}, appErr
		},
		func(ctx context.Context, p db.QueryUpdateProjectParams) *repository.RepositoryError {
			errR := h.Users.QueryUpdateProject(ctx, p)
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicProjects)
			}
			return errR
		},
		TypedValidate(TypedRequired[projectReq]("project_name", "description", "start_date")),
	)(w, r)
}

func (h *UserHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	GenericDelete(
		func(userID pgtype.UUID, r *http.Request) (db.QueryDeleteProjectParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryDeleteProjectParams{ID: id, UserID: userID}, appErr
		},
		func(ctx context.Context, p db.QueryDeleteProjectParams) *repository.RepositoryError {
			errR := h.Users.QueryDeleteProject(ctx, p.ID.String(), p.UserID.String())
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicProjects)
			}
			return errR
		},
	)(w, r)
}

// Certificates

func (h *UserHandler) ListCertificates(w http.ResponseWriter, r *http.Request) {
	GenericList(
		func(userID pgtype.UUID, q PaginationParams) db.QueryListCertificatesParams {
			search := pgtype.Text{}
			if q.Search != nil {
				search = pgtype.Text{String: *q.Search, Valid: true}
			}
			return db.QueryListCertificatesParams{UserID: userID, Search: search, Cursor: q.Cursor, Size: q.Size}
		},
		h.Users.QueryListCertificates,
		func(r db.QueryListCertificatesRow) db.QueryListCertificatesRow { return r },
		func(r db.QueryListCertificatesRow) int32 { return int32(r.TotalCount) },
	)(w, r)
}

func (h *UserHandler) InsertCertificate(w http.ResponseWriter, r *http.Request) {
	GenericCreate(
		func(ctx context.Context, userID pgtype.UUID, req certificateReq) *repository.RepositoryError {
			_, errR := h.Users.QueryInsertCertificate(ctx, db.QueryInsertCertificateParams{
				UserID:              userID,
				CertificateName:     req.CertificateName,
				IssuingOrganization: req.IssuingOrganization,
				IssueDate:           util.ParsePgDate(req.IssueDate),
				CredentialUrl:       util.ConvertToPgText(req.CredentialUrl),
				Tags:                req.Tags,
			})
			if errR == nil {
				h.invalidate(ctx, userID.String(), ucache.TopicCertificates)
			}
			return errR
		},
		TypedValidate(TypedRequired[certificateReq]("certificate_name", "issuing_organization", "issue_date")),
	)(w, r)
}

func (h *UserHandler) UpdateCertificate(w http.ResponseWriter, r *http.Request) {
	GenericUpdate(
		func(userID pgtype.UUID, r *http.Request, req certificateReq) (db.QueryUpdateCertificateParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryUpdateCertificateParams{
				ID:                  id,
				UserID:              userID,
				CertificateName:     req.CertificateName,
				IssuingOrganization: req.IssuingOrganization,
				IssueDate:           util.ParsePgDate(req.IssueDate),
				CredentialUrl:       util.ConvertToPgText(req.CredentialUrl),
				Tags:                req.Tags,
			}, appErr
		},
		func(ctx context.Context, p db.QueryUpdateCertificateParams) *repository.RepositoryError {
			errR := h.Users.QueryUpdateCertificate(ctx, p)
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicCertificates)
			}
			return errR
		},
		TypedValidate(TypedRequired[certificateReq]("certificate_name", "issuing_organization", "issue_date")),
	)(w, r)
}

func (h *UserHandler) DeleteCertificate(w http.ResponseWriter, r *http.Request) {
	GenericDelete(
		func(userID pgtype.UUID, r *http.Request) (db.QueryDeleteCertificateParams, *repository.RepositoryError) {
			id, appErr := util.ParseUUID(r.PathValue("id"))
			return db.QueryDeleteCertificateParams{ID: id, UserID: userID}, appErr
		},
		func(ctx context.Context, p db.QueryDeleteCertificateParams) *repository.RepositoryError {
			errR := h.Users.QueryDeleteCertificate(ctx, p.ID.String(), p.UserID.String())
			if errR == nil {
				h.invalidate(ctx, p.UserID.String(), ucache.TopicCertificates)
			}
			return errR
		},
	)(w, r)
}
