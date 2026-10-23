package worker

import (
	"context"
	"encoding/json"
	"errors"
	"hackton-treino/internal/db"
	"hackton-treino/internal/metrics"
	"hackton-treino/internal/repository/admin"
	"hackton-treino/internal/resume"
	"hackton-treino/internal/scraper"
	"hackton-treino/internal/services"
	"hackton-treino/internal/sse"
	"hackton-treino/internal/util"
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

	vagas, err := w.Pipeline.WorkerSelectPendingJobs(ctx, w.batchSize)
	if err != nil {
		w.Logger.Error("Erro ao buscar vagas pendentes", zap.Error(err))
		return
	}

	w.Logger.Info("Quantidade de vagas pendentes", zap.Int("quantity", len(vagas)))

	sem := make(chan struct{}, w.maxConcurrent)
	var wg sync.WaitGroup

	for _, vaga := range vagas {
		wg.Add(1)
		sem <- struct{}{}
		metrics.WorkerGoroutines.Inc()
		go func(v db.Job) {
			defer wg.Done()
			defer func() { <-sem }()
			defer metrics.WorkerGoroutines.Dec()
			w.processarJob(ctx, &v)
		}(vaga)
	}
	wg.Wait()
}

func (w *Worker) processarJob(ctx context.Context, job *db.Job) {
	jobID := job.ID.String()
	ctx = services.WithJobID(ctx, jobID)
	userID := job.UserID.String()
	w.Logger.Info("Processando vaga", zap.String("job_id", jobID))

	if err := w.Pipeline.WorkerUpdateJobStatus(ctx, db.WorkerUpdateJobStatusParams{
		Status: db.JobStatusProcessing,
		ID:     job.ID,
	}); err != nil {
		w.Logger.Error("Erro ao atualizar status da vaga para processing", zap.String("job_id", jobID), zap.Error(err))
		return
	}
	if w.Bus != nil {
		w.Bus.Publish(userID, sse.JobEvent{ID: jobID, Status: string(db.JobStatusProcessing)})
	}

	// Run scraper and match concurrently — both are independent at this stage.
	type scraperOut struct {
		result *scraper.ResultScraper
		err    error
	}
	type matchOut struct {
		matches services.MatchResult
		err     error
	}

	scraperCh := make(chan scraperOut, 1)
	matchCh := make(chan matchOut, 1)

	go func() {
		r, err := w.doScraper(ctx, job)
		scraperCh <- scraperOut{r, err}
	}()
	go func() {
		m, err := w.matchComBancoPessoal(ctx, job)
		matchCh <- matchOut{m, err}
	}()

	sOut := <-scraperCh
	mOut := <-matchCh

	if sOut.err != nil {
		if w.handleIfRateLimit(ctx, job, jobID, sOut.err) {
			return
		}
		errType := "scraper_error"
		switch {
		case errors.Is(sOut.err, scraper.ErrUnsupportedDomain):
			errType = "scraper_incompatible"
		case errors.Is(sOut.err, scraper.ErrEmptyDescription):
			errType = "scraper_empty"
		}
		jobIDStr := jobID
		userIDStr := userID
		url := job.ExternalUrl
		msg := sOut.err.Error()
		w.AdminRepo.InsertErrorLog(ctx, admin.InsertErrorLogParams{
			JobID:        &jobIDStr,
			UserID:       &userIDStr,
			ErrorType:    errType,
			ErrorMessage: msg,
			URL:          &url,
		})
		w.Logger.Warn("Erro no scrape, marcando como error", zap.String("job_id", jobID), zap.String("type", errType), zap.Error(sOut.err))
		w.failJob(ctx, job)
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
		w.failJob(ctx, job)
		return
	}
	// Atualiza campos do job em memória para o evento SSE de completed ter company/title
	job.CompanyName = util.ConvertToPgText(result.Company)
	job.JobTitle = util.ConvertToPgText(result.Title)

	if mOut.err != nil {
		w.Logger.Error("Erro ao fazer match com banco pessoal", zap.String("job_id", jobID), zap.Error(mOut.err))
		w.failJob(ctx, job)
		return
	}

	// Sem nada no perfil não há o que a IA escrever: falha aqui em vez de pagar uma chamada.
	// (Só experiência vazia é ok — o prompt tem a variante "primeiro emprego".)
	if m := mOut.matches; len(m.Experiencias) == 0 && len(m.Projetos) == 0 && len(m.Habilidades) == 0 &&
		len(m.Formacoes) == 0 && len(m.Certificacoes) == 0 {
		jobIDStr, userIDStr, url := jobID, userID, job.ExternalUrl
		w.AdminRepo.InsertErrorLog(ctx, admin.InsertErrorLogParams{
			JobID:        &jobIDStr,
			UserID:       &userIDStr,
			ErrorType:    "profile_incomplete",
			ErrorMessage: "perfil vazio: cadastre ao menos uma experiência, projeto, habilidade ou formação",
			URL:          &url,
		})
		w.Logger.Warn("Perfil vazio, currículo não gerado", zap.String("job_id", jobID))
		w.failJob(ctx, job)
		return
	}

	quality := calcularQualidade(result, mOut.matches.Habilidades, w.aliases)
	gap := analisarGap(result.Stack, mOut.matches.Habilidades, w.aliases)

	str, errJ := w.buildUserPrompt(job, &mOut.matches)
	if errJ != nil {
		w.Logger.Error("Erro ao montar user prompt", zap.String("job_id", jobID), zap.Error(errJ))
		w.failJob(ctx, job)
		return
	}

	prompt := w.promptPTBR
	if job.Language.String == "en" {
		prompt = w.promptEN
	}
	if prompt == "" {
		w.Logger.Error("System prompt não carregado", zap.String("job_id", jobID))
		w.failJob(ctx, job)
		return
	}

	profile, errProfile := w.Users.QuerySelectProfile(ctx, userID)
	if errProfile != nil {
		w.Logger.Warn("Erro ao buscar perfil do usuário, currículo sem contato", zap.String("job_id", jobID), zap.Error(errProfile))
	}
	src := w.buildSource(job, &mOut.matches, profile)

	llmStart := time.Now()
	llmResponse, errLLM := w.generateWithCorrection(ctx, jobID, prompt, str, src)
	llmStatus := "ok"
	if errLLM != nil {
		var errRL *services.ErrRateLimit
		if errors.As(errLLM, &errRL) {
			llmStatus = "rate_limit"
		} else {
			llmStatus = "error"
		}
	}
	metrics.LLMDuration.WithLabelValues(llmStatus).Observe(time.Since(llmStart).Seconds())
	if errLLM != nil {
		if w.handleIfRateLimit(ctx, job, jobID, errLLM) {
			return
		}
		llmErrType := "llm_error"
		var errRL *services.ErrRateLimit
		if errors.As(errLLM, &errRL) {
			llmErrType = "llm_rate_limit"
		}
		jobIDStr := jobID
		userIDStr := userID
		url := job.ExternalUrl
		msg := errLLM.Error()
		w.AdminRepo.InsertErrorLog(ctx, admin.InsertErrorLogParams{
			JobID:        &jobIDStr,
			UserID:       &userIDStr,
			ErrorType:    llmErrType,
			ErrorMessage: msg,
			URL:          &url,
		})
		w.Logger.Error("Erro ao chamar LLM", zap.String("job_id", jobID), zap.Error(errLLM))
		w.failJob(ctx, job)
		return
	}

	// A IA escreveu só o texto; empresa, cargo, datas e contato vêm do banco.
	doc, warnings := resume.Assemble(src, *llmResponse)
	for _, wr := range warnings {
		w.Logger.Warn("Currículo: conteúdo da IA descartado", zap.String("job_id", jobID), zap.String("motivo", wr))
	}

	coverLetter := strings.TrimSpace(rePlaceholder.ReplaceAllString(
		strings.ReplaceAll(llmResponse.CoverLetter, "{{CANDIDATO_NOME}}", src.Header.Name), ""))

	conteudo := storedResume{Version: resume.DocVersion, CoverLetter: coverLetter}
	if job.Mode != "cover_only" {
		conteudo.Doc = &doc
		conteudo.Curriculo = resume.Text(doc)
	}
	if job.Mode == "resume_only" {
		conteudo.CoverLetter = ""
	}

	conteudoJSON, errJSON := json.Marshal(conteudo)
	if errJSON != nil {
		w.Logger.Error("Erro ao serializar conteudo do curriculo", zap.String("job_id", jobID), zap.Error(errJSON))
		w.failJob(ctx, job)
		return
	}

	_, errInsert := w.Pipeline.WorkerInsertGeneratedResume(ctx, db.WorkerInsertGeneratedResumeParams{
		JobID:       job.ID,
		UserID:      job.UserID,
		ContentJson: conteudoJSON,
	})
	if errInsert != nil {
		w.Logger.Error("Erro ao inserir curriculo gerado", zap.String("job_id", jobID), zap.Error(errInsert))
		w.failJob(ctx, job)
		return
	}

	w.completeJob(ctx, job, quality, gap)

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
