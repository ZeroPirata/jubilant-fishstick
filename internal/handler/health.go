package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Healthz: GET /healthz, público e sem autenticação. 200 quando o Postgres e o Redis
// respondem; 503 dizendo qual falhou. Usado pelo healthcheck do container e pelo
// smoke test do deploy (antes era um login com conta inexistente, que sujava o log de
// erros, os eventos de segurança e o bloqueio por tentativas).
func Healthz(db *pgxpool.Pool, rds *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		checks := map[string]string{"database": "ok", "cache": "ok"}
		status := http.StatusOK
		if err := db.Ping(ctx); err != nil {
			checks["database"], status = "erro", http.StatusServiceUnavailable
		}
		if err := rds.Ping(ctx).Err(); err != nil {
			checks["cache"], status = "erro", http.StatusServiceUnavailable
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, status, checks)
	}
}
