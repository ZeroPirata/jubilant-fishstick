package main

import (
	"context"
	"errors"
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
	"time"

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

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- startServer(ctx, *cfg, db, rds, bus)
	}()

	llm := services.NewLLMService(cfg)
	go func() {
		worker.NewWorker(zap.L(), db, *llm, cfg.ScrapeAi.Activate, rds, bus, cfg.Worker).Start(ctx)
	}()

	<-ctx.Done()
	zap.L().Info("Encerrando aplicação graciosamente...")

	// Aguarda o servidor HTTP drenar as conexões em andamento (timeout de 15s dentro de startServer).
	if err := <-serverDone; err != nil {
		zap.L().Error("Servidor HTTP encerrou com erro", zap.Error(err))
	}
	return nil
}

func startServer(ctx context.Context, cfg config.Config, db *pgxpool.Pool, rds *redis.Client, bus *sse.Bus) error {
	port := ":8080"
	if cfg.Server.Port != "" {
		port = ":" + cfg.Server.Port
	}

	mux := http.NewServeMux()
	routes.Setup(mux, zap.L(), db, cfg, rds, bus)

	handler := middleware.MetricsMiddleware(
		middleware.SecurityHeaders(
			middleware.CORSMiddleware(cfg.Server.CORSAllowedOrigin)(
				middleware.LoggingMiddleware(zap.L(),
					middleware.BodyLimit(1<<20)(mux),
				),
			),
		),
	)
	handler = middleware.MiddlewarePanicRecovery(zap.L())(handler)

	server := &http.Server{
		Addr:    port,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			zap.L().Error("Erro no shutdown do servidor HTTP", zap.Error(err))
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
