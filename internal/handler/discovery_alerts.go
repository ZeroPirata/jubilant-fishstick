package handler

import (
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/middleware"
	"net/http"
	"time"
)

// Alertas de vagas novas (sininho): vagas que entraram no catálogo do Descobrir desde
// a última vez que o usuário viu os alertas, com compatibilidade a partir do limite
// escolhido, já com o local e o nível salvos. Abrir o sininho ou o Descobrir marca
// como vistas (POST /discovery/alerts/seen): a mesma vaga não é avisada de novo.
//
// Só na plataforma por enquanto; Telegram e e-mail podem usar a mesma lista
// (newAlerts) depois.

// alertListMax: teto de vagas que o painel do sininho lista (o total vem em count).
// Abrir o painel marca todas como vistas, então ele mostra todas as novas: o teto só
// protege contra um volume anormal (primeira varredura de uma fonte grande).
const alertListMax = 100

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
	rk, errR := h.rankedJobs(ctx, userID)
	if errR != nil {
		writeRepositoryError(w, errR)
		return
	}
	if rk.jobs == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	jobs := newAlerts(filterDiscovered(rk.jobs, rk.filter).Jobs, cfg.SeenAt.Time, db.JobQuality(cfg.MinQuality))
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

// newAlerts mantém, na ordem do ranking, as vagas vistas pela primeira vez depois de
// since com compatibilidade a partir de minQuality. Vaga sem stack conhecida não
// avisa: não dá para dizer que é compatível.
func newAlerts(jobs []DiscoveredJob, since time.Time, minQuality db.JobQuality) []DiscoveredJob {
	out := []DiscoveredJob{}
	for _, j := range jobs {
		if !j.firstSeen.After(since) || len(j.fit.Items) == 0 {
			continue
		}
		if j.fit.Quality == db.JobQualityHigh || (minQuality == db.JobQualityMid && j.fit.Quality == db.JobQualityMid) {
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
