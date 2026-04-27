package worker

import (
	"context"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository/aliases"
	"hackton-treino/internal/repository/feedbacks"
	"hackton-treino/internal/repository/users"
	workerRepo "hackton-treino/internal/repository/worker"
	"hackton-treino/internal/services"
	"hackton-treino/internal/sse"
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
	Logger     *zap.Logger
	Pipeline   workerRepo.Repository
	Users      users.Repository
	Feedbacks  feedbacks.Repository
	Aliases    aliases.Repository
	aliases    map[string]string
	promptPTBR string
	promptEN   string
	LLM        services.AiService
	ScraperAi  bool
	Cache      ucache.Cache
	Bus        *sse.Bus
}

func NewWorker(logger *zap.Logger, conn *pgxpool.Pool, llm services.AiService, scraperAi bool, rds *redis.Client, bus *sse.Bus) *Worker {
	return &Worker{
		Logger:    logger,
		LLM:       llm,
		Pipeline:  workerRepo.New(conn),
		Users:     users.New(conn),
		Feedbacks: feedbacks.New(conn),
		Aliases:   aliases.New(conn),
		ScraperAi: scraperAi,
		Cache:     ucache.New(rds),
		Bus:       bus,
	}
}

func (w *Worker) failJob(ctx context.Context, job *db.Job) {
	_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
	if w.Bus != nil {
		w.Bus.Publish(job.UserID.String(), sse.JobEvent{ID: job.ID.String(), Status: string(db.JobStatusError)})
	}
}

func (w *Worker) completeJob(ctx context.Context, job *db.Job, quality db.JobQuality) {
	_ = w.Pipeline.WorkerUpdateJobQuality(ctx, db.WorkerUpdateJobQualityParams{
		Quality: db.NullJobQuality{JobQuality: quality, Valid: true},
		ID:      job.ID,
	})
	_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusCompleted, ID: job.ID})
	if w.Bus != nil {
		w.Bus.Publish(job.UserID.String(), sse.JobEvent{
			ID:          job.ID.String(),
			Status:      string(db.JobStatusCompleted),
			Quality:     string(quality),
			CompanyName: job.CompanyName.String,
			JobTitle:    job.JobTitle.String,
		})
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

	if p, err := w.loadPrompt(promptPTBRPath); err != nil {
		w.Logger.Fatal("Falha ao carregar system prompt pt-BR", zap.Error(err))
	} else {
		w.promptPTBR = p
	}
	if p, err := w.loadPrompt(promptENPath); err != nil {
		w.Logger.Fatal("Falha ao carregar system prompt en", zap.Error(err))
	} else {
		w.promptEN = p
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
