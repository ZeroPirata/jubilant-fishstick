package routes

import (
	"hackton-treino/internal/db"
	"hackton-treino/internal/handler"
	"hackton-treino/internal/lockout"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository/secevents"
	"hackton-treino/internal/security"
	"hackton-treino/internal/util"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type AuthRoutes struct {
	Mux    *http.ServeMux
	Logger *zap.Logger
	Db     *pgxpool.Pool
	Hasher *security.Hasher
	Jwt    security.TokenProvider
	SecLog secevents.Repository
	Rds    *redis.Client
}

func setupAuthRoutes(args AuthRoutes) {
	locker := lockout.New(args.Rds)
	h := handler.NewAuthHandler(args.Logger, args.Db, args.Hasher, args.Jwt, args.SecLog, locker)
	// 10 tentativas por minuto por IP — suficiente para uso normal, bloqueia brute force.
	limiter := middleware.NewRateLimiter(10, time.Minute).
		OnBlocked(func(r *http.Request) {
			args.SecLog.Insert(r.Context(), secevents.InsertParams{
				EventType: db.SecurityEventTypeRateLimited,
				IP:        util.ClientIP(r),
				Metadata:  []byte(`{"path":"` + r.URL.Path + `"}`),
			})
		})

	args.Mux.Handle("POST /auth/register", limiter.Middleware(http.HandlerFunc(h.Register)))
	args.Mux.Handle("POST /auth/login", limiter.Middleware(http.HandlerFunc(h.Login)))
}
