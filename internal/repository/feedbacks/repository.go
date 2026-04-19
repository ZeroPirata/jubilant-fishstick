package feedbacks

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/util"
)

func (r *feedbackRepository) QueryInsertFeedback(ctx context.Context, args db.QueryInsertFeedbackParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.QueryInsertFeedback(ctx, args))
}

func (r *feedbackRepository) QuerySelectFeedbackByResume(ctx context.Context, args db.QuerySelectFeedbackByResumeParams) (db.ResumesFeedback, *repository.RepositoryError) {
	row, err := r.Q.QuerySelectFeedbackByResume(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *feedbackRepository) QuerySelectGoodFeedbacks(ctx context.Context, userID string) ([]string, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectGoodFeedbacks(ctx, uid)
	if err != nil {
		return nil, repository.HandleDatabaseError(err)
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Valid {
			result = append(result, row.String)
		}
	}
	return result, nil
}

func (r *feedbackRepository) QuerySelectExcellentResumes(ctx context.Context, userID string) ([][]byte, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectExcellentResumes(ctx, uid)
	return rows, repository.HandleDatabaseError(err)
}
