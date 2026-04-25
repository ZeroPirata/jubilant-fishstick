package routes

import (
	"hackton-treino/config"
	"hackton-treino/internal/handler"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository/secevents"
	"hackton-treino/internal/security"
	"hackton-treino/internal/sse"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func Setup(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool, cfg config.Config, rds *redis.Client, bus *sse.Bus) {
	timeout := config.GetConfigDurationOrDefault(cfg.Project.ContextTimeout, 60*time.Second)
	hash := config.LoadHashConfig(cfg)
	hasher := security.NewHasher(hash)
	jwtManager := security.NewJwtManager(cfg.Jwt.Secret, cfg.Jwt.Expiration)
	revoker := security.NewRevoker(rds)
	secLog := secevents.New(db)

	mux.HandleFunc("/", handler.ServeUI)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	outputFS := http.FileServer(http.Dir(handler.OutputBaseDir()))
	mux.Handle("/output/",
		middleware.RevocationMiddleware(revoker)(
			middleware.AuthMiddleware(jwtManager)(
				http.StripPrefix("/output/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// First path segment after /output/ is the owner UUID.
					ownerID := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)[0]
					userID, ok := middleware.GetUserID(r.Context())
					if !ok || ownerID != userID.String() {
						http.Error(w, "forbidden", http.StatusForbidden)
						return
					}
					w.Header().Set("Content-Disposition", "attachment")
					outputFS.ServeHTTP(w, r)
				})),
			),
		),
	)

	apiMux := http.NewServeMux()
	setupAuthRoutes(AuthRoutes{Mux: apiMux, Logger: logger, Db: db, Hasher: hasher, Jwt: jwtManager, SecLog: secLog, Rds: rds})

	protectedMux := http.NewServeMux()
	jobHandler := setupJobRoutes(protectedMux, logger, db, bus, secLog)
	setupUserRoutes(protectedMux, logger, db, rds)
	setupFilterRoutes(protectedMux, logger, db)
	setupSecurityRoutes(protectedMux, logger, secLog)

	logoutHandler := handler.NewLogoutHandler(logger, revoker)
	protectedMux.Handle("POST /auth/logout", http.HandlerFunc(logoutHandler.Logout))

	protectedHandler := middleware.RevocationMiddleware(revoker)(middleware.AuthMiddleware(jwtManager)(protectedMux))
	apiMux.Handle("/", protectedHandler)

	apiHandler := middleware.TimeoutMiddleware(timeout)(apiMux)
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiHandler))

	// SSE endpoint bypasses TimeoutMiddleware — long-lived connection, exact match wins over /api/v1/ prefix
	mux.Handle("GET /api/v1/jobs/events",
		middleware.RevocationMiddleware(revoker)(
			middleware.AuthMiddleware(jwtManager)(http.HandlerFunc(jobHandler.StreamEvents)),
		),
	)
}
