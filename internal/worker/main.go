package worker

import (
	"context"
	"fmt"
	"hackton-treino/config"
	"hackton-treino/internal/db"
	"hackton-treino/internal/metrics"
	"hackton-treino/internal/repository/aliases"
	"hackton-treino/internal/repository/feedbacks"
	"hackton-treino/internal/repository/users"
	workerRepo "hackton-treino/internal/repository/worker"
	"hackton-treino/internal/services"
	"hackton-treino/internal/sse"
	"os"
	"time"

	ucache "hackton-treino/internal/repository/cache"
	adminRepo "hackton-treino/internal/repository/admin"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	promptENPath   = "internal/worker/prompts/system_en.txt"
	promptPTBRPath = "internal/worker/prompts/system_ptbr.txt"
)

type Worker struct {
	Logger        *zap.Logger
	Pipeline      workerRepo.Repository
	Users         users.Repository
	Feedbacks     feedbacks.Repository
	Aliases       aliases.Repository
	AdminRepo     adminRepo.Repository
	aliases       map[string]string
	promptPTBR    string
	promptEN      string
	LLM           services.AiService
	ScraperAi     bool
	Cache         ucache.Cache
	Bus           *sse.Bus
	maxConcurrent   int
	batchSize       int32
	interval        time.Duration
	recoveryTimeout time.Duration
}

func NewWorker(logger *zap.Logger, conn *pgxpool.Pool, llm services.AiService, scraperAi bool, rds *redis.Client, bus *sse.Bus, cfg config.WorkerConfig) *Worker {
	return &Worker{
		Logger:        logger,
		LLM:           llm,
		Pipeline:      workerRepo.New(conn),
		Users:         users.New(conn),
		Feedbacks:     feedbacks.New(conn),
		Aliases:       aliases.New(conn),
		AdminRepo:     adminRepo.New(conn),
		ScraperAi:     scraperAi,
		Cache:         ucache.New(rds),
		Bus:           bus,
		maxConcurrent:   cfg.MaxConcurrent,
		batchSize:       int32(cfg.BatchSize),
		interval:        cfg.Interval,
		recoveryTimeout: cfg.RecoveryTimeout,
	}
}

func (w *Worker) failJob(ctx context.Context, job *db.Job) {
	_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
	if w.Bus != nil {
		w.Bus.Publish(job.UserID.String(), sse.JobEvent{ID: job.ID.String(), Status: string(db.JobStatusError)})
	}
	metrics.JobsProcessed.WithLabelValues("error").Inc()
}

func (w *Worker) completeJob(ctx context.Context, job *db.Job, quality db.JobQuality, gap sse.GapAnalysis) {
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
			Gap:         &gap,
		})
	}
	metrics.JobsProcessed.WithLabelValues("completed").Inc()
}

func (w *Worker) recoverStuckJobs(ctx context.Context) {
	if w.recoveryTimeout == 0 {
		return
	}
	cutoff := pgtype.Timestamptz{Time: time.Now().Add(-w.recoveryTimeout), Valid: true}
	n, err := w.Pipeline.WorkerRecoverStuckJobs(ctx, cutoff)
	if err != nil {
		w.Logger.Error("Erro ao recuperar jobs travados", zap.Error(err))
		return
	}
	if n > 0 {
		w.Logger.Warn("Jobs travados recuperados para pending", zap.Int64("count", n), zap.Duration("timeout", w.recoveryTimeout))
		metrics.JobsProcessed.WithLabelValues("recovered").Add(float64(n))
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

	ticker := time.NewTicker(w.interval)
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
			w.recoverStuckJobs(ctx)
			w.processarPendentes(ctx)
		}
	}
}
