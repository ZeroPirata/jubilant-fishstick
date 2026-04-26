package routes

import (
	"context"
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
	mux.Handle("/static/", http.StripPrefix("/static/", noCacheStatic(http.FileServer(http.Dir("static")))))
	outputFS := http.FileServer(http.Dir(handler.OutputBaseDir()))
	mux.Handle("/output/",
		middleware.RevocationMiddleware(revoker)(
			downloadAuth(jwtManager)(
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

func noCacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

// downloadAuth aceita JWT tanto no header Authorization quanto no query param ?token=
// para permitir downloads diretos via <a href> no browser.
func downloadAuth(jwt security.TokenProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tokenParam := r.URL.Query().Get("token"); tokenParam != "" {
				claims, err := jwt.Validate(tokenParam)
				if err != nil {
					http.Error(w, "invalid token", http.StatusUnauthorized)
					return
				}
				ctx := context.WithValue(r.Context(), middleware.ContextKeyUserID, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			middleware.AuthMiddleware(jwt)(next).ServeHTTP(w, r)
		})
	}
}
