package main

import (
	"context"
	"fmt"
	"hackton-treino/config"
	"hackton-treino/database/cache"
	"hackton-treino/database/postgres"
	"hackton-treino/internal/bootstrap"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/routes"
	"hackton-treino/internal/services"
	"hackton-treino/internal/sse"
	"hackton-treino/internal/worker"
	dbmigrations "hackton-treino/repository/migrations"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		log.Fatal("failed to run the system", zap.Error(err))
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger, flushLog, err := bootstrap.InitZapLog(cfg)
	if err != nil {
		log.Fatalf("failed to init log: %v", err)
	}
	defer flushLog()

	zap.ReplaceGlobals(logger)

	zap.L().Info("Aplicação iniciada com sucesso",
		zap.String("version", cfg.Project.Version),
		zap.String("env", cfg.Project.Name),
		zap.String("hostname", cfg.Server.Host),
		zap.String("port", cfg.Server.Port),
	)

	db, err := postgres.GetDataBasePool(cfg, zap.L())
	if err != nil {
		return fmt.Errorf("failed to get database pool: %w", err)
	}
	defer postgres.ClosePool(zap.L())

	if err := postgres.RunMigrations(dbmigrations.FS, cfg, zap.L()); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	rds, err := cache.GetClient(cfg, zap.L())
	if err != nil {
		return fmt.Errorf("failed to get cache pool: %w", err)
	}
	defer cache.Close(zap.L())

	bus := sse.NewBus()

	go func() {
		if err := startServer(*cfg, db, rds, bus); err != nil {
			zap.L().Fatal("Erro fatal no servidor HTTP", zap.Error(err))
		}
	}()

	llm := services.NewLLMService(cfg)
	go func() {
		worker.NewWorker(zap.L(), db, *llm, cfg.ScrapeAi.Activate, rds, bus).Start(ctx)
	}()

	<-ctx.Done()
	zap.L().Info("Encerrando aplicação graciosamente...")
	return nil
}

func startServer(cfg config.Config, db *pgxpool.Pool, rds *redis.Client, bus *sse.Bus) error {
	var port string
	mux := http.NewServeMux()

	if cfg.Server.Port != "" {
		port = ":" + cfg.Server.Port
	} else {
		port = ":8080"
	}

	routes.Setup(mux, zap.L(), db, cfg, rds, bus)
	handler := middleware.SecurityHeaders(
		middleware.CORSMiddleware(cfg.Server.CORSAllowedOrigin)(
			middleware.LoggingMiddleware(zap.L(),
				middleware.BodyLimit(1<<20)(mux), // 1 MB
			),
		),
	)
	handler = middleware.MiddlewarePanicRecovery(zap.L())(handler)

	server := &http.Server{
		Addr:    port,
		Handler: handler,
	}
	return server.ListenAndServe()
}
