package worker

import (
	"context"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/scraper"
	"hackton-treino/internal/util"

	"go.uber.org/zap"
)

func (w *Worker) doScraper(ctx context.Context, job *db.Job) (*scraper.ResultScraper, error) {
	result := &scraper.ResultScraper{}

	if err := w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{
		Status: db.JobStatusScrapingBasic,
		ID:     job.ID,
	}); err != nil {
		w.Logger.Warn("Erro ao atualizar status para scraping_basic", zap.String("job_id", job.ID.String()), zap.Error(err))
	}

	newScraper := scraper.NewScraper(job.ExternalUrl, w.Logger)
	basicScraper, err := newScraper.Scrape()
	if err != nil {
		w.Logger.Error("doScraper: Erro ao fazer scrape da vaga", zap.String("job_id", job.ID.String()), zap.Error(err))
		return nil, fmt.Errorf("não foi possivel realizar o scraper: %w", err)
	}

	if err := w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{
		Status: db.JobStatusScrapingNl,
		ID:     job.ID,
	}); err != nil {
		w.Logger.Warn("Erro ao atualizar status para scraping_nl", zap.String("job_id", job.ID.String()), zap.Error(err))
	}

	nlScraper, err := w.LLM.GenerateScrapeSite(ctx, basicScraper.BasicDescription)
	if err != nil {
		w.Logger.Error("doScraper: Erro ao enriquecer vaga com A.I", zap.String("job_id", job.ID.String()), zap.Error(err))
		return nil, fmt.Errorf("não foi possivel realizar o scraper com A.I: %w", err)
	}

	result.Company = basicScraper.Company
	result.Title = basicScraper.Title
	result.BasicDescription = basicScraper.BasicDescription
	result.CompressedDescription = nlScraper.CompressedDescription
	result.Stack = util.NormalizeStack(nlScraper.Stack, w.aliases)
	result.Requirements = nlScraper.Requirements
	return result, nil
}
