package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/admin"
	"hackton-treino/internal/services"
	"hackton-treino/internal/sse"
	"hackton-treino/internal/util"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ── Seleção automática (botão em Vagas, 0.4.0) ──
// Fluxo de um clique:
//  1. confere acesso (admin, master ou liberado no painel), se está ligada e o teto
//     do dia (vagas escolhidas desde 00h de Brasília);
//  2. pega as vagas do Descobrir já ranqueadas pela stack, com o local e o nível
//     salvos, e tira as que já estão na pipeline, as dispensadas e as que a IA leu
//     nos últimos 14 dias; ficam as N mais compatíveis acima do mínimo (candidates);
//  3. em segundo plano, a IA lê essas vagas em lotes e dá nota e motivo a cada uma;
//  4. as aprovadas com nota mínima, da maior para a menor, até o limite do clique,
//     entram na pipeline com a tag "auto-gerado" e o worker gera o currículo;
//  5. tudo fica registrado (rodada e cada vaga lida) para a tela mostrar o porquê e
//     para as métricas.
// A candidatura em si continua com a pessoa: os sites não permitem enviar por ela.

// autoSelectStaleAfter: rodada "running" mais velha que isso foi interrompida.
const autoSelectStaleAfter = 15 * time.Minute

// autoSelectRunTimeout: tempo máximo da leitura pela IA numa rodada.
const autoSelectRunTimeout = 10 * time.Minute

// autoSelectMaxAge: vaga publicada há mais tempo não entra (provavelmente já fechou).
const autoSelectMaxAge = 30 * 24 * time.Hour

type AutoSelectHandler struct {
	*BaseHandler
	repository.Base
	Disc  *DiscoveryHandler
	Admin admin.Repository
	LLM   *services.AiService
	Bus   *sse.Bus
}

func NewAutoSelectHandler(logger *zap.Logger, conn *pgxpool.Pool, disc *DiscoveryHandler, adminRepo admin.Repository, llm *services.AiService, bus *sse.Bus) *AutoSelectHandler {
	return &AutoSelectHandler{
		BaseHandler: NewBaseHandler(logger),
		Base:        repository.NewBase(conn),
		Disc:        disc,
		Admin:       adminRepo,
		LLM:         llm,
		Bus:         bus,
	}
}

// AutoSelectSettings: a configuração do painel admin.
type AutoSelectSettings struct {
	Enabled    bool       `json:"enabled"`
	PerRun     int32      `json:"per_run"`
	PerDay     int32      `json:"per_day"`
	Candidates int32      `json:"candidates"`
	MinScore   int32      `json:"min_score"`
	MinQuality string     `json:"min_quality"`
	Mode       string     `json:"mode"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type AutoSelectPick struct {
	Url         string `json:"url"`
	Title       string `json:"title"`
	Company     string `json:"company"`
	CoveragePct int32  `json:"coverage_pct"`
	Quality     string `json:"quality"`
	Score       *int32 `json:"score,omitempty"`
	Selected    bool   `json:"selected"`
	Reason      string `json:"reason"`
	JobID       string `json:"job_id,omitempty"`
	JobStatus   string `json:"job_status,omitempty"`
}

type AutoSelectRun struct {
	ID         string           `json:"id"`
	Status     string           `json:"status"` // running | done | empty | error
	Requested  int32            `json:"requested"`
	Candidates int32            `json:"candidates"`
	Analyzed   int32            `json:"analyzed"`
	Selected   int32            `json:"selected"`
	Error      string           `json:"error,omitempty"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt *time.Time       `json:"finished_at,omitempty"`
	Picks      []AutoSelectPick `json:"picks"`
}

type AutoSelectStatus struct {
	Allowed   bool           `json:"allowed"`
	Enabled   bool           `json:"enabled"`
	PerRun    int32          `json:"per_run"`
	PerDay    int32          `json:"per_day"`
	UsedToday int32          `json:"used_today"`
	Remaining int32          `json:"remaining"`
	Run       *AutoSelectRun `json:"run,omitempty"`
}

