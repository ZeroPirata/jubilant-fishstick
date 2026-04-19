package routes

import (
	"hackton-treino/internal/handler"
	"hackton-treino/internal/security"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type AuthRoutes struct {
	Mux    *http.ServeMux
	Logger *zap.Logger
	Db     *pgxpool.Pool
	Hasher *security.Hasher
	Jwt    security.TokenProvider
}

func setupAuthRoutes(args AuthRoutes) {
	h := handler.NewAuthHandler(args.Logger, args.Db, args.Hasher, args.Jwt)

	args.Mux.HandleFunc("POST /auth/register", h.Register)
	args.Mux.HandleFunc("POST /auth/login", h.Login)
}
