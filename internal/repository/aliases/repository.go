package aliases

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/util"
)

func (r *aliasesRepository) QuerySelectAllStackAliases(ctx context.Context) (map[string]string, *repository.RepositoryError) {
	rows, err := r.Q.QuerySelectAllStackAliases(ctx)
	if err != nil {
		return nil, repository.HandleDatabaseError(err)
	}
	aliases := make(map[string]string, len(rows))
	for _, row := range rows {
		aliases[row.AliasFrom] = row.AliasTo
	}
	return aliases, nil
}

func (r *aliasesRepository) QuerySelectAllStackAliasesWithID(ctx context.Context) ([]db.QuerySelectAllStackAliasesWithIDRow, *repository.RepositoryError) {
	rows, err := r.Q.QuerySelectAllStackAliasesWithID(ctx)
	return rows, repository.HandleDatabaseError(err)
}

func (r *aliasesRepository) QueryInsertStackAlias(ctx context.Context, args db.QueryInsertStackAliasParams) (db.StackAlias, *repository.RepositoryError) {
	row, err := r.Q.QueryInsertStackAlias(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *aliasesRepository) QueryDeleteStackAlias(ctx context.Context, id string) *repository.RepositoryError {
	uid, appErr := util.ParseUUID(id)
	if appErr != nil {
		return appErr
	}
	return repository.HandleDatabaseError(r.Q.QueryDeleteStackAlias(ctx, uid))
}
