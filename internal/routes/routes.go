package routes

import (
	"hackton-treino/config"
	"hackton-treino/internal/handler"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/security"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func Setup(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool, cfg config.Config, rds *redis.Client) {
	timeout := config.GetConfigDurationOrDefault(cfg.Project.ContextTimeout, 60*time.Second)
	hash := config.LoadHashConfig(cfg)
	hasher := security.NewHasher(hash)
	jwtManager := security.NewJwtManager(cfg.Jwt.Secret, cfg.Jwt.Expiration)

	mux.HandleFunc("/", handler.ServeUI)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	outputFS := http.FileServer(http.Dir("output"))
	mux.Handle("/output/", http.StripPrefix("/output/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", "attachment")
		outputFS.ServeHTTP(w, r)
	})))

	apiMux := http.NewServeMux()
	setupAuthRoutes(AuthRoutes{Mux: apiMux, Logger: logger, Db: db, Hasher: hasher, Jwt: jwtManager})

	protectedMux := http.NewServeMux()
	setupJobRoutes(protectedMux, logger, db)
	setupUserRoutes(protectedMux, logger, db, rds)
	setupFilterRoutes(protectedMux, logger, db)

	protectedHandler := middleware.AuthMiddleware(jwtManager)(protectedMux)
	apiMux.Handle("/", protectedHandler)

	apiHandler := middleware.TimeoutMiddleware(timeout)(apiMux)
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiHandler))
}
