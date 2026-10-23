package aisettings

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProviderRow struct {
	ID        string
	Purpose   string
	Position  int
	Provider  string
	URL       string
	Model     string
	APIKeyEnc []byte
	Enabled   bool
	UpdatedAt time.Time
}

type Repository interface {
	List(ctx context.Context, purpose string) ([]ProviderRow, error)
	// Replace troca a cadeia inteira do purpose numa transação, na ordem recebida.
	Replace(ctx context.Context, purpose string, rows []ProviderRow, updatedBy string) error
}

type repo struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) Repository {
	return &repo{db: db}
}

func (r *repo) List(ctx context.Context, purpose string) ([]ProviderRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, purpose, position, provider, url, model, api_key_enc, enabled, updated_at
		 FROM ai_providers
		 WHERE purpose = $1
		 ORDER BY position`,
		purpose,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ProviderRow
	for rows.Next() {
		var p ProviderRow
		if err := rows.Scan(&p.ID, &p.Purpose, &p.Position, &p.Provider, &p.URL, &p.Model, &p.APIKeyEnc, &p.Enabled, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *repo) Replace(ctx context.Context, purpose string, rows []ProviderRow, updatedBy string) error {
	var by pgtype.UUID
	if err := by.Scan(updatedBy); err != nil {
		return err
	}

	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM ai_providers WHERE purpose = $1`, purpose); err != nil {
			return err
		}
		for i, p := range rows {
			var id pgtype.UUID
			if p.ID != "" {
				if err := id.Scan(p.ID); err != nil {
					return err
				}
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO ai_providers (id, purpose, position, provider, url, model, api_key_enc, enabled, updated_by)
				 VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6, $7, $8, $9)`,
				id, purpose, i, p.Provider, p.URL, p.Model, p.APIKeyEnc, p.Enabled, by,
			)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
