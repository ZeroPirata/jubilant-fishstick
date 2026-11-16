// Package linkedin guarda as buscas salvas no LinkedIn de cada usuário.
package linkedin

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	List(ctx context.Context, userID pgtype.UUID) ([]db.UserLinkedinSearch, *repository.RepositoryError)
	Count(ctx context.Context, userID pgtype.UUID) (int64, *repository.RepositoryError)
	Insert(ctx context.Context, args db.LinkedInInsertSearchParams) (db.UserLinkedinSearch, *repository.RepositoryError)
	Update(ctx context.Context, args db.LinkedInUpdateSearchParams) (db.UserLinkedinSearch, *repository.RepositoryError)
	// Delete devolve false quando a busca não existe ou é de outro usuário.
	Delete(ctx context.Context, args db.LinkedInDeleteSearchParams) (bool, *repository.RepositoryError)
}

type linkedinRepository struct {
	repository.Base
}

func New(conn *pgxpool.Pool) Repository {
	return &linkedinRepository{Base: repository.NewBase(conn)}
}

func (r *linkedinRepository) List(ctx context.Context, userID pgtype.UUID) ([]db.UserLinkedinSearch, *repository.RepositoryError) {
	rows, err := r.Q.LinkedInListSearches(ctx, userID)
	return rows, repository.HandleDatabaseError(err)
}

func (r *linkedinRepository) Count(ctx context.Context, userID pgtype.UUID) (int64, *repository.RepositoryError) {
	n, err := r.Q.LinkedInCountSearches(ctx, userID)
	return n, repository.HandleDatabaseError(err)
}

func (r *linkedinRepository) Insert(ctx context.Context, args db.LinkedInInsertSearchParams) (db.UserLinkedinSearch, *repository.RepositoryError) {
	row, err := r.Q.LinkedInInsertSearch(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *linkedinRepository) Update(ctx context.Context, args db.LinkedInUpdateSearchParams) (db.UserLinkedinSearch, *repository.RepositoryError) {
	row, err := r.Q.LinkedInUpdateSearch(ctx, args)
	return row, repository.HandleDatabaseError(err)
}

func (r *linkedinRepository) Delete(ctx context.Context, args db.LinkedInDeleteSearchParams) (bool, *repository.RepositoryError) {
	n, err := r.Q.LinkedInDeleteSearch(ctx, args)
	return n > 0, repository.HandleDatabaseError(err)
}
