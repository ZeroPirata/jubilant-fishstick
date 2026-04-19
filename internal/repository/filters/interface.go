package filters

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type filtersRepository struct {
	repository.Base
}

type Repository interface {
	QuerySelectFiltersForUser(ctx context.Context, userID string) ([]string, *repository.RepositoryError)
	QuerySelectFiltersForUserWithID(ctx context.Context, params db.QuerySelectFiltersForUserWithIDParams) ([]db.QuerySelectFiltersForUserWithIDRow, *repository.RepositoryError)
	QueryInsertFilter(ctx context.Context, args db.QueryInsertFilterParams) (db.UserJobFilter, *repository.RepositoryError)
	QueryDeleteFilter(ctx context.Context, args db.QueryDeleteFilterParams) *repository.RepositoryError
}

func New(conn *pgxpool.Pool) Repository {
	return &filtersRepository{Base: repository.NewBase(conn)}
}
