package main

import (
	"context"
	"fmt"
	"hackton-treino/config"
	"hackton-treino/database/postgres"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/routes"
	"hackton-treino/internal/services"
	"hackton-treino/internal/worker"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.Project.LoggerFolder != "" {
		if err := os.MkdirAll(cfg.Project.LoggerFolder, 0755); err != nil {
			return fmt.Errorf("could not create log directory: %w", err)
		}
	}

	loggerConfig := zapConfigFromProjectConfig(*cfg)
	logger, err := loggerConfig.Build()
	if err != nil {
		return err
	}

	logger = logger.With(
		zap.String("service", cfg.Project.Name),
		zap.String("env", cfg.Project.Name),
		zap.String("version", cfg.Project.Version),
	)

	defer func() {
		if err := logger.Sync(); err != nil {
			if !strings.Contains(err.Error(), "stdout") && !strings.Contains(err.Error(), "stderr") {
				fmt.Printf("Error syncing logger: %v\n", err)
			}
		}
	}()

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

	llm := services.NewAiService(cfg)

	go func() {
		if err := startServer(*cfg, db); err != nil {
			zap.L().Fatal("Erro fatal no servidor HTTP", zap.Error(err))
		}
	}()

	go func() {
		worker.NewWorker(zap.L(), db, *llm).Start(ctx)
	}()

	<-ctx.Done()
	zap.L().Info("Encerrando aplicação graciosamente...")
	return nil
}

func zapConfigFromProjectConfig(cfg config.Config) zap.Config {
	var zapConfig zap.Config

	if cfg.Project.Debug {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
	}

	zapConfig.OutputPaths = []string{"stdout"}
	zapConfig.ErrorOutputPaths = []string{"stderr"}

	if !cfg.Project.Debug && cfg.Project.LoggerFolder != "" {
		zapConfig.OutputPaths = append(
			zapConfig.OutputPaths,
			cfg.Project.LoggerFolder+"/"+cfg.Project.Name+".log",
		)
		zapConfig.ErrorOutputPaths = append(
			zapConfig.ErrorOutputPaths,
			cfg.Project.LoggerFolder+"/"+cfg.Project.Name+"_error.log",
		)
	}

	enc := zapConfig.EncoderConfig

	enc.EncodeLevel = zapcore.LowercaseLevelEncoder
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	enc.EncodeCaller = zapcore.ShortCallerEncoder
	enc.EncodeDuration = zapcore.StringDurationEncoder

	zapConfig.EncoderConfig = enc

	zapConfig.DisableStacktrace = true

	if cfg.Project.Debug {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	return zapConfig
}

func startServer(config config.Config, db *pgxpool.Pool) error {
	var port string
	mux := http.NewServeMux()

	if config.Server.Port != "" {
		port = ":" + config.Server.Port
	} else {
		port = ":8080"
	}

	routes.PipeCurriculoStup(mux, zap.L(), db)
	handler := middleware.CORSMiddleware(middleware.LoggingMiddleware(zap.L(), mux))

	server := &http.Server{
		Addr:    port,
		Handler: handler,
	}
	return server.ListenAndServe()
}
