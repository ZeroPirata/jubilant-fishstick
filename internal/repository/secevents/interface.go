package secevents

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

type seceventRepository struct {
	repository.Base
}

type InsertParams struct {
	EventType db.SecurityEventType
	IP        string
	UserID    string // empty = unknown
	Metadata  []byte
}

type Repository interface {
	Insert(ctx context.Context, p InsertParams)
	List(ctx context.Context, days int) ([]db.SecurityEvent, *repository.RepositoryError)
}

func New(conn *pgxpool.Pool) Repository {
	return &seceventRepository{Base: repository.NewBase(conn)}
}
