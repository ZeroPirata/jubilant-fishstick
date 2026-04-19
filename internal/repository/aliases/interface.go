package aliases

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type aliasesRepository struct {
	repository.Base
}

type Repository interface {
	QuerySelectAllStackAliases(ctx context.Context) (map[string]string, *repository.RepositoryError)
	QuerySelectAllStackAliasesWithID(ctx context.Context) ([]db.QuerySelectAllStackAliasesWithIDRow, *repository.RepositoryError)
	QueryInsertStackAlias(ctx context.Context, args db.QueryInsertStackAliasParams) (db.StackAlias, *repository.RepositoryError)
	QueryDeleteStackAlias(ctx context.Context, id string) *repository.RepositoryError
}

func New(conn *pgxpool.Pool) Repository {
	return &aliasesRepository{Base: repository.NewBase(conn)}
}