func settingsFromRow(s db.AutoSelectGetSettingsRow) AutoSelectSettings {
	return AutoSelectSettings{
		Enabled: s.Enabled, PerRun: s.PerRun, PerDay: s.PerDay, Candidates: s.Candidates,
		MinScore: s.MinScore, MinQuality: s.MinQuality, Mode: s.Mode, UpdatedAt: pgTimePtr(s.UpdatedAt),
	}
}

// brtDayStart: 00h de hoje em Brasília (o teto diário vira à meia-noite daqui).
func brtDayStart(now time.Time) time.Time {
	t := now.In(brt)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, brt)
}

func (h *AutoSelectHandler) allowed(ctx context.Context, userID pgtype.UUID) (bool, error) {
	ok, err := h.Q.AutoSelectAccess(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

// Status: GET /auto-select — acesso, limites, quanto já usou hoje e a última rodada.
func (h *AutoSelectHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	ctx := r.Context()
	allowed, err := h.allowed(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	if !allowed {
		writeJSON(w, http.StatusOK, AutoSelectStatus{})
		return
	}
	set, err := h.Q.AutoSelectGetSettings(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	_ = h.Q.AutoSelectExpireStale(ctx, pgtype.Timestamptz{Time: time.Now().Add(-autoSelectStaleAfter), Valid: true})
	used, err := h.Q.AutoSelectUsedSince(ctx, db.AutoSelectUsedSinceParams{UserID: userID, Since: pgtype.Timestamptz{Time: brtDayStart(time.Now()), Valid: true}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	out := AutoSelectStatus{
		Allowed: true, Enabled: set.Enabled, PerRun: set.PerRun, PerDay: set.PerDay,
		UsedToday: used, Remaining: max(set.PerDay-used, 0),
	}
	if last, err := h.Q.AutoSelectLatestRun(ctx, userID); err == nil {
		run := runView(last.ID, last.Status, last.Requested, last.Candidates, last.Analyzed, last.Selected, last.Error, last.StartedAt, last.FinishedAt)
		h.attachPicks(ctx, userID, &run)
		out.Run = &run
	}
	writeJSON(w, http.StatusOK, out)
}

// GetRun: GET /auto-select/runs/{id} — a tela acompanha a rodada por aqui.
func (h *AutoSelectHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	id, errP := util.ParseUUID(r.PathValue("id"))
	if errP != nil {
		writeRepositoryError(w, errP)
		return
	}
	row, err := h.Q.AutoSelectGetRun(r.Context(), db.AutoSelectGetRunParams{ID: id, UserID: userID})
	if err != nil {
		writeRepositoryError(w, repository.HandleDatabaseError(err))
		return
	}
	run := runView(row.ID, row.Status, row.Requested, row.Candidates, row.Analyzed, row.Selected, row.Error, row.StartedAt, row.FinishedAt)
	h.attachPicks(r.Context(), userID, &run)
	writeJSON(w, http.StatusOK, run)
}

func runView(id pgtype.UUID, status string, requested, candidates, analyzed, selected int32, errMsg string, started, finished pgtype.Timestamptz) AutoSelectRun {
	return AutoSelectRun{
		ID: id.String(), Status: status, Requested: requested, Candidates: candidates,
		Analyzed: analyzed, Selected: selected, Error: errMsg,
		StartedAt: started.Time, FinishedAt: pgTimePtr(finished), Picks: []AutoSelectPick{},
	}
}

func (h *AutoSelectHandler) attachPicks(ctx context.Context, userID pgtype.UUID, run *AutoSelectRun) {
	var id pgtype.UUID
	if id.Scan(run.ID) != nil {
		return
	}
	rows, err := h.Q.AutoSelectRunPicks(ctx, db.AutoSelectRunPicksParams{RunID: id, UserID: userID})
	if err != nil {
		h.Logger.Warn("auto-select: erro ao listar vagas da rodada", zap.Error(err))
		return
	}
	for _, p := range rows {
		pick := AutoSelectPick{
			Url: p.Url, Title: p.Title, Company: p.Company, CoveragePct: p.CoveragePct,
			Quality: p.Quality, Selected: p.Selected, Reason: p.Reason, JobStatus: p.JobStatus,
		}
		if p.Score.Valid {
			s := p.Score.Int32
			pick.Score = &s
		}
		if p.JobID.Valid {
			pick.JobID = p.JobID.String()
		}
		run.Picks = append(run.Picks, pick)
	}
}

// Start: POST /auto-select/runs — escolhe as candidatas na hora e manda a IA ler em
// segundo plano. 202 com a rodada ("running"), ou 200 já terminada ("empty") quando
// não há vaga nova para ler.
func (h *AutoSelectHandler) Start(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	ctx := r.Context()
	allowed, err := h.allowed(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "a seleção automática não está liberada para a sua conta")
		return
	}
	set, err := h.Q.AutoSelectGetSettings(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	if !set.Enabled {
		writeError(w, http.StatusConflict, "a seleção automática está desligada no momento")
		return
	}
	_ = h.Q.AutoSelectExpireStale(ctx, pgtype.Timestamptz{Time: time.Now().Add(-autoSelectStaleAfter), Valid: true})
	used, err := h.Q.AutoSelectUsedSince(ctx, db.AutoSelectUsedSinceParams{UserID: userID, Since: pgtype.Timestamptz{Time: brtDayStart(time.Now()), Valid: true}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	n := min(set.PerRun, set.PerDay-used)
	if n <= 0 {
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("limite de hoje atingido (%d vagas por dia). Amanhã libera de novo.", set.PerDay))
		return
	}

	cands, filter, errR := h.candidates(ctx, userID, set)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}

	started, err := h.Q.AutoSelectStartRun(ctx, db.AutoSelectStartRunParams{UserID: userID, Requested: n})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(w, http.StatusConflict, "já existe uma seleção em andamento")
			return
		}
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	run := AutoSelectRun{ID: started.ID.String(), Status: "running", Requested: n, Candidates: int32(len(cands)), StartedAt: started.StartedAt.Time, Picks: []AutoSelectPick{}}

	if len(cands) == 0 {
		h.finish(context.WithoutCancel(ctx), started.ID, "empty", 0, 0, 0, "")
		run.Status = "empty"
		writeJSON(w, http.StatusOK, run)
		return
	}

	// A leitura continua depois da resposta (e se a pessoa fechar a aba).
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), autoSelectRunTimeout)
	go func() {
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				h.Logger.Error("auto-select: panic na rodada", zap.Any("panic", rec))
				h.finish(context.Background(), started.ID, "error", int32(len(cands)), 0, 0, "erro interno")
			}
		}()
		h.run(bg, userID, started.ID, cands, filter, set, int(n))
	}()
	writeJSON(w, http.StatusAccepted, run)
}

// candidates: as vagas do Descobrir que a IA vai ler, na ordem do ranking.
func (h *AutoSelectHandler) candidates(ctx context.Context, userID pgtype.UUID, set db.AutoSelectGetSettingsRow) ([]DiscoveredJob, discoveryFilter, *repository.RepositoryError) {
	rk, errR := h.Disc.rankedJobs(ctx, userID, hideDismissed)
	if errR != nil || rk.jobs == nil {
		return nil, rk.filter, errR
	}
	excludedRows, err := h.Q.AutoSelectExcludedUrls(ctx, userID)
	if err != nil {
		return nil, rk.filter, repository.HandleDatabaseError(err)
	}
	excluded := make(map[string]bool, len(excludedRows))
	for _, u := range excludedRows {
		excluded[u] = true
	}
	minRank := autoQualityRank(db.JobQuality(set.MinQuality))
	oldest := time.Now().Add(-autoSelectMaxAge)

	// Nível: o escolhido no Descobrir manda (filterDiscovered já aplica). Sem escolha,
	// vem do tempo de carreira; sem experiências com data, não filtra.
	var target, stretch []string
	autoLevel := false
	if len(rk.filter.Seniority.Levels) == 0 {
		if exps, errE := h.Disc.Users.QuerySelectAllExperiences(ctx, userID.String()); errE == nil {
			if total, ok := autoExperience(exps, time.Now()); ok {
				target, stretch = autoLevelFit(total.Hours() / 24 / 365.25)
				autoLevel = true
			}
		}
	}

	limit := int(set.Candidates)
	out, extra := []DiscoveredJob{}, []DiscoveredJob{}
	dedup := autoDedup{}
	for _, j := range filterDiscovered(rk.jobs, rk.filter).Jobs {
		if len(out) >= limit {
			break
		}
		if len(j.fit.Items) == 0 || autoQualityRank(j.fit.Quality) < minRank {
			continue
		}
		// Quem marcou skills avançadas/expert: a vaga precisa usar alguma delas.
		if rk.onlyStrong && j.fit.Strong == 0 {
			continue
		}
		if j.PublishedAt != nil && j.PublishedAt.Before(oldest) {
			continue
		}
		if excluded[j.Url] || anyExcluded(j.AlsoOn, excluded) {
			continue
		}
		if autoIsNonTech(j.Title) || dedup.seen(j.Title, j.Company) {
			continue
		}
		rank := 0
		if autoLevel {
			rank = autoLevelRank(j.Seniority, target, stretch)
		}
		switch rank {
		case 0:
			out = append(out, j)
		case 1:
			extra = append(extra, j)
		}
	}
	// Nível alvo primeiro (na ordem de compatibilidade); "esticar" só completa.
	out = append(out, extra[:min(len(extra), max(limit-len(out), 0))]...)
	return out, rk.filter, nil
}

func anyExcluded(links []JobLink, excluded map[string]bool) bool {
	for _, l := range links {
		if excluded[l.Url] {
			return true
		}
	}
	return false
}

// run: a IA lê as candidatas, as melhores entram na pipeline e a rodada é registrada.
func (h *AutoSelectHandler) run(ctx context.Context, userID pgtype.UUID, runID pgtype.UUID, jobs []DiscoveredJob, filter discoveryFilter, set db.AutoSelectGetSettingsRow, n int) {
	start := time.Now()
	exps, _ := h.Disc.Users.QuerySelectAllExperiences(ctx, userID.String())
	skills, _ := h.Disc.Users.QuerySelectAllSkills(ctx, userID.String())
	profile := autoProfileText(exps, skills, filter, time.Now())
	years := -1.0
	if total, ok := autoExperience(exps, time.Now()); ok {
		years = total.Hours() / 24 / 365.25
	}

	ids := make([]pgtype.UUID, 0, len(jobs))
	for _, j := range jobs {
		var id pgtype.UUID
		if id.Scan(j.ID) == nil {
			ids = append(ids, id)
		}
	}
	descs := map[string]string{}
	if rows, err := h.Q.AutoSelectDescriptions(ctx, ids); err == nil {
		for _, d := range rows {
			descs[d.ID.String()] = d.Description
		}
	}

	cands := make([]autoCandidate, len(jobs))
	for i, j := range jobs {
		cands[i] = autoCandidate{Job: j}
	}
	var lastErr error
	analyzed := 0
	for from := 0; from < len(cands); from += autoSelectBatch {
		if ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}
		to := min(from+autoSelectBatch, len(cands))
		batch := cands[from:to]
		var user strings.Builder
		user.WriteString("PERFIL DO CANDIDATO\n")
		user.WriteString(profile)
		user.WriteString("\nVAGAS\n")
		for i, c := range batch {
			user.WriteString("\n")
			user.WriteString(autoJobText(i+1, c.Job, descs[c.Job.ID]))
		}
		var verdicts map[int]autoVerdict
		err := h.LLM.ExtractJSON(ctx, services.PurposeTriage, 150*time.Second, autoSelectSystemPrompt, user.String(), func(content string) error {
			v, err := parseAutoVerdicts(content, len(batch))
			if err == nil {
				verdicts = v
			}
			return err
		})
		if err != nil {
			lastErr = err
			h.Logger.Warn("auto-select: lote sem resposta da IA", zap.String("run_id", runID.String()), zap.Error(err))
			var rl *services.ErrRateLimit
			if errors.As(err, &rl) {
				break // os próximos lotes também esbarrariam no limite
			}
			continue
		}
		for i := range batch {
			if v, ok := verdicts[i+1]; ok {
				autoApplyRules(&v, batch[i].Job, years, filter.Location.WorkplaceTypes)
				batch[i].Verdict = &v
				analyzed++
			}
		}
	}

	chosen := autoChoose(cands, int(set.MinScore), n)
	jobIDs := map[int]pgtype.UUID{}
	for _, i := range chosen {
		c := cands[i]
		lang := c.Job.Language
		if lang != "en" {
			lang = "pt-br"
		}
		id, err := h.Q.QueryInsertAutoJob(ctx, db.QueryInsertAutoJobParams{
			ExternalUrl: c.Job.Url,
			UserID:      userID,
			CompanyName: pgtype.Text{String: c.Job.Company, Valid: c.Job.Company != ""},
			JobTitle:    pgtype.Text{String: c.Job.Title, Valid: c.Job.Title != ""},
			Language:    pgtype.Text{String: lang, Valid: true},
			Mode:        set.Mode,
			AutoScore:   pgtype.Int4{Int32: int32(c.Verdict.Score), Valid: true},
			AutoReason:  pgtype.Text{String: c.Verdict.Reason, Valid: c.Verdict.Reason != ""},
		})
		if err != nil {
			// Já estava na pipeline (corrida com um clique em "Gerar"): fica de fora.
			if !errors.Is(err, pgx.ErrNoRows) {
				h.Logger.Warn("auto-select: erro ao pôr vaga na pipeline", zap.String("url", c.Job.Url), zap.Error(err))
			}
			continue
		}
		jobIDs[i] = id
		if h.Bus != nil {
			h.Bus.Publish(userID.String(), sse.JobEvent{ID: id.String(), Status: string(db.JobStatusPending), CompanyName: c.Job.Company, JobTitle: c.Job.Title})
		}
	}

	// Registro: as que a IA leu (e as que ficaram sem resposta), com nota e motivo.
	rec := context.WithoutCancel(ctx)
	for i, c := range cands {
		p := db.AutoSelectInsertPickParams{
			RunID: runID, UserID: userID, Url: c.Job.Url, Title: c.Job.Title, Company: c.Job.Company,
			CoveragePct: int32(c.Job.fit.CoveragePct), Quality: string(c.Job.fit.Quality),
		}
		if c.Verdict != nil {
			p.Score = pgtype.Int4{Int32: int32(c.Verdict.Score), Valid: true}
			p.Reason = c.Verdict.Reason
		} else {
			p.Reason = "A IA não respondeu sobre esta vaga."
		}
		if id, ok := jobIDs[i]; ok {
			p.Selected, p.JobID = true, id
		}
		if err := h.Q.AutoSelectInsertPick(rec, p); err != nil {
			h.Logger.Warn("auto-select: erro ao registrar vaga lida", zap.Error(err))
		}
	}

	status, msg := "done", ""
	switch {
	case analyzed == 0:
		status = "error"
		msg = "A IA não conseguiu analisar as vagas agora. Tente de novo mais tarde."
		if lastErr != nil {
			var rl *services.ErrRateLimit
			if errors.As(lastErr, &rl) {
				msg = "A IA atingiu o limite de uso agora. Tente de novo mais tarde."
			}
		}
	case len(jobIDs) == 0:
		status = "empty"
	}
	h.finish(rec, runID, status, int32(len(cands)), int32(analyzed), int32(len(jobIDs)), msg)
	h.Logger.Info("auto-select: rodada concluída",
		zap.String("run_id", runID.String()), zap.String("status", status),
		zap.Int("candidatas", len(cands)), zap.Int("lidas", analyzed), zap.Int("escolhidas", len(jobIDs)),
		zap.Duration("duracao", time.Since(start)))
}

