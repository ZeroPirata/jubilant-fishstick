package jobs

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgtype"
)

// ApplicationAddEvent grava uma mudança de etapa (ou anotação) da candidatura e
// recalcula a etapa atual da vaga, numa transação. 404 se a vaga não é do usuário.
func (r *jobRepository) ApplicationAddEvent(ctx context.Context, args db.QueryInsertApplicationEventParams) (string, *repository.RepositoryError) {
	var id pgtype.UUID
	errR := r.ExecTx(ctx, func(q *db.Queries) error {
		if _, err := q.QueryApplicationJobOwned(ctx, db.QueryApplicationJobOwnedParams{ID: args.JobID, UserID: args.UserID}); err != nil {
			return err
		}
		var err error
		if id, err = q.QueryInsertApplicationEvent(ctx, args); err != nil {
			return err
		}
		return q.QueryRecomputeApplicationStage(ctx, db.QueryRecomputeApplicationStageParams{UserID: args.UserID, JobID: args.JobID})
	})
	if errR != nil {
		return "", errR
	}
	return id.String(), nil
}

// ApplicationDeleteEvent apaga um item do histórico (desfazer) e recalcula a etapa.
func (r *jobRepository) ApplicationDeleteEvent(ctx context.Context, userID, jobID, eventID pgtype.UUID) *repository.RepositoryError {
	return r.ExecTx(ctx, func(q *db.Queries) error {
		if _, err := q.QueryApplicationJobOwned(ctx, db.QueryApplicationJobOwnedParams{ID: jobID, UserID: userID}); err != nil {
			return err
		}
		if _, err := q.QueryDeleteApplicationEvent(ctx, db.QueryDeleteApplicationEventParams{ID: eventID, JobID: jobID, UserID: userID}); err != nil {
			return err
		}
		return q.QueryRecomputeApplicationStage(ctx, db.QueryRecomputeApplicationStageParams{UserID: userID, JobID: jobID})
	})
}

func (r *jobRepository) ApplicationEvents(ctx context.Context, userID, jobID pgtype.UUID) ([]db.QueryListApplicationEventsRow, *repository.RepositoryError) {
	rows, err := r.Q.QueryListApplicationEvents(ctx, db.QueryListApplicationEventsParams{JobID: jobID, UserID: userID})
	return rows, repository.HandleDatabaseError(err)
}

func (r *jobRepository) ApplicationBoard(ctx context.Context, userID pgtype.UUID) ([]db.QueryApplicationBoardRow, *repository.RepositoryError) {
	rows, err := r.Q.QueryApplicationBoard(ctx, userID)
	return rows, repository.HandleDatabaseError(err)
}
