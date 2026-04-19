package filters

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/util"
)

func (r *filtersRepository) QuerySelectFiltersForUser(ctx context.Context, userID string) ([]string, *repository.RepositoryError) {
	uid, appErr := util.ParseUUID(userID)
	if appErr != nil {
		return nil, appErr
	}
	rows, err := r.Q.QuerySelectFiltersForUser(ctx, uid)
	return rows, repository.HandleDatabaseError(err)
}

func (r *filtersRepository) QuerySelectFiltersForUserWithID(ctx context.Context, params db.QuerySelectFiltersForUserWithIDParams) ([]db.QuerySelectFiltersForUserWithIDRow, *repository.RepositoryError) {
	rows, err := r.Q.QuerySelectFiltersForUserWithID(ctx, params)
	return rows, repository.HandleDatabaseError(err)
}

func (r *filtersRepository) QueryInsertFilter(ctx context.Context, args db.QueryInsertFilterParams) (db.UserJobFilter, *repository.RepositoryError) {
	row, err := r.Q.QueryInsertFilter(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *filtersRepository) QueryDeleteFilter(ctx context.Context, args db.QueryDeleteFilterParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.QueryDeleteFilter(ctx, args))
}
