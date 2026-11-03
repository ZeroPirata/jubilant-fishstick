package handler

import (
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"hackton-treino/internal/util"
	"net/http"
	"time"
)

// Alertas de vagas novas (sininho): vagas que entraram no catálogo do Descobrir desde
// a última vez que o usuário viu os alertas, com compatibilidade a partir do limite
// escolhido, já com o local e o nível salvos. Abrir o sininho ou o Descobrir marca
// como vistas (POST /discovery/alerts/seen): a mesma vaga não é avisada de novo.
//
// Para o sino não virar ruído, além da faixa de compatibilidade a vaga precisa:
//   - cobrir pelo menos uma skill avançada/expert do perfil (Git, SQL e Docker
//     sozinhos não bastam); quem não marcou nenhuma skill assim fica sem essa regra;
//   - ter sido publicada há no máximo alertMaxAge (vaga antiga que só agora entrou no
//     catálogo, por uma fonte nova, não é novidade); sem data de publicação, passa;
//   - não ter sido dispensada pelo usuário.
// As vagas que não passam continuam no Descobrir.
//
// Só na plataforma por enquanto; Telegram e e-mail podem usar a mesma lista
// (newAlerts) depois.

// alertListMax: teto de vagas que o painel do sininho lista (o total vem em count).
// Abrir o painel marca todas como vistas, então ele mostra todas as novas: o teto só
// protege contra um volume anormal (primeira varredura de uma fonte grande).
const alertListMax = 100

// alertMaxAge: vaga publicada antes disso não gera alerta.
const alertMaxAge = 7 * 24 * time.Hour

type JobAlerts struct {
	Enabled    bool            `json:"enabled"`
	MinQuality string          `json:"min_quality"`
	Since      time.Time       `json:"since"`
	Count      int             `json:"count"`
	Jobs       []DiscoveredJob `json:"jobs"`
}

type jobAlertSettings struct {
	Enabled    bool   `json:"enabled"`
	MinQuality string `json:"min_quality"`
}

func (h *DiscoveryHandler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	ctx := r.Context()
	cfg, errR := h.Repo.SelectAlerts(ctx, userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	out := JobAlerts{Enabled: cfg.Enabled, MinQuality: cfg.MinQuality, Since: cfg.SeenAt.Time, Jobs: []DiscoveredJob{}}
	if !cfg.Enabled {
		writeJSON(w, http.StatusOK, out)
		return
	}
	rk, errR := h.rankedJobs(ctx, userID, hideDismissed)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	if rk.jobs == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	rule := alertRule{
		since:         cfg.SeenAt.Time,
		minQuality:    db.JobQuality(cfg.MinQuality),
		requireStrong: rk.onlyStrong,
		publishedFrom: time.Now().Add(-alertMaxAge),
	}
	jobs := newAlerts(filterDiscovered(rk.jobs, rk.filter).Jobs, rule)
	out.Count = len(jobs)
	if len(jobs) > alertListMax {
		jobs = jobs[:alertListMax]
	}
	if errR := h.fillPage(ctx, userID, jobs, 0, alertListMax); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	out.Jobs = jobs
	writeJSON(w, http.StatusOK, out)
}

type alertRule struct {
	since         time.Time     // vistas pela primeira vez depois disso
	minQuality    db.JobQuality // high, ou mid (alta e média)
	requireStrong bool          // exige uma skill avançada/expert na stack
	publishedFrom time.Time     // publicadas a partir disso (sem data passa)
}

// newAlerts mantém, na ordem do ranking, as vagas que passam na regra. Vaga sem stack
// conhecida não avisa: não dá para dizer que é compatível.
func newAlerts(jobs []DiscoveredJob, rule alertRule) []DiscoveredJob {
	out := []DiscoveredJob{}
	for _, j := range jobs {
		if !j.firstSeen.After(rule.since) || len(j.fit.Items) == 0 {
			continue
		}
		if j.PublishedAt != nil && j.PublishedAt.Before(rule.publishedFrom) {
			continue
		}
		if rule.requireStrong && j.fit.Strong == 0 {
			continue
		}
		if j.fit.Quality == db.JobQualityHigh || (rule.minQuality == db.JobQualityMid && j.fit.Quality == db.JobQualityMid) {
			out = append(out, j)
		}
	}
	return out
}

func (h *DiscoveryHandler) SaveAlerts(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	var body jobAlertSettings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	if body.MinQuality != string(db.JobQualityHigh) && body.MinQuality != string(db.JobQualityMid) {
		writeError(w, http.StatusBadRequest, "min_quality deve ser high ou mid")
		return
	}
	if errR := h.Repo.UpsertAlertSettings(r.Context(), db.AlertsUpsertSettingsParams{
		UserID: userID, Enabled: body.Enabled, MinQuality: body.MinQuality,
	}); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *DiscoveryHandler) MarkAlertsSeen(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	if errR := h.Repo.MarkAlertsSeen(r.Context(), userID); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DismissJob (POST) e RestoreJob (DELETE) em /discovery/jobs/{id}/dismiss: a vaga
// some (ou volta) da lista do Descobrir e dos alertas, só para o usuário.
func (h *DiscoveryHandler) DismissJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	jobID, errID := util.ParseUUID(r.PathValue("id"))
	if errID != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	found, errR := h.Repo.Dismiss(r.Context(), userID, jobID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "vaga não encontrada no catálogo")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DiscoveryHandler) RestoreJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	jobID, errID := util.ParseUUID(r.PathValue("id"))
	if errID != nil {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if errR := h.Repo.Undismiss(r.Context(), userID, jobID); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type alertFeedbackBody struct {
	JobID       string `json:"job_id"`
	Relevant    bool   `json:"relevant"`
	Quality     string `json:"quality"`
	CoveragePct int    `json:"coverage_pct"`
}

// AlertFeedback grava se o alerta foi útil, com a compatibilidade que a vaga tinha
// (mostrada em Métricas → Busca de vagas, para ajustar a regra com dados).
func (h *DiscoveryHandler) AlertFeedback(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}
	var body alertFeedbackBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrInvalidRequestBody.Error())
		return
	}
	jobID, errID := util.ParseUUID(body.JobID)
	if errID != nil {
		writeError(w, http.StatusBadRequest, "job_id inválido")
		return
	}
	switch db.JobQuality(body.Quality) {
	case db.JobQualityHigh, db.JobQualityMid, db.JobQualityLow:
	default:
		writeError(w, http.StatusBadRequest, "quality deve ser high, mid ou low")
		return
	}
	if body.CoveragePct < 0 || body.CoveragePct > 100 {
		writeError(w, http.StatusBadRequest, "coverage_pct fora de 0–100")
		return
	}
	if errR := h.Repo.SaveAlertFeedback(r.Context(), db.AlertsFeedbackUpsertParams{
		UserID: userID, JobID: jobID, Relevant: body.Relevant,
		Quality: body.Quality, CoveragePct: int32(body.CoveragePct),
	}); errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// alertFeedbackStat: avaliações dos alertas de uma faixa de compatibilidade.
type alertFeedbackStat struct {
	Quality     string `json:"quality"`
	Relevant    int64  `json:"relevant"`
	NotRelevant int64  `json:"not_relevant"`
}
