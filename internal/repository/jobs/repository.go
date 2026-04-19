package jobs

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
)

func (r *jobRepository) QueryInsertUrlJob(ctx context.Context, args db.QueryInsertUrlJobParams) (string, *repository.RepositoryError) {
	jobId, err := r.Q.QueryInsertUrlJob(ctx, args)
	return jobId.String(), repository.HandleDatabaseError(err)
}

func (r *jobRepository) QueryInsertFullJob(ctx context.Context, args db.QueryInsertFullJobParams) (string, *repository.RepositoryError) {
	jobId, err := r.Q.QueryInsertFullJob(ctx, args)
	return jobId.String(), repository.HandleDatabaseError(err)
}

func (r *jobRepository) QuerySelectJobsForUser(ctx context.Context, args db.QuerySelectJobsForUserParams) ([]db.QuerySelectJobsForUserRow, *repository.RepositoryError) {
	jobs, err := r.Q.QuerySelectJobsForUser(ctx, args)
	return jobs, repository.HandleDatabaseError(err)
}

func (r *jobRepository) QueryReprocessJob(ctx context.Context, args db.QueryReprocessJobParams) *repository.RepositoryError {
	err := r.Q.QueryReprocessJob(ctx, args)
	return repository.HandleDatabaseError(err)
}

func (r *jobRepository) QueryUpdateJob(ctx context.Context, args db.QueryUpdateJobParams) *repository.RepositoryError {
	err := r.Q.QueryUpdateJob(ctx, args)
	return repository.HandleDatabaseError(err)
}

func (r *jobRepository) QueryDeleteJob(ctx context.Context, args db.QueryDeleteJobParams) *repository.RepositoryError {
	err := r.Q.QueryDeleteJob(ctx, args)
	return repository.HandleDatabaseError(err)
}

func (r *jobRepository) QuerySelectResumeJob(ctx context.Context, args db.QuerySelectResumeJobParams) (db.QuerySelectResumeJobRow, *repository.RepositoryError) {
	resume, err := r.Q.QuerySelectResumeJob(ctx, args)
	return resume, repository.HandleDatabaseError(err)
}

func (r *jobRepository) QueryUpdateResumePaths(ctx context.Context, args db.QueryUpdateResumePathsParams) *repository.RepositoryError {
	err := r.Q.QueryUpdateResumePaths(ctx, args)
	return repository.HandleDatabaseError(err)
}

func (r *jobRepository) QueryFindJobByUrl(ctx context.Context, args db.QueryFindJobByUrlParams) (string, *repository.RepositoryError) {
	uuid, err := r.Q.QueryFindJobByUrl(ctx, args)
	return uuid.String(), repository.HandleDatabaseError(err)
}

func (r *jobRepository) QueryListResumesForUser(ctx context.Context, args db.QueryListResumesForUserParams) ([]db.QueryListResumesForUserRow, *repository.RepositoryError) {
	resumes, err := r.Q.QueryListResumesForUser(ctx, args)
	return resumes, repository.HandleDatabaseError(err)
}

func (r *jobRepository) QueryDeleteResume(ctx context.Context, args db.QueryDeleteResumeParams) *repository.RepositoryError {
	err := r.Q.QueryDeleteResume(ctx, args)
	return repository.HandleDatabaseError(err)
}
