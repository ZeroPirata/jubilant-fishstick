package jobs

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type jobRepository struct {
	repository.Base
}

type Repository interface {
	QueryInsertUrlJob(ctx context.Context, args db.QueryInsertUrlJobParams) (string, *repository.RepositoryError)
	QueryInsertFullJob(ctx context.Context, args db.QueryInsertFullJobParams) (string, *repository.RepositoryError)
	QuerySelectJobsForUser(ctx context.Context, args db.QuerySelectJobsForUserParams) ([]db.QuerySelectJobsForUserRow, *repository.RepositoryError)
	QueryReprocessJob(ctx context.Context, args db.QueryReprocessJobParams) *repository.RepositoryError
	QueryUpdateJob(ctx context.Context, args db.QueryUpdateJobParams) *repository.RepositoryError
	QueryDeleteJob(ctx context.Context, args db.QueryDeleteJobParams) *repository.RepositoryError
	QuerySelectResumeJob(ctx context.Context, args db.QuerySelectResumeJobParams) (db.QuerySelectResumeJobRow, *repository.RepositoryError)
	QueryUpdateResumePaths(ctx context.Context, args db.QueryUpdateResumePathsParams) *repository.RepositoryError
	QueryFindJobByUrl(ctx context.Context, args db.QueryFindJobByUrlParams) (string, *repository.RepositoryError)
	QueryListResumesForUser(ctx context.Context, args db.QueryListResumesForUserParams) ([]db.QueryListResumesForUserRow, *repository.RepositoryError)
	QueryDeleteResume(ctx context.Context, args db.QueryDeleteResumeParams) *repository.RepositoryError
}

func New(conn *pgxpool.Pool) Repository {
	return &jobRepository{Base: repository.NewBase(conn)}
}
