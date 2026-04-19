package repository

import (
	"context"
	"fmt"
	"hackton-treino/internal/db"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Base struct {
	Conn *pgxpool.Pool
	Q    *db.Queries
}

func NewBase(conn *pgxpool.Pool) Base {
	return Base{Conn: conn, Q: db.New(conn)}
}

func (r *Base) ExecTx(ctx context.Context, fn func(*db.Queries) error) *RepositoryError {
	tx, err := r.Conn.Begin(ctx)
	if err != nil {
		return HandleDatabaseError(err)
	}
	qtx := r.Q.WithTx(tx)
	err = fn(qtx)
	if err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return &RepositoryError{StatusCode: http.StatusInternalServerError, Message: fmt.Sprintf("tx err: %v, rb err: %v", err, rbErr)}
		}
		return HandleDatabaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return HandleDatabaseError(err)
	}
	return nil
}
