package worker

import (
	"context"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/metrics"
	"hackton-treino/internal/scraper"
	"hackton-treino/internal/util"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (w *Worker) doScraper(ctx context.Context, job *db.Job) (*scraper.ResultScraper, error) {
	start := time.Now()
	result := &scraper.ResultScraper{}

	var basicDescription string

	if err := w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{
		Status: db.JobStatusScrapingBasic,
		ID:     job.ID,
	}); err != nil {
		w.Logger.Warn("Erro ao atualizar status para scraping_basic", zap.String("job_id", job.ID.String()), zap.Error(err))
	}

	// Descrição colada (ou editada) pelo usuário: é a vaga. Não abre o site, que pode nem
	// ser legível (é para isso que o campo existe). Não é job.Description: essa o worker
	// sobrescreve com o resumo da IA.
	var basicScraper scraper.BasicScraperResult
	var err error
	if manual := strings.TrimSpace(job.SourceDescription.String); job.SourceDescription.Valid && manual != "" {
		w.Logger.Info("doScraper: usando a descrição colada no cadastro", zap.String("job_id", job.ID.String()))
		basicScraper = scraper.BasicScraperResult{Title: job.JobTitle.String, Company: job.CompanyName.String, BasicDescription: manual}
	} else {
		newScraper := scraper.NewScraper(job.ExternalUrl, w.Logger)
		basicScraper, err = newScraper.Scrape()
	}
	if err == nil && strings.TrimSpace(basicScraper.BasicDescription) == "" {
		err = scraper.ErrEmptyDescription
	}
	if err != nil {
		// Vaga vinda do Descobrir: o catálogo já tem a descrição. Cobre os sites sem
		// leitor próprio (GitHub, Himalayas) e os que bloqueiam acesso automatizado.
		fromCatalog, ok := w.catalogDescription(ctx, job.ExternalUrl)
		if !ok {
			w.Logger.Error("doScraper: Erro ao fazer scrape da vaga", zap.String("job_id", job.ID.String()), zap.String("url", job.ExternalUrl), zap.Error(err))
			return nil, fmt.Errorf("não foi possivel realizar o scraper: %w", err)
		}
		w.Logger.Info("doScraper: usando a descrição do catálogo de vagas", zap.String("job_id", job.ID.String()), zap.NamedError("scrape", err))
		basicScraper = fromCatalog
	}

	basicDescription = basicScraper.BasicDescription
	result.Company = basicScraper.Company
	result.Title = basicScraper.Title
	result.BasicDescription = basicScraper.BasicDescription

	if err := w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{
		Status: db.JobStatusScrapingNl,
		ID:     job.ID,
	}); err != nil {
		w.Logger.Warn("Erro ao atualizar status para scraping_nl", zap.String("job_id", job.ID.String()), zap.Error(err))
	}

	nlScraper, err := w.LLM.GenerateScrapeSite(ctx, basicDescription)
	if err != nil {
		metrics.ScraperDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())
		w.Logger.Error("doScraper: Erro ao enriquecer vaga com A.I", zap.String("job_id", job.ID.String()), zap.Error(err))
		return nil, fmt.Errorf("não foi possivel realizar o scraper com A.I: %w", err)
	}

	metrics.ScraperDuration.WithLabelValues("ok").Observe(time.Since(start).Seconds())
	result.BasicDescription = basicDescription
	result.CompressedDescription = nlScraper.CompressedDescription
	result.Stack = util.NormalizeStack(nlScraper.Stack, w.aliases)
	result.Requirements = nlScraper.Requirements
	return result, nil
}

func (w *Worker) catalogDescription(ctx context.Context, url string) (scraper.BasicScraperResult, bool) {
	if w.Catalog == nil {
		return scraper.BasicScraperResult{}, false
	}
	row, found, errR := w.Catalog.SelectByURL(ctx, url)
	if errR != nil {
		w.Logger.Warn("doScraper: erro ao consultar o catálogo de vagas", zap.String("url", url), zap.String("erro", errR.Message))
		return scraper.BasicScraperResult{}, false
	}
	if !found {
		return scraper.BasicScraperResult{}, false
	}
	return scraper.BasicScraperResult{Title: row.Title, Company: row.Company, BasicDescription: row.Description}, true
}
