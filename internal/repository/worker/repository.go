package worker

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgtype"
)

func (r *workerRepository) WorkerSelectPendingJobs(ctx context.Context, maxCount int32) ([]db.Job, *repository.RepositoryError) {
	jobs, err := r.Q.WorkerSelectPendingJobs(ctx, maxCount)
	return jobs, repository.HandleDatabaseError(err)
}

func (r *workerRepository) WorkerUpdateJobStatus(ctx context.Context, args db.WorkerUpdateJobStatusParams) *repository.RepositoryError {
	err := r.Q.WorkerUpdateJobStatus(ctx, args)
	return repository.HandleDatabaseError(err)
}

func (r *workerRepository) WorkerUpdateJob(ctx context.Context, args db.WorkerUpdateJobParams) *repository.RepositoryError {
	err := r.Q.WorkerUpdateJob(ctx, args)
	return repository.HandleDatabaseError(err)
}

func (r *workerRepository) WorkerUpdateJobQuality(ctx context.Context, args db.WorkerUpdateJobQualityParams) *repository.RepositoryError {
	err := r.Q.WorkerUpdateJobQuality(ctx, args)
	return repository.HandleDatabaseError(err)
}

func (r *workerRepository) WorkerInsertGeneratedResume(ctx context.Context, args db.WorkerInsertGeneratedResumeParams) (string, *repository.RepositoryError) {
	id, err := r.Q.WorkerInsertGeneratedResume(ctx, args)
	return id.String(), repository.HandleDatabaseError(err)
}

func (r *workerRepository) WorkerRecoverStuckJobs(ctx context.Context, cutoff pgtype.Timestamptz) (int64, *repository.RepositoryError) {
	n, err := r.Q.WorkerRecoverStuckJobs(ctx, cutoff)
	return n, repository.HandleDatabaseError(err)
}
