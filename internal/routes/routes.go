package routes

import (
	"context"
	"encoding/json"
	"hackton-treino/config"
	"hackton-treino/internal/aiconfig"
	"hackton-treino/internal/handler"
	"hackton-treino/internal/middleware"
	adminRepo "hackton-treino/internal/repository/admin"
	"hackton-treino/internal/repository/aimetrics"
	"hackton-treino/internal/repository/usermetrics"
	ucache "hackton-treino/internal/repository/cache"
	"hackton-treino/internal/repository/secevents"
	"hackton-treino/internal/security"
	"hackton-treino/internal/services"
	"hackton-treino/internal/sse"
	"net/http"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func Setup(mux *http.ServeMux, logger *zap.Logger, db *pgxpool.Pool, cfg config.Config, rds *redis.Client, bus *sse.Bus, aiStore *aiconfig.Store, llm *services.AiService) {
	timeout := config.GetConfigDurationOrDefault(cfg.Project.ContextTimeout, 60*time.Second)
	hash := config.LoadHashConfig(cfg)
	hasher := security.NewHasher(hash)
	jwtManager := security.NewJwtManager(cfg.Jwt.Secret, cfg.Jwt.Expiration)
	revoker := security.NewRevoker(rds)
	secLog := secevents.New(db)
	adminR := adminRepo.New(db)
	adminH := handler.NewAdminHandler(logger, adminR)

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
	setupProfileImportRoutes(protectedMux, logger, db, rds, llm)
	setupFilterRoutes(protectedMux, logger, db)
	setupSecurityRoutes(protectedMux, logger, secLog)

	logoutHandler := handler.NewLogoutHandler(logger, revoker)
	protectedMux.Handle("POST /auth/logout", http.HandlerFunc(logoutHandler.Logout))

	// /me — retorna is_admin do usuário autenticado
	protectedMux.HandleFunc("GET /me", adminH.Me)

	// Rotas de administrador
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /errors",           adminH.ListErrorLogs)
	adminMux.HandleFunc("GET /users",            adminH.ListUsers)
	adminMux.HandleFunc("PATCH /users/{id}",     adminH.SetAdmin)

	// Métricas (admin): Prometheus bruto (antes público em /metrics) e histórico das chamadas de IA
	userMetrics := usermetrics.New(db)
	metricsH := handler.NewMetricsHandler(logger, aimetrics.New(db), userMetrics)
	adminMux.Handle("GET /metrics/raw", promhttp.Handler())
	adminMux.HandleFunc("GET /metrics/ai", metricsH.AI)
	adminMux.HandleFunc("GET /metrics/users", metricsH.UserStats)

	// Só master: configurações de IA (contêm API keys) e promoção de masters
	master := middleware.MasterMiddleware(adminR)
	aiH := handler.NewAISettingsHandler(logger, aiStore, ucache.New(rds), adminR, cfg)
	adminMux.Handle("GET /ai-settings",             master(http.HandlerFunc(aiH.Get)))
	adminMux.Handle("PUT /ai-settings/{purpose}",   master(http.HandlerFunc(aiH.Save)))
	adminMux.Handle("POST /ai-settings/resume",     master(http.HandlerFunc(aiH.ResumeWorker)))
	adminMux.Handle("PATCH /users/{id}/master",     master(http.HandlerFunc(aiH.SetMaster)))
	adminHandler := middleware.AdminMiddleware(adminR)(adminMux)
	protectedMux.Handle("/admin/", http.StripPrefix("/admin", adminHandler))

	protectedMux.HandleFunc("POST /debug/gc", func(w http.ResponseWriter, r *http.Request) {
		var before runtime.MemStats
		runtime.ReadMemStats(&before)
		runtime.GC()
		debug.FreeOSMemory()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"heap_before": before.HeapAlloc,
			"heap_after":  after.HeapAlloc,
			"freed_bytes": int64(before.HeapAlloc) - int64(after.HeapAlloc),
		})
	})

	// ActivityMiddleware depois do Auth: precisa do usuário no contexto para registrar o último acesso.
	activity := middleware.ActivityMiddleware(userMetrics, 5*time.Minute)
	protectedHandler := middleware.RevocationMiddleware(revoker)(middleware.AuthMiddleware(jwtManager)(activity(protectedMux)))
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
