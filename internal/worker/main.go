package worker

import (
	"context"
	"fmt"
	"hackton-treino/internal/repository/aliases"
	"hackton-treino/internal/repository/feedbacks"
	"hackton-treino/internal/repository/filters"
	"hackton-treino/internal/repository/users"
	workerRepo "hackton-treino/internal/repository/worker"
	"hackton-treino/internal/services"
	"os"
	"time"

	ucache "hackton-treino/internal/repository/cache"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	promptENPath   = "internal/worker/prompts/system_en.txt"
	promptPTBRPath = "internal/worker/prompts/system_ptbr.txt"
)

type Worker struct {
	Logger    *zap.Logger
	Pipeline  workerRepo.Repository
	Users     users.Repository
	Filters   filters.Repository
	Feedbacks feedbacks.Repository
	Aliases   aliases.Repository
	aliases   map[string]string
	LLM       services.AiService
	ScraperAi bool
	Cache     ucache.Cache
}

func NewWorker(logger *zap.Logger, conn *pgxpool.Pool, llm services.AiService, scraperAi bool, rds *redis.Client) *Worker {
	return &Worker{
		Logger:    logger,
		LLM:       llm,
		Pipeline:  workerRepo.New(conn),
		Users:     users.New(conn),
		Filters:   filters.New(conn),
		Feedbacks: feedbacks.New(conn),
		Aliases:   aliases.New(conn),
		ScraperAi: scraperAi,
		Cache:     ucache.New(rds),
	}
}

func (w *Worker) loadPrompt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("load prompt %s: %w", path, err)
	}
	return string(data), nil
}

func (w *Worker) Start(ctx context.Context) {
	w.Logger.Info("Worker started")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	if aliasMap, err := w.Aliases.QuerySelectAllStackAliases(ctx); err == nil && aliasMap != nil {
		w.aliases = aliasMap
	} else if err != nil {
		w.Logger.Error("Erro ao buscar stack aliases", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			w.Logger.Info("Worker context cancelled")
			return
		case <-ticker.C:
			if aliasMap, err := w.Aliases.QuerySelectAllStackAliases(ctx); err == nil && aliasMap != nil {
				w.aliases = aliasMap
			}
			w.processarPendentes(ctx)
		}
	}
}
