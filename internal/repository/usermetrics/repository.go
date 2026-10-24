// Package usermetrics registra o último acesso dos usuários e agrega os números de uso.
package usermetrics

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Stats struct {
	Total       int `json:"total"`
	New7d       int `json:"new_7d"`
	Active24h   int `json:"active_24h"`
	Active7d    int `json:"active_7d"`
	Active30d   int `json:"active_30d"`
	WithResumes int `json:"with_resumes"` // geraram ao menos um currículo
	NeverSeen   int `json:"never_seen"`   // sem acesso registrado (antes do last_seen_at existir)
}

type Repository struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// TouchLastSeen implementa middleware.ActivityToucher.
func (r *Repository) TouchLastSeen(ctx context.Context, userID string) error {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `UPDATE user_accounts SET last_seen_at = NOW() WHERE id = $1`, uid)
	return err
}

func (r *Repository) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE created_at   >= NOW() - INTERVAL '7 days'),
		       COUNT(*) FILTER (WHERE last_seen_at >= NOW() - INTERVAL '24 hours'),
		       COUNT(*) FILTER (WHERE last_seen_at >= NOW() - INTERVAL '7 days'),
		       COUNT(*) FILTER (WHERE last_seen_at >= NOW() - INTERVAL '30 days'),
		       COUNT(*) FILTER (WHERE last_seen_at IS NULL),
		       (SELECT COUNT(DISTINCT gr.user_id) FROM generated_resumes gr
		         JOIN user_accounts u ON u.id = gr.user_id
		        WHERE gr.deleted_at IS NULL AND u.deleted_at IS NULL)
		FROM user_accounts
		WHERE deleted_at IS NULL`,
	).Scan(&s.Total, &s.New7d, &s.Active24h, &s.Active7d, &s.Active30d, &s.NeverSeen, &s.WithResumes)
	return s, err
}
