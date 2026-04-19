package users

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type usersRepository struct {
	repository.Base
}

type Repository interface {
	// Account
	QueryInsertAccount(ctx context.Context, args db.QueryInsertAccountParams) (db.QueryInsertAccountRow, *repository.RepositoryError)
	QuerySelectAccountByEmail(ctx context.Context, email string) (db.QuerySelectAccountByEmailRow, *repository.RepositoryError)
	QuerySoftDeleteAccount(ctx context.Context, id string) *repository.RepositoryError

	// Profile & Links
	QueryUpsertProfile(ctx context.Context, args db.QueryUpsertProfileParams) (db.UserProfile, *repository.RepositoryError)
	QuerySelectProfile(ctx context.Context, userID string) (db.QuerySelectProfileRow, *repository.RepositoryError)
	QueryUpsertLinks(ctx context.Context, args db.QueryUpsertLinksParams) (db.UserLink, *repository.RepositoryError)

	// Experiences
	QuerySelectAllExperiences(ctx context.Context, userID string) ([]db.UserExperience, *repository.RepositoryError)
	QuerySelectExperiencesByTags(ctx context.Context, userID string, tags []string) ([]db.UserExperience, *repository.RepositoryError)
	QueryInsertExperience(ctx context.Context, args db.QueryInsertExperienceParams) (db.UserExperience, *repository.RepositoryError)
	QueryUpdateExperience(ctx context.Context, args db.QueryUpdateExperienceParams) *repository.RepositoryError
	QueryDeleteExperience(ctx context.Context, id, userID string) *repository.RepositoryError

	// Academic
	QuerySelectAllAcademicHistories(ctx context.Context, userID string) ([]db.UserAcademicHistory, *repository.RepositoryError)
	QueryInsertAcademicHistory(ctx context.Context, args db.QueryInsertAcademicHistoryParams) (db.UserAcademicHistory, *repository.RepositoryError)
	QueryUpdateAcademicHistory(ctx context.Context, args db.QueryUpdateAcademicHistoryParams) *repository.RepositoryError
	QueryDeleteAcademicHistory(ctx context.Context, id, userID string) *repository.RepositoryError

	// Skills
	QuerySelectAllSkills(ctx context.Context, userID string) ([]db.UserSkill, *repository.RepositoryError)
	QuerySelectSkillsByTags(ctx context.Context, userID string, tags []string) ([]db.UserSkill, *repository.RepositoryError)
	QueryInsertSkill(ctx context.Context, args db.QueryInsertSkillParams) (db.UserSkill, *repository.RepositoryError)
	QueryUpdateSkill(ctx context.Context, args db.QueryUpdateSkillParams) *repository.RepositoryError
	QueryDeleteSkill(ctx context.Context, id, userID string) *repository.RepositoryError

	// Projects
	QuerySelectAllProjects(ctx context.Context, userID string) ([]db.UserProject, *repository.RepositoryError)
	QuerySelectProjectsByTags(ctx context.Context, userID string, tags []string) ([]db.UserProject, *repository.RepositoryError)
	QueryInsertProject(ctx context.Context, args db.QueryInsertProjectParams) (db.UserProject, *repository.RepositoryError)
	QueryUpdateProject(ctx context.Context, args db.QueryUpdateProjectParams) *repository.RepositoryError
	QueryDeleteProject(ctx context.Context, id, userID string) *repository.RepositoryError

	// Certificates
	QuerySelectAllCertificates(ctx context.Context, userID string) ([]db.UserCertificate, *repository.RepositoryError)
	QueryInsertCertificate(ctx context.Context, args db.QueryInsertCertificateParams) (db.UserCertificate, *repository.RepositoryError)
	QueryUpdateCertificate(ctx context.Context, args db.QueryUpdateCertificateParams) *repository.RepositoryError
	QueryDeleteCertificate(ctx context.Context, id, userID string) *repository.RepositoryError
}

func New(conn *pgxpool.Pool) Repository {
	return &usersRepository{Base: repository.NewBase(conn)}
}
