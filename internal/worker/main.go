package worker

import (
	"context"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/services"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	promptENPath   = "internal/worker/prompts/system_en.txt"
	promptPTBRPath = "internal/worker/prompts/system_ptbr.txt"
)

type Worker struct {
	Logger     *zap.Logger
	Repository repository.Repository
	filtros    []string
	inf        db.InformacoesBasica
	LLM        services.AiService
}

func NewWorker(logger *zap.Logger, repo *pgxpool.Pool, llm services.AiService) *Worker {
	r := repository.NewRepository(repo)

	return &Worker{
		Logger:     logger,
		LLM:        llm,
		Repository: r,
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

	filtros, err := w.Repository.QuerySelectAllFiltros(ctx)
	if err != nil {
		w.Logger.Error("Erro ao buscar filtros", zap.Error(err))
	}
	if filtros != nil {
		w.filtros = filtros
	}

	for {
		select {
		case <-ctx.Done():
			w.Logger.Info("Worker context cancelled")
			return
		case <-ticker.C:
			if info, err := w.Repository.QuerySelectBasicInfo(ctx); err == nil {
				w.inf = info
			}
			w.processarPendentes(ctx)
		}
	}
}
