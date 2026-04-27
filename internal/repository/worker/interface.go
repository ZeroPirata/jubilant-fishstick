package worker

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type workerRepository struct {
	repository.Base
}

type Repository interface {
	WorkerSelectPendingJobs(ctx context.Context, maxCount int32) ([]db.Job, *repository.RepositoryError)
	WorkerUpdateJobStatus(ctx context.Context, args db.WorkerUpdateJobStatusParams) *repository.RepositoryError
	WorkerUpdateJob(ctx context.Context, args db.WorkerUpdateJobParams) *repository.RepositoryError
	WorkerUpdateJobQuality(ctx context.Context, args db.WorkerUpdateJobQualityParams) *repository.RepositoryError
	WorkerInsertGeneratedResume(ctx context.Context, args db.WorkerInsertGeneratedResumeParams) (string, *repository.RepositoryError)
	WorkerRecoverStuckJobs(ctx context.Context, cutoff pgtype.Timestamptz) (int64, *repository.RepositoryError)
}

func New(conn *pgxpool.Pool) Repository {
	return &workerRepository{Base: repository.NewBase(conn)}
}
