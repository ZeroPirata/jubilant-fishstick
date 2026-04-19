package feedbacks

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type feedbackRepository struct {
	repository.Base
}

type Repository interface {
	QueryInsertFeedback(ctx context.Context, args db.QueryInsertFeedbackParams) *repository.RepositoryError
	QuerySelectFeedbackByResume(ctx context.Context, args db.QuerySelectFeedbackByResumeParams) (db.ResumesFeedback, *repository.RepositoryError)
	QuerySelectGoodFeedbacks(ctx context.Context, userID string) ([]string, *repository.RepositoryError)
	QuerySelectExcellentResumes(ctx context.Context, userID string) ([][]byte, *repository.RepositoryError)
}

func New(conn *pgxpool.Pool) Repository {
	return &feedbackRepository{Base: repository.NewBase(conn)}
}