func (h *AutoSelectHandler) finish(ctx context.Context, runID pgtype.UUID, status string, candidates, analyzed, selected int32, msg string) {
	if err := h.Q.AutoSelectFinishRun(ctx, db.AutoSelectFinishRunParams{
		ID: runID, Status: status, Candidates: candidates, Analyzed: analyzed, Selected: selected, Error: msg,
	}); err != nil {
		h.Logger.Error("auto-select: erro ao fechar rodada", zap.String("run_id", runID.String()), zap.Error(err))
	}
}

// ── Painel admin ──

// GetSettings: GET /admin/auto-select/settings.
func (h *AutoSelectHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	set, err := h.Q.AutoSelectGetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	writeJSON(w, http.StatusOK, settingsFromRow(set))
}

var autoSelectModes = map[string]bool{"full": true, "resume_only": true, "cover_only": true}

func validateAutoSelectSettings(s AutoSelectSettings) error {
	switch {
	case s.PerRun < 1 || s.PerRun > 20:
		return errors.New("vagas por clique: de 1 a 20")
	case s.PerDay < 1 || s.PerDay > 100:
		return errors.New("vagas por dia: de 1 a 100")
	case s.PerRun > s.PerDay:
		return errors.New("vagas por clique não pode passar das vagas por dia")
	case s.Candidates < 5 || s.Candidates > 50:
		return errors.New("vagas lidas pela IA: de 5 a 50")
	case s.Candidates < s.PerRun:
		return errors.New("a IA precisa ler pelo menos tantas vagas quanto escolhe por clique")
	case s.MinScore < 0 || s.MinScore > 100:
		return errors.New("nota mínima: de 0 a 100")
	case s.MinQuality != "high" && s.MinQuality != "mid" && s.MinQuality != "low":
		return errors.New("compatibilidade mínima inválida")
	case !autoSelectModes[s.Mode]:
		return errors.New("o que gerar: inválido")
	}
	return nil
}

