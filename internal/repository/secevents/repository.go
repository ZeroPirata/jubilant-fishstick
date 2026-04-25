package secevents

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func (r *seceventRepository) Insert(ctx context.Context, p InsertParams) {
	ip := pgtype.Text{}
	if p.IP != "" {
		ip = pgtype.Text{String: p.IP, Valid: true}
	}

	userID := pgtype.UUID{}
	if p.UserID != "" {
		_ = userID.Scan(p.UserID)
	}

	metadata := p.Metadata
	if len(metadata) == 0 {
		metadata = []byte("{}")
	}

	if err := r.Q.QueryInsertSecurityEvent(ctx, db.QueryInsertSecurityEventParams{
		EventType: p.EventType,
		Ip:        ip,
		UserID:    userID,
		Metadata:  metadata,
	}); err != nil {
		zap.L().Warn("secevents: falha ao inserir evento", zap.Error(err))
	}
}

func (r *seceventRepository) List(ctx context.Context, days int) ([]db.SecurityEvent, *repository.RepositoryError) {
	since := pgtype.Timestamptz{
		Time:  time.Now().AddDate(0, 0, -days),
		Valid: true,
	}
	rows, err := r.Q.QueryListSecurityEvents(ctx, since)
	return rows, repository.HandleDatabaseError(err)
}
