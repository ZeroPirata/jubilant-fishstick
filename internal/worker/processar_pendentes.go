package worker

import (
	"context"
	"encoding/json"
	"errors"
	"hackton-treino/internal/db"
	"hackton-treino/internal/scraper"
	"hackton-treino/internal/services"
	"hackton-treino/internal/util"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const rateLimitTTL = 12 * time.Hour

func (w *Worker) processarPendentes(ctx context.Context) {
	if w.Cache.IsRateLimited(ctx) {
		w.Logger.Info("LLM rate limit ativo, aguardando janela expirar")
		return
	}

	const batchSize int32 = 20
	vagas, err := w.Pipeline.WorkerSelectPendingJobs(ctx, batchSize)
	if err != nil {
		w.Logger.Error("Erro ao buscar vagas pendentes", zap.Error(err))
		return
	}

	w.Logger.Info("Quantidade de vagas pendentes", zap.Int("quantity", len(vagas)))

	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, vaga := range vagas {
		wg.Add(1)
		sem <- struct{}{}
		go func(v db.Job) {
			defer wg.Done()
			defer func() { <-sem }()
			w.processarJob(ctx, &v)
		}(vaga)
	}
	wg.Wait()
}

func (w *Worker) processarJob(ctx context.Context, job *db.Job) {
	jobID := job.ID.String()
	userID := job.UserID.String()
	w.Logger.Info("Processando vaga", zap.String("job_id", jobID))

	if err := w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{
		Status: db.JobStatusProcessing,
		ID:     job.ID,
	}); err != nil {
		w.Logger.Error("Erro ao atualizar status da vaga para processing", zap.String("job_id", jobID), zap.Error(err))
		return
	}

	// Run scraper, match, and filters concurrently — all are independent at this stage.
	type scraperOut struct {
		result *scraper.ResultScraper
		err    error
	}
	type matchOut struct {
		matches services.MatchResult
		err     error
	}
	type filtersOut struct {
		filtros []string
		err     error
	}

	scraperCh := make(chan scraperOut, 1)
	matchCh := make(chan matchOut, 1)
	filtersCh := make(chan filtersOut, 1)

	go func() {
		r, err := w.doScraper(ctx, job)
		scraperCh <- scraperOut{r, err}
	}()
	go func() {
		m, err := w.matchComBancoPessoal(ctx, job)
		matchCh <- matchOut{m, err}
	}()
	go func() {
		f, err := w.Filters.QuerySelectFiltersForUser(ctx, userID)
		filtersCh <- filtersOut{f, err}
	}()

	sOut := <-scraperCh
	mOut := <-matchCh
	fOut := <-filtersCh

	if sOut.err != nil {
		if w.handleIfRateLimit(ctx, job, jobID, sOut.err) {
			return
		}
		w.Logger.Warn("Erro no scrape, marcando como error", zap.String("job_id", jobID), zap.Error(sOut.err))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	result := sOut.result

	if err := w.Pipeline.WorkerUpdateJob(ctx, db.WorkerUpdateJobParams{
		CompanyName:  util.ConvertToPgText(result.Company),
		JobTitle:     util.ConvertToPgText(result.Title),
		Description:  util.ConvertToPgText(result.CompressedDescription),
		TechStack:    result.Stack,
		Requirements: result.Requirements,
		Language:     job.Language,
		ID:           job.ID,
	}); err != nil {
		w.Logger.Error("Erro ao salvar dados do scrape", zap.String("job_id", jobID), zap.Error(err))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	if fOut.err != nil {
		w.Logger.Error("Erro ao buscar filtros do usuário", zap.String("job_id", jobID), zap.Error(fOut.err))
	}

	quality := calcularQualidade(result, fOut.filtros, w.aliases)
	if quality == db.JobQualityLow {
		w.Logger.Info("Vaga fora do perfil", zap.String("job_id", jobID))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusCompleted, ID: job.ID})
		_ = w.Pipeline.WorkerUpdateJobQuality(ctx, db.WorkerUpdateJobQualityParams{
			Quality: db.NullJobQuality{JobQuality: db.JobQualityLow, Valid: true},
			ID:      job.ID,
		})
		return
	}

	if mOut.err != nil {
		w.Logger.Error("Erro ao fazer match com banco pessoal", zap.String("job_id", jobID), zap.Error(mOut.err))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	str, errJ := w.buildUserPrompt(job, &mOut.matches)
	if errJ != nil {
		w.Logger.Error("Erro ao montar user prompt", zap.String("job_id", jobID), zap.Error(errJ))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	promptPath := promptPTBRPath
	if job.Language.String == "en" {
		promptPath = promptENPath
	}

	prompt, errP := w.loadPrompt(promptPath)
	if errP != nil {
		w.Logger.Error("Erro ao carregar system prompt", zap.String("path", promptPath), zap.Error(errP))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	llmResponse, errLLM := w.LLM.GenerateCurriculum(ctx, prompt, str)
	if errLLM != nil {
		if w.handleIfRateLimit(ctx, job, jobID, errLLM) {
			return
		}
		w.Logger.Error("Erro ao chamar LLM", zap.String("job_id", jobID), zap.Error(errLLM))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	if !strings.Contains(llmResponse.Curriculo, "{{CANDIDATO_NOME}}") {
		w.Logger.Error("LLM não respeitou tokens de sistema — currículo rejeitado",
			zap.String("job_id", jobID),
			zap.String("preview", firstN(llmResponse.Curriculo, 120)),
		)
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	profile, errProfile := w.Users.QuerySelectProfile(ctx, userID)
	if errProfile != nil {
		w.Logger.Warn("Erro ao buscar perfil do usuário, usando placeholders", zap.String("job_id", jobID), zap.Error(errProfile))
	}

	var webLinks []string
	if profile.PortfolioUrl.Valid && profile.PortfolioUrl.String != "" {
		webLinks = append(webLinks, profile.PortfolioUrl.String)
	}
	if len(profile.OtherLinks) > 0 {
		var otherLinks []struct {
			Label string `json:"label"`
			URL   string `json:"url"`
		}
		if err := json.Unmarshal(profile.OtherLinks, &otherLinks); err == nil {
			for _, l := range otherLinks {
				if l.URL != "" {
					webLinks = append(webLinks, l.URL)
				}
			}
		}
	}
	portfolioStr := strings.Join(webLinks, " | ")

	replacer := strings.NewReplacer(
		"{{CANDIDATO_NOME}}", profile.FullName.String,
		"{{CANDIDATO_EMAIL}}", profile.Email,
		"{{CANDIDATO_LINKEDIN}}", profile.LinkedinUrl.String,
		"{{CANDIDATO_GITHUB}}", profile.GithubUrl.String,
		"{{CANDIDATO_PORTFOLIO}}", portfolioStr,
		"{{CANDIDAO_PORTFOLIO}}", portfolioStr,
		"{{CANDIDATO_TELEFONE}}", profile.Phone.String,
		"{{VAGA_EMPRESA}}", job.CompanyName.String,
		"{{VAGA_TITULO}}", job.JobTitle.String,
	)

	rePlaceholder := regexp.MustCompile(`\s*\|\s*\{\{[^}]+\}\}|\{\{[^}]+\}\}\s*\|\s*|\{\{[^}]+\}\}`)
	cleanUp := func(s string) string {
		return rePlaceholder.ReplaceAllString(replacer.Replace(s), "")
	}

	conteudo := struct {
		Curriculo   string `json:"curriculo"`
		CoverLetter string `json:"cover_letter"`
	}{
		Curriculo:   cleanUp(llmResponse.Curriculo),
		CoverLetter: cleanUp(llmResponse.CoverLetter),
	}

	conteudoJSON, errJSON := json.Marshal(conteudo)
	if errJSON != nil {
		w.Logger.Error("Erro ao serializar conteudo do curriculo", zap.String("job_id", jobID), zap.Error(errJSON))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	_, errInsert := w.Pipeline.WorkerInsertGeneratedResume(ctx, db.WorkerInsertGeneratedResumeParams{
		JobID:       job.ID,
		UserID:      job.UserID,
		ContentJson: conteudoJSON,
	})
	if errInsert != nil {
		w.Logger.Error("Erro ao inserir curriculo gerado", zap.String("job_id", jobID), zap.Error(errInsert))
		_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusError, ID: job.ID})
		return
	}

	_ = w.Pipeline.WorkerUpdateJobQuality(ctx, db.WorkerUpdateJobQualityParams{
		Quality: db.NullJobQuality{JobQuality: quality, Valid: true},
		ID:      job.ID,
	})
	_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{Status: db.JobStatusCompleted, ID: job.ID})

	w.Logger.Info("Vaga processada com sucesso", zap.String("url", job.ExternalUrl), zap.String("job_id", jobID))
}

// handleIfRateLimit verifica se o erro é um ErrRateLimit. Se sim, seta a flag no Redis,
// volta o job para pending e retorna true. Caso contrário retorna false.
func (w *Worker) handleIfRateLimit(ctx context.Context, job *db.Job, jobID string, err error) bool {
	var errRL *services.ErrRateLimit
	if !errors.As(err, &errRL) {
		return false
	}

	w.Logger.Warn("Rate limit da LLM atingido, reagendando vaga e pausando worker",
		zap.String("job_id", jobID),
		zap.Duration("pausa", rateLimitTTL),
	)

	if cacheErr := w.Cache.SetRateLimit(ctx, rateLimitTTL); cacheErr != nil {
		w.Logger.Error("Erro ao setar flag de rate limit no Redis", zap.Error(cacheErr))
	}

	_ = w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{
		Status: db.JobStatusPending,
		ID:     job.ID,
	})

	return true
}