// SaveSettings: PUT /admin/auto-select/settings.
func (h *AutoSelectHandler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	var body AutoSelectSettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	if err := validateAutoSelectSettings(body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.Q.AutoSelectSaveSettings(r.Context(), db.AutoSelectSaveSettingsParams{
		Enabled: body.Enabled, PerRun: body.PerRun, PerDay: body.PerDay, Candidates: body.Candidates,
		MinScore: body.MinScore, MinQuality: body.MinQuality, Mode: body.Mode, UpdatedBy: userID,
	}); err != nil {
		h.Logger.Error("auto-select: erro ao salvar configuração", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	h.GetSettings(w, r)
}

// SetAccess: PATCH /admin/users/{id}/auto-select {allowed} — libera para quem não é
// admin (admin e master sempre podem).
func (h *AutoSelectHandler) SetAccess(w http.ResponseWriter, r *http.Request) {
	id, errP := util.ParseUUID(r.PathValue("id"))
	if errP != nil {
		writeRepositoryError(w, errP)
		return
	}
	var body struct {
		Allowed bool `json:"allowed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	n, err := h.Q.AutoSelectSetAccess(r.Context(), db.AutoSelectSetAccessParams{Allowed: body.Allowed, UserID: id})
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "usuário não encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Metrics: GET /admin/metrics/auto-select?range=24h|7d|30d.
func (h *AutoSelectHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	windows := map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}
	win, ok := windows[r.URL.Query().Get("range")]
	if !ok {
		win = windows["7d"]
	}
	since := pgtype.Timestamptz{Time: time.Now().Add(-win), Valid: true}
	ctx := r.Context()
	sum, err := h.Q.AutoSelectMetricsSummary(ctx, since)
	if err != nil {
		h.Logger.Error("auto-select: erro nas métricas", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	runs, err := h.Q.AutoSelectMetricsRuns(ctx, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	users, err := h.Q.AutoSelectMetricsUsers(ctx, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	set, _ := h.Q.AutoSelectGetSettings(ctx)

	type runOut struct {
		ID         string     `json:"id"`
		Email      string     `json:"email"`
		Status     string     `json:"status"`
		Requested  int32      `json:"requested"`
		Candidates int32      `json:"candidates"`
		Analyzed   int32      `json:"analyzed"`
		Selected   int32      `json:"selected"`
		Error      string     `json:"error,omitempty"`
		StartedAt  time.Time  `json:"started_at"`
		FinishedAt *time.Time `json:"finished_at,omitempty"`
	}
	runList := make([]runOut, len(runs))
	for i, x := range runs {
		runList[i] = runOut{x.ID.String(), x.Email, x.Status, x.Requested, x.Candidates, x.Analyzed, x.Selected, x.Error, x.StartedAt.Time, pgTimePtr(x.FinishedAt)}
	}
	if users == nil {
		users = []db.AutoSelectMetricsUsersRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"summary":  sum,
		"runs":     runList,
		"users":    users,
		"settings": settingsFromRow(set),
	})
}
