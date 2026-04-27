package admin

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ErrorLog struct {
	ID           string
	JobID        *string
	UserEmail    *string
	ErrorType    string
	ErrorMessage string
	URL          *string
	CreatedAt    time.Time
}

type UserAccount struct {
	ID        string
	Email     string
	IsAdmin   bool
	CreatedAt time.Time
}

type InsertErrorLogParams struct {
	JobID        *string
	UserID       *string
	ErrorType    string
	ErrorMessage string
	URL          *string
}

type Repository interface {
	InsertErrorLog(ctx context.Context, p InsertErrorLogParams)
	ListErrorLogs(ctx context.Context, limit, offset int) ([]ErrorLog, int, error)
	ListUsers(ctx context.Context) ([]UserAccount, error)
	SetAdmin(ctx context.Context, userID string, isAdmin bool) error
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

type adminRepo struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) Repository {
	return &adminRepo{db: db}
}
